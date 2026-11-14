<template>
  <section class="access-choices" :inert="disabled || undefined" data-testid="access-restrictions">
    <h3>额外访问限制 <span>可选，可组合</span></h3>
    <p class="access-intro">先检查访问来源，再执行已开启的登录认证、限流和攻击检测。</p>
    <div class="access-choice">
      <ElCheckbox v-model="policy.gate.enabled" :disabled="policy.gate.require_token" aria-label="使用独立访问账号"
        >独立访问账号</ElCheckbox
      >
      <p>
        {{
          policy.gate.require_token
            ? '当前 API 模板使用服务令牌，保存后在认证设置中生成。'
            : '先登录 HomeWaf，再访问原服务，减少未认证请求直接触达原服务漏洞的机会。'
        }}
      </p>
    </div>
    <div class="access-choice">
      <ElCheckbox v-model="regional" aria-label="限制允许访问的地区" @change="updateSources(true)"
        >仅允许指定地区</ElCheckbox
      >
      <p>直接选择中国、重庆等国家或省市，无需填写 IP 段。IPv4 和 IPv6 共用所选地区规则。</p>
      <div v-if="regional" class="access-detail">
        <label for="allowed-service-regions">允许访问的地区</label>
        <ElSelectV2
          id="allowed-service-regions"
          v-model="chosenRegions"
          :options="regionOptions"
          multiple
          filterable
          clearable
          :loading="loading"
          placeholder="搜索国家或省市，例如中国、重庆"
          aria-label="允许访问的地区"
          @change="updateSources(true)"
        />
        <p>匹配所选国家的全部地区，或所选省市。无法识别归属地的地址需通过白名单 IP 组获准访问。</p>
      </div>
    </div>
    <div class="access-choice">
      <ElCheckbox v-model="allowGroupsEnabled" aria-label="使用白名单IP组" @change="updateSources(true)"
        >仅允许白名单 IP 组</ElCheckbox
      >
      <p>选用已维护的 IP 组；没有合适的组时，可在这里新建。</p>
      <div v-if="allowGroupsEnabled" class="access-detail">
        <IPGroupPicker
          v-model="chosenAllowGroups"
          purpose="allow"
          title="允许访问的 IP 组"
          :groups="groups"
          :loading="groupsLoading"
          :load-error="groupsError"
          @update:model-value="updateSources(true)"
          @refresh="loadGroups"
          @created="groupCreated"
          @busy="groupBusy = $event"
        />
      </div>
    </div>
    <div class="access-choice">
      <ElCheckbox v-model="blacklist" aria-label="启用IP黑名单" @change="updateSources()">IP 黑名单</ElCheckbox>
      <p>优先拒绝命中的来源，即使它同时在白名单中。</p>
      <div v-if="blacklist" class="access-detail">
        <ElCheckbox v-model="outsideMainland" aria-label="禁止中国大陆以外IP访问" @change="updateSources()"
          >禁止中国大陆以外 IP 访问</ElCheckbox
        >
        <p>包含港澳台和境外来源；归属地未知的公网地址也会被拒绝。内网地址不参与地域判断，仍受其他访问规则限制。</p>
        <label for="denied-service-regions">其他禁止访问的地区（可选）</label>
        <ElSelectV2
          id="denied-service-regions"
          v-model="chosenDeniedRegions"
          :options="regionOptions"
          multiple
          filterable
          clearable
          :loading="loading"
          placeholder="选择要屏蔽的国家或省市"
          aria-label="禁止访问的地区"
          @change="updateSources()"
        />
        <IPGroupPicker
          v-model="chosenDenyGroups"
          purpose="deny"
          title="禁止访问的 IP 组（可选）"
          :groups="groups"
          :loading="groupsLoading"
          :load-error="groupsError"
          @update:model-value="updateSources()"
          @refresh="loadGroups"
          @created="groupCreated"
          @busy="groupBusy = $event"
        />
      </div>
    </div>
    <div v-if="regional || (blacklist && (outsideMainland || chosenDeniedRegions.length))" class="database-tools">
      <p v-if="regionError" role="alert" class="access-error">{{ regionError }}</p>
      <p v-else-if="!loading && !regions.length" class="access-error">当前没有可用的地区选项，请检查 IP 数据库。</p>
      <div class="region-tools">
        <ElButton :loading="loading" @click="loadRegions">刷新地区列表</ElButton
        ><RouterLink to="/sys/IPLocation" target="_blank" rel="noopener noreferrer">管理 IP 数据库</RouterLink>
      </div>
      <p v-if="dbStatus" class="database-state">
        IPv4：{{ dbStatus.ipv4_file_size > 0 ? '已加载' : '未加载' }} · IPv6：{{
          dbStatus.ipv6_file_size > 0 ? '已加载' : '未加载'
        }}。地区选项来自当前数据库，更新后可刷新。
      </p>
    </div>
    <p v-if="regional || allowGroupsEnabled" class="access-summary">
      {{
        regional && allowGroupsEnabled
          ? '所选地区或白名单 IP 组，满足任意一项即可继续；黑名单始终优先。'
          : '只有命中白名单的来源才可继续；黑名单始终优先。'
      }}
    </p>
    <p v-if="legacyEntries" class="access-summary">
      已保留 {{ legacyEntries }} 条已有单独地址规则，可在高级配置中管理；新增地址请使用统一 IP 组。
    </p>
    <p v-if="conflictingGroups" class="access-error">
      同一个 IP 组同时出现在黑白名单中：黑名单优先，该组的地址会被拒绝。
    </p>
    <p
      v-if="
        policy.ip_group.unlisted_action === 'continue' &&
        (policy.ip_group.regions?.length || policy.ip_group.allow_groups?.length)
      "
      class="access-summary"
    >
      已有来源名单，但尚未限制其他来源。勾选上方白名单选项后才会拒绝未命中的来源。
    </p>
    <p class="access-summary">
      {{
        policy.gate.enabled
          ? '获准来源仍需登录独立访问账号。'
          : policy.gate.require_token
            ? '获准来源仍需提供服务令牌。'
            : ''
      }}白名单 IP 组不会跳过限流和攻击检测；拦截原因可在访问日志与请求解释中查看。
    </p>
    <p v-if="showValidation && validationMessage" class="access-error" role="alert">{{ validationMessage }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { ElButton, ElCheckbox, ElSelectV2 } from 'element-plus';
import { pipelineGet } from '@/apis/pipeline';
import { getIPDBStatusApi } from '@/apis/iplocation';
import { wafIPGroupOptionsApi } from '@/apis/ipgroup';
import IPGroupPicker from './IPGroupPicker.vue';
import type { Policy } from './model';
const props = defineProps({ policy: { type: Object as PropType<Policy>, required: true }, disabled: Boolean });
const emit = defineEmits(['invalid']);
const regional = ref(false),
  allowGroupsEnabled = ref(false),
  blacklist = ref(false),
  outsideMainland = ref(false);
const chosenRegions = ref<string[]>([]),
  chosenDeniedRegions = ref<string[]>([]),
  chosenAllowGroups = ref<string[]>([]),
  chosenDenyGroups = ref<string[]>([]);
const regions = ref<any[]>([]),
  regionError = ref(''),
  loading = ref(false),
  dbStatus = ref<any>(null),
  showValidation = ref(false);
const groups = ref<any[]>([]),
  groupsLoading = ref(false),
  groupsError = ref(''),
  groupsLoaded = ref(false),
  groupBusy = ref(false);
const regionKey = (region: any) => JSON.stringify([region.country, region.province || '']);
const legacyEntries = computed(() => (props.policy.ip_group.entries || []).length);
const legacyAllows = computed(() => (props.policy.ip_group.entries || []).some((entry) => entry.type === 'allow'));
let updatingSources = false;
watch(
  () => props.policy.ip_group,
  (group) => {
    if (updatingSources) return;
    regional.value = group.unlisted_action === 'deny' && !!group.regions?.length;
    chosenRegions.value = (group.regions || []).map(regionKey);
    allowGroupsEnabled.value = group.unlisted_action === 'deny' && !!group.allow_groups?.length;
    chosenAllowGroups.value = [...(group.allow_groups || [])];
    chosenDenyGroups.value = [...(group.deny_groups || [])];
    chosenDeniedRegions.value = (group.denied_regions || []).map(regionKey);
    outsideMainland.value = !!group.block_outside_mainland;
    blacklist.value = outsideMainland.value || !!chosenDeniedRegions.value.length || !!chosenDenyGroups.value.length;
  },
  { immediate: true, deep: true, flush: 'sync' },
);
const regionOptions = computed(() => {
  const values = new Map<string, any>();
  for (const region of [
    ...regions.value,
    ...(props.policy.ip_group.regions || []),
    ...(props.policy.ip_group.denied_regions || []),
  ]) {
    values.set(regionKey(region), region);
    values.set(regionKey({ country: region.country }), { country: region.country });
  }
  return [...values.values()]
    .sort(
      (a, b) =>
        a.country.localeCompare(b.country, 'zh-CN') || (a.province || '').localeCompare(b.province || '', 'zh-CN'),
    )
    .map((region) => ({ label: `${region.country} / ${region.province || '全部地区'}`, value: regionKey(region) }));
});
const conflictingGroups = computed(
  () => blacklist.value && chosenDenyGroups.value.some((code) => chosenAllowGroups.value.includes(code)),
);
const validationMessage = computed(() => {
  if (groupBusy.value) return '正在创建 IP 组，请稍候。';
  if (regional.value && !chosenRegions.value.length) return '请选择至少一个允许访问的地区，或取消地区限制。';
  if (allowGroupsEnabled.value && !chosenAllowGroups.value.length)
    return '请选择或新建一个白名单 IP 组，或取消 IP 组限制。';
  if (blacklist.value && !outsideMainland.value && !chosenDeniedRegions.value.length && !chosenDenyGroups.value.length)
    return '请选择一项黑名单规则，或取消 IP 黑名单。';
  if (
    groupsLoaded.value &&
    [...(props.policy.ip_group.allow_groups || []), ...(props.policy.ip_group.deny_groups || [])].some(
      (code) => !groups.value.some((group) => group.group_code === code),
    )
  )
    return '所选 IP 组已不存在，请刷新并重新选择。';
  if (
    props.policy.ip_group.unlisted_action === 'deny' &&
    !regional.value &&
    !allowGroupsEnabled.value &&
    !legacyAllows.value
  )
    return '当前模板仅允许白名单访问，请选择允许的地区或 IP 组。';
  return '';
});
watch(
  validationMessage,
  (message) => {
    emit('invalid', !!message);
    if (message) showValidation.value = true;
  },
  { immediate: true },
);
function decodeRegions(values: string[]) {
  return values.map((value) => {
    const [country, province] = JSON.parse(value);
    return { country, province };
  });
}
function updateSources(allowChanged = false) {
  updatingSources = true;
  try {
    const group = props.policy.ip_group;
    if (allowChanged) {
      group.regions = regional.value ? decodeRegions(chosenRegions.value) : [];
      group.allow_groups = allowGroupsEnabled.value ? [...chosenAllowGroups.value] : [];
      group.unlisted_action =
        regional.value || allowGroupsEnabled.value || (legacyAllows.value && group.unlisted_action === 'deny')
          ? 'deny'
          : 'continue';
    }
    group.block_outside_mainland = blacklist.value && outsideMainland.value;
    group.denied_regions = blacklist.value ? decodeRegions(chosenDeniedRegions.value) : [];
    group.deny_groups = blacklist.value ? [...chosenDenyGroups.value] : [];
  } finally {
    updatingSources = false;
  }
}
function groupCreated(group: any) {
  groups.value = [...groups.value.filter((existing) => existing.group_code !== group.group_code), group];
}
function validate() {
  showValidation.value = true;
  return !validationMessage.value;
}
defineExpose({ validate });
async function loadGroups() {
  if (groupsLoading.value) return;
  groupsLoading.value = true;
  groupsError.value = '';
  try {
    const result: any = await wafIPGroupOptionsApi();
    if (result.code !== 0 || !Array.isArray(result.data)) throw new Error(result.msg || '返回格式不正确');
    groups.value = result.data;
    groupsLoaded.value = true;
  } catch (cause: any) {
    groupsError.value = `IP 组读取失败：${cause.message || '请重试'}`;
  } finally {
    groupsLoading.value = false;
  }
}
async function loadRegions() {
  if (loading.value) return;
  loading.value = true;
  regionError.value = '';
  const [catalog, status] = await Promise.allSettled([pipelineGet('regions'), getIPDBStatusApi()]);
  if (catalog.status === 'fulfilled' && Array.isArray(catalog.value)) regions.value = catalog.value;
  else
    regionError.value = `地区列表读取失败：${catalog.status === 'rejected' ? catalog.reason?.message || '请重试' : '返回格式不正确'}`;
  if (status.status === 'fulfilled' && (status.value as any)?.code === 0) dbStatus.value = (status.value as any).data;
  loading.value = false;
}
onMounted(() => {
  void loadRegions();
  void loadGroups();
});
</script>

<style scoped>
.access-choices {
  margin: 24px 0;
  border-top: 1px solid var(--el-border-color-light);
  padding-top: 24px;
}
.access-choices h3 {
  font-size: 16px;
  font-weight: 500;
  margin: 0 0 8px;
}
.access-choices h3 > span {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-left: 8px;
  font-weight: 400;
}
.access-intro,
.access-choice > p,
.access-detail p,
.access-summary,
.database-tools p {
  color: var(--el-text-color-regular);
  font-size: 13px;
  line-height: 1.8;
  margin: 0;
}
.access-intro {
  margin-bottom: 16px;
}
.access-choice {
  padding: 16px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.access-choice :deep(.el-checkbox) {
  min-height: 44px;
  height: auto;
  white-space: normal;
}
.access-choice :deep(.el-checkbox__label) {
  font-size: 14px;
  color: var(--el-text-color-primary);
  white-space: normal;
}
.access-choice > p {
  padding-left: 24px;
}
.access-detail {
  display: grid;
  gap: 12px;
  margin: 16px 0 0 24px;
}
.access-detail > label {
  font-size: 14px;
}
.access-detail :deep(.el-select__wrapper) {
  min-height: 44px;
}
.access-detail :deep(.el-select) {
  width: 100%;
}
.region-tools {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.region-tools a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--el-color-primary);
}
.region-tools :deep(.el-button) {
  min-height: 44px;
}
.access-detail .access-error,
.access-error {
  color: var(--el-color-danger);
  font-size: 13px;
  line-height: 1.8;
  margin-top: 8px;
}
.access-summary,
.database-tools {
  margin-top: 16px;
}
.database-state {
  font-variant-numeric: tabular-nums;
}
@media (max-width: 600px) {
  .access-detail {
    margin-left: 0;
  }
  .access-choice > p {
    padding-left: 0;
  }
}
</style>
