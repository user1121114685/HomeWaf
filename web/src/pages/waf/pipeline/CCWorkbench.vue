<template>
  <section class="cc-workbench">
    <Fields
      :model="config.policy.ruleset"
      :definitions="fields.ruleset.filter((field) => field.path.startsWith('cc.'))"
    />
    <div class="cc-actions">
      <ElButton :disabled="disabled" @click="recommendVisible = true">按历史流量估算阈值</ElButton
      ><ElButton :disabled="disabled" @click="openHits">查看最近 CC 命中</ElButton
      ><ElButton :disabled="disabled" @click="openServices">各服务挑战状态</ElButton>
    </div>
    <section class="challenge-settings">
      <h3>临时访问挑战</h3>
      <p>
        让匹配的访客先完成轻量 Cookie 挑战。可能影响 App、Webhook 和未启用 Cookie 的客户端；以下修改随“保存 CC
        防护”一起生效。
      </p>
      <ElTag :type="activeChallenge ? 'warning' : 'info'">{{ challengeState }}</ElTag>
      <div class="challenge-fields">
        <label
          >挑战模式<ElSelect
            :model-value="challenge.mode || 'off'"
            :disabled="disabled"
            aria-label="临时挑战模式"
            @update:model-value="setChallenge('mode', $event)"
            ><ElOption label="关闭" value="off" /><ElOption label="仅观察" value="observe" /><ElOption
              label="要求挑战"
              value="block" /></ElSelect
        ></label>
        <label
          >到期时间（留空为手动关闭）<input
            type="datetime-local"
            :value="localTime(challenge.until)"
            :disabled="disabled"
            aria-label="挑战到期时间"
            @change="setChallenge('until', epoch(($event.target as HTMLInputElement).value))"
        /></label>
      </div>
      <div class="cc-actions">
        <ElButton
          v-for="minutes in [30, 60, 360]"
          :key="minutes"
          :disabled="disabled"
          @click="setChallenge('until', Math.floor(Date.now() / 1000) + minutes * 60)"
          >{{ minutes < 60 ? minutes + ' 分钟后关闭' : minutes / 60 + ' 小时后关闭' }}</ElButton
        >
      </div>
      <details @toggle="scopeOpened">
        <summary>挑战范围与例外</summary>
        <Scope v-if="scopeReady" :model="config.policy.ruleset.challenge.scope" />
        <p>
          已通过挑战的客户端及挑战处理路径会跳过重复挑战。这里的路径例外只作用于临时挑战，CC
          的静态资源例外由上方开关控制。
        </p>
      </details>
    </section>
    <ElDialog
      v-model="recommendVisible"
      title="估算当前服务的 CC 阈值"
      width="min(680px,96vw)"
      :close-on-click-modal="false"
    >
      <p>
        端口 {{ config.port }} ·
        {{ config.name || '当前服务' }}。按草稿中的统计窗口和路径/静态资源选项计算；估算不会保存配置。
      </p>
      <div class="cc-actions">
        <label
          >最近天数<ElSelect v-model="days" aria-label="估算天数"
            ><ElOption v-for="day in [1, 3, 7]" :key="day" :value="day" :label="day + ' 天'" /></ElSelect></label
        ><ElButton :loading="recommending" @click="recommend">读取流量并估算</ElButton>
      </div>
      <ElAlert v-if="recommendError" :title="recommendError" type="error" :closable="false" />
      <template v-if="recommendation">
        <p>{{ recommendation.scope }}</p>
        <p>
          有效样本 {{ recommendation.requests }} 条 · {{ recommendation.clients }} 个来源 IP<span
            v-if="recommendation.from"
          >
            · {{ time(recommendation.from) }} 至 {{ time(recommendation.to) }}</span
          >
        </p>
        <ElAlert v-if="!recommendation.supported" :title="recommendation.reason" type="warning" :closable="false" />
        <template v-else
          ><p>
            窗口 {{ recommendation.window_seconds }} 秒 · P99 {{ recommendation.p99 }} 次 · 建议观察阈值
            <strong>{{ recommendation.suggested }} 次</strong>
          </p>
          <ElAlert v-if="staleRecommendation" title="统计选项已变化，请重新估算。" type="warning" :closable="false"
        /></template>
      </template>
      <template #footer
        ><ElButton @click="recommendVisible = false">取消</ElButton
        ><ElButton
          type="primary"
          :disabled="!recommendation?.supported || staleRecommendation || recommending || disabled"
          @click="applyRecommendation"
          >填入草稿阈值</ElButton
        ></template
      >
    </ElDialog>
    <ElDialog v-model="hitsVisible" title="当前服务最近 CC 命中" width="min(1000px,96vw)">
      <p>当前日志分区最近最多 200 条 CC 命中，受保留期和队列状态影响，不是累计攻击总数。</p>
      <ElButton :loading="hitsLoading" @click="openHits">刷新命中</ElButton>
      <ElAlert v-if="hitsError" :title="hitsError" type="error" :closable="false" />
      <p v-if="hits.length">
        {{ hits.length }} 条命中 · 观察 {{ hits.filter((row) => row.observe).length }} 条 ·
        {{ new Set(hits.map((row) => row.client_ip)).size }} 个来源
      </p>
      <ElTable
        :data="hits"
        :empty-text="hitsError ? '读取失败，请重试' : hitsLoading ? '正在读取…' : '当前分区暂无 CC 命中'"
        max-height="450"
        ><ElTableColumn label="时间" min-width="170"
          ><template #default="{ row }">{{ time(row.time_ms) }}</template></ElTableColumn
        ><ElTableColumn prop="client_ip" label="来源 IP" min-width="150" /><ElTableColumn label="判定" min-width="130"
          ><template #default="{ row }">{{
            row.observe
              ? '仅观察'
              : row.verdict === 'challenge'
                ? '要求挑战'
                : row.verdict === 'deny'
                  ? '拒绝'
                  : row.verdict
          }}</template></ElTableColumn
        ><ElTableColumn label="详情" width="120"
          ><template #default="{ row }"
            ><ElButton link :disabled="!row.req_id" @click="openLog(row)">查看请求</ElButton></template
          ></ElTableColumn
        ></ElTable
      >
    </ElDialog>
    <ElDialog v-model="servicesVisible" title="各服务临时挑战状态" width="min(1000px,96vw)">
      <p>显示已保存的配置；过期挑战不再执行。选择服务后可在 CC 页修改。</p>
      <ElButton :loading="servicesLoading" @click="openServices">刷新状态</ElButton
      ><ElAlert v-if="servicesError" :title="servicesError" type="error" :closable="false" />
      <ElTable
        :data="services"
        :empty-text="servicesError ? '读取失败，请重试' : servicesLoading ? '正在读取…' : '暂无服务'"
        ><ElTableColumn label="服务" min-width="180"
          ><template #default="{ row }">{{ row.config.name }} · :{{ row.config.port }}</template></ElTableColumn
        ><ElTableColumn label="实际状态" min-width="180"
          ><template #default="{ row }">{{
            row.listener?.running ? stateText(row.config.policy.ruleset.challenge) : '未监听'
          }}</template></ElTableColumn
        ><ElTableColumn label="路径例外" min-width="180"
          ><template #default="{ row }">{{
            (row.config.policy.ruleset.challenge?.scope?.exclude_paths || []).join('、') || '无'
          }}</template></ElTableColumn
        ><ElTableColumn label="操作" width="100"
          ><template #default="{ row }"
            ><ElButton link @click="openService(row)">配置</ElButton></template
          ></ElTableColumn
        ></ElTable
      >
    </ElDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, type PropType } from 'vue';
import { useRouter } from 'vue-router';
import { ElAlert, ElButton, ElDialog, ElOption, ElSelect, ElTable, ElTableColumn, ElTag } from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import { fields, type PortConfig } from './model';
import Fields from './Fields.vue';
import Scope from './Scope.vue';
const props = defineProps({ config: { type: Object as PropType<PortConfig>, required: true }, disabled: Boolean });
const router = useRouter(),
  challenge = computed<any>(() => props.config.policy.ruleset.challenge || {});
const scopeReady = ref(false),
  recommendVisible = ref(false),
  hitsVisible = ref(false),
  servicesVisible = ref(false);
const days = ref(1),
  recommending = ref(false),
  recommendError = ref(''),
  recommendation = ref<any>(),
  recommendationKey = ref('');
const hits = ref<any[]>([]),
  hitsLoading = ref(false),
  hitsError = ref('');
const services = ref<any[]>([]),
  servicesLoading = ref(false),
  servicesError = ref('');
const request = computed(() => ({
  port: props.config.port,
  days: days.value,
  window_seconds: props.config.policy.ruleset.cc.window_seconds,
  by_path: !!props.config.policy.ruleset.cc.by_path,
  exclude_static: !!props.config.policy.ruleset.cc.exclude_static,
}));
const staleRecommendation = computed(() => recommendationKey.value !== JSON.stringify(request.value));
let mounted = true;
const now = ref(Date.now());
let clock: ReturnType<typeof setInterval>;
onMounted(() => {
  clock = setInterval(() => {
    now.value = Date.now();
  }, 1000);
});
onBeforeUnmount(() => {
  mounted = false;
  clearInterval(clock);
});
const time = (value: number) => new Date(value).toLocaleString();
const localTime = (value: number) =>
  value ? new Date(value * 1000 - new Date(value * 1000).getTimezoneOffset() * 60000).toISOString().slice(0, 16) : '';
const epoch = (value: string) => (value ? Math.floor(new Date(value).getTime() / 1000) : 0);
function stateText(value: any) {
  if (!value || !value.mode || value.mode === 'off') return '关闭';
  if (value.until && value.until <= now.value / 1000) return '已到期';
  return value.mode === 'observe' ? '仅观察' : '要求挑战';
}
const challengeState = computed(() => stateText(challenge.value)),
  activeChallenge = computed(() => ['仅观察', '要求挑战'].includes(challengeState.value));
function setChallenge(key: string, value: any) {
  props.config.policy.ruleset.challenge ||= { mode: 'off', until: 0, scope: {} };
  props.config.policy.ruleset.challenge[key] = value;
}
function scopeOpened(event: Event) {
  if ((event.target as HTMLDetailsElement).open) {
    setChallenge('scope', challenge.value.scope || {});
    scopeReady.value = true;
  }
}
async function recommend() {
  if (recommending.value) return;
  recommending.value = true;
  recommendError.value = '';
  recommendation.value = undefined;
  const data = { ...request.value },
    key = JSON.stringify(data);
  try {
    const result = await pipelinePost('cc/recommend', data);
    if (mounted) {
      recommendation.value = result;
      recommendationKey.value = key;
    }
  } catch (cause: any) {
    if (mounted) recommendError.value = cause.message || '估算失败，请重试。';
  } finally {
    recommending.value = false;
  }
}
function applyRecommendation() {
  if (!recommendation.value?.supported || staleRecommendation.value) return;
  props.config.policy.ruleset.cc.threshold = recommendation.value.suggested;
  recommendVisible.value = false;
}
async function openHits() {
  hitsVisible.value = true;
  if (hitsLoading.value) return;
  hitsLoading.value = true;
  hitsError.value = '';
  try {
    hits.value = await pipelineGet('cc/hits', { port: props.config.port });
  } catch (cause: any) {
    hits.value = [];
    hitsError.value = cause.message;
  } finally {
    hitsLoading.value = false;
  }
}
async function openServices() {
  servicesVisible.value = true;
  if (servicesLoading.value) return;
  servicesLoading.value = true;
  servicesError.value = '';
  try {
    services.value = (await pipelineGet('overview')).ports || [];
  } catch (cause: any) {
    services.value = [];
    servicesError.value = cause.message;
  } finally {
    servicesLoading.value = false;
  }
}
function openLog(row: any) {
  hitsVisible.value = false;
  router.push({ name: 'WafAttackLogDetail', query: { req_uuid: row.req_id, current_db_name: 'auto' } });
}
function openService(row: any) {
  servicesVisible.value = false;
  router.push({ name: 'WafAntiCCList', query: { host_code: row.config.host_code || 'pipeline-' + row.config.port } });
}
</script>
<style scoped>
.cc-workbench {
  display: grid;
  gap: 24px;
}
.cc-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 10px;
}
.cc-actions label,
.challenge-fields label {
  display: grid;
  gap: 8px;
}
.challenge-settings {
  border-top: 1px solid var(--hw-line);
  padding-top: 20px;
}
h3 {
  font-size: 16px;
  margin: 0;
}
p {
  color: var(--hw-muted);
  line-height: 1.7;
}
.challenge-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  margin: 18px 0;
}
input {
  min-width: 0;
  min-height: 34px;
  padding: 6px 10px;
  color: var(--hw-ink);
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: 4px;
}
details {
  margin-top: 18px;
}
summary {
  cursor: pointer;
  margin-bottom: 16px;
}
@media (max-width: 600px) {
  .challenge-fields {
    grid-template-columns: 1fr;
  }
}
</style>
