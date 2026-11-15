<template>
  <div class="service-task-editor" :aria-busy="busy">
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <ServiceIdentityPanel
      v-if="identityMode"
      :key="config.port + task"
      :port="config.port"
      :revision="draft.revision"
      :mode="identityMode"
      @busy="identityBusy = $event"
      @editing="identityEditing = $event"
      @revision="setRevision"
    />
    <template v-else>
      <CCWorkbench v-if="task === 'WafAntiCCList'" :config="draft" :disabled="busy" />
      <template v-else-if="ipTask">
        <RuleCollection
          v-model="ipEntries"
          :title="ipType === 'allow' ? '手动允许的来源' : '手动拒绝的来源'"
          :description="
            ipType === 'allow'
              ? '普通允许仍经过登录与检测。可信免检会跳过检测与自动处罚，请谨慎使用。'
              : '黑名单优先于白名单与可信来源，仅修改当前服务。'
          "
          :schema="ipSchema"
          :defaults="{ id: '', value: '', type: ipType, trust: 'allow_only', expires_at: 0, remark: '' }"
          :disabled="busy"
        />
        <details class="task-advanced">
          <summary>地区与共享 IP 组</summary>
          <AccessRestrictions :policy="draft.policy" :disabled="busy" @invalid="invalid.access = $event" />
        </details>
        <details class="task-advanced">
          <summary>名单外访问与高级策略</summary>
          <Fields :model="draft.policy.ip_group" :definitions="fields.ip_group" />
        </details>
      </template>
      <RuleCollection
        v-else-if="urlTask"
        v-model="urlEntries"
        :title="urlAction === 'allow' ? '允许的路径' : '拒绝的路径'"
        description="黑名单优先。白名单只跳过后续攻击检测，不绕过访问限制与登录要求。"
        :schema="urlSchema"
        :defaults="{ id: '', match: 'prefix', value: '', action: urlAction, mode: 'observe' }"
        :disabled="busy"
      />
      <template v-else-if="task === 'Wafruleedit'">
        <div><ElButton :disabled="busy" @click="navigateRules('WafRule')">返回检测规则</ElButton></div>
        <RuleAssistant :config="draft" :disabled="busy" @add="addPreparedRule" @editing="ruleAssistantEditing = $event" @busy="ruleAssistantBusy = $event" />
        <RuleCollection
          v-model="draft.policy.ruleset.scripts"
          title="自定义规则"
          description="编辑规则条件，选择观察或拦截，再保存到当前服务。语法与字段由服务器校验。"
          :schema="scriptSchema"
          :defaults="{ id: '', source: '', mode: 'observe', severity: 'medium' }"
          summary-key="id"
          add-label="添加规则"
          :disabled="busy"
        />
        <details class="task-advanced">
          <summary>规则使用的 IP 组快照</summary>
          <Fields
            :model="draft"
            :definitions="[
              {
                path: 'script_groups',
                label: '规则 IP 组快照',
                kind: 'json',
                help: '只影响当前服务；组名或 code 映射到 IP/CIDR 数组。',
              },
            ]"
            @invalid="invalid.groups = $event"
          />
        </details>
      </template>
      <template v-else-if="task === 'WafRule'">
        <div><ElButton :disabled="busy" @click="navigateRules('Wafruleedit')">管理自定义规则</ElButton></div>
        <div class="task-section-heading">
          <h3>攻击检测</h3>
          <p>先确认命中原因与正常业务，再将对应规则从观察改为拦截。</p>
        </div>
        <ElTable :data="draft.policy.ruleset.rules" class="rule-modes">
          <ElTableColumn label="检测规则" min-width="220"
            ><template #default="{ row }"
              ><strong>{{ ruleNames[row.category] || row.category }}</strong>
              <p class="rule-help">{{ ruleHelp[row.category] }}</p></template
            ></ElTableColumn
          >
          <ElTableColumn label="模式" width="145"
            ><template #default="{ row }"
              ><ElSelect v-model="row.mode" :aria-label="(ruleNames[row.category] || row.category) + '模式'"
                ><ElOption
                  v-for="mode in modes"
                  :key="mode[0]"
                  :value="mode[0]"
                  :label="mode[1]" /></ElSelect></template
          ></ElTableColumn>
          <ElTableColumn label="严重度" width="140"
            ><template #default="{ row }"
              ><ElSelect v-model="row.severity" aria-label="规则严重度"
                ><ElOption
                  v-for="level in ['low', 'medium', 'high', 'critical']"
                  :key="level"
                  :value="level"
                  :label="level" /></ElSelect></template
          ></ElTableColumn>
          <ElTableColumn label="范围" width="110"
            ><template #default="{ row }"
              ><ElButton link @click="openScope(row)">范围与例外</ElButton></template
            ></ElTableColumn
          >
        </ElTable>
        <details class="task-advanced">
          <summary>检测引擎与协议参数</summary>
          <Fields
            :model="draft.policy.ruleset"
            :definitions="fields.ruleset.filter((field) => !field.path.startsWith('cc.') && field.path !== 'sensitive')"
            @invalid="invalid.ruleset = $event"
          />
        </details>
      </template>
      <template v-else-if="task === 'WafSensitiveList'">
        <Fields
          :model="draft.policy.ruleset.sensitive"
          :definitions="[
            {
              path: 'mode',
              label: '检测模式',
              kind: 'select',
              options: modes,
              help: '观察只记录；拦截模式才执行拒绝或替换。',
            },
            { path: 'replacement', label: '替换文本', help: '仅用于选择替换动作的规则。' },
          ]"
        />
        <RuleCollection
          v-model="draft.policy.ruleset.sensitive.rules"
          title="敏感内容规则"
          :schema="sensitiveSchema"
          :defaults="{ content: '', check_direction: 'all', action: 'deny' }"
          summary-key="content"
          :disabled="busy"
        />
      </template>
      <template v-else-if="task === 'WafAccessConfig'">
        <Fields
          :model="draft.policy.gate"
          :definitions="
            fields.gate.filter((field) => ['enabled', 'require_token', 'mode', 'title', 'hint'].includes(field.path))
          "
        />
        <details class="task-advanced">
          <summary>会话时限、免登录范围与访问频率</summary>
          <Fields
            :model="draft.policy.gate"
            :definitions="
              fields.gate.filter((field) => !['enabled', 'require_token', 'mode', 'title', 'hint'].includes(field.path))
            "
            @invalid="invalid.gate = $event"
          />
        </details>
        <details class="task-advanced">
          <summary>登录页标识</summary>
          <label class="logo-upload"
            >登录页 Logo<input type="file" accept="image/png,image/jpeg" aria-label="登录页Logo" @change="setGateLogo"
          /></label>
          <p class="rule-help">PNG/JPEG，64 KiB 以内，宽高不超过 512 像素。文件保存在服务配置中。</p>
          <img
            v-if="draft.policy.gate.logo_data"
            :src="draft.policy.gate.logo_data"
            class="gate-logo"
            alt="登录页Logo预览"
          /><ElButton v-if="draft.policy.gate.logo_data" @click="draft.policy.gate.logo_data = ''">清除Logo</ElButton>
        </details>
        <ServiceIdentityPanel
          :port="config.port"
          :revision="draft.revision"
          mode="token"
          @busy="identityBusy = $event"
          @revision="setRevision"
        />
      </template>
      <template v-else-if="privacyTask">
        <Fields
          :model="draft.policy.network.response.privacy"
          :definitions="[
            {
              path: 'mode',
              label: '响应脱敏模式',
              kind: 'select',
              options: [['off', '关闭'], ['observe', '仅观察'], ['block', '执行脱敏']],
              help: '观察不修改响应正文；选择拦截模式时执行脱敏。',
            },
          ]"
        />
        <RuleCollection
          v-model="draft.policy.network.response.privacy.urls"
          title="需要脱敏的路径"
          :schema="privacySchema"
          :defaults="{ match: 'prefix', value: '' }"
          :disabled="busy"
        />
        <details class="task-advanced">
          <summary>隐私引擎高级配置</summary>
          <label class="secret-label"
            >自定义配置（留空保留）<ElInput
              v-model="privacySecret"
              type="textarea"
              :rows="6"
              autocomplete="off"
              aria-label="隐私引擎配置" /></label
          ><ElCheckbox v-model="clearPrivacy">清除自定义隐私配置并使用内置规则</ElCheckbox>
        </details>
      </template>
      <template v-else-if="task === 'WafPathRuleList'">
        <Fields
          :model="draft"
          :definitions="[
            { path: 'upstream', label: '默认上游地址', help: '没有匹配下方路由时使用的业务目标。' },
            {
              path: 'upstream_dial_ip',
              label: '固定连接 IP（可选）',
              help: '仅改变连接地址，Host 与 TLS SNI 保持上游主机名。',
            },
          ]"
        />
        <RuleCollection
          v-model="draft.routes"
          title="路径转发规则"
          description="同一端口的路由共用安全策略、账号和计数器。按主机名精度与最长路径前缀匹配。"
          :schema="routeSchema"
          :defaults="{ hosts: [], path_prefix: '/', upstream: '', dial_ip: '' }"
          summary-key="upstream"
          :disabled="busy"
        />
      </template>
      <div class="task-savebar">
        <span>{{ dirty ? '有未保存的修改' : '当前配置已读取' }} · 仅作用于 {{ config.name || '此服务' }}</span
        ><ElButton :disabled="!dirty || busy" @click="reset">撤销修改</ElButton
        ><ElButton type="primary" :loading="saving" :disabled="!dirty || busy || invalidDraft" @click="save">{{
          task === 'WafAntiCCList' ? '保存 CC 防护' : '保存配置'
        }}</ElButton>
      </div>
      <p v-if="invalidDraft" role="alert" class="task-error">请先修正上方配置错误；输入已保留。</p>
    </template>
    <ElDialog
      v-model="scopeVisible"
      :title="'范围与例外 · ' + (scopeRule ? ruleNames[scopeRule.category] || scopeRule.id : '')"
      width="min(850px,94vw)"
      :close-on-click-modal="false"
    >
      <Scope v-if="scopeRule" :model="scopeRule.scope" /><Fields
        v-if="scopeRule?.category === 'scanner'"
        :model="scopeRule.params"
        :definitions="[
          {
            path: 'probe_paths',
            label: '本站不存在的探测路径',
            kind: 'lines',
            help: '每行一个；不要填写真实业务路径。',
          },
        ]"
      /><template #footer><ElButton @click="scopeVisible = false">返回规则</ElButton></template>
    </ElDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, reactive, ref, watch, type PropType } from 'vue';
import { useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElDialog,
  ElInput,
  ElMessage,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
} from 'element-plus';
import { pipelinePost } from '@/apis/pipeline';
import { clone, fields, modes, ruleHelp, ruleNames, type PortConfig } from './model';
import Fields from './Fields.vue';
import Scope from './Scope.vue';
import AccessRestrictions from './AccessRestrictions.vue';
import RuleCollection, { type CollectionField } from './RuleCollection.vue';
import ServiceIdentityPanel from './ServiceIdentityPanel.vue';
import CCWorkbench from './CCWorkbench.vue';
import RuleAssistant from './RuleAssistant.vue';
const props = defineProps({
  config: { type: Object as PropType<PortConfig>, required: true },
  task: { type: String, required: true },
});
const emit = defineEmits(['saved', 'busy']);
const router = useRouter();
function navigateRules(name: string) { router.push({ name, query: { host_code: props.config.host_code || 'pipeline-' + props.config.port } }); }
const ipTask = computed(() => ['WafIpWhiteList', 'WafIpBlockList'].includes(props.task));
const ipType = computed(() => (props.task === 'WafIpWhiteList' ? 'allow' : 'deny'));
const urlTask = computed(() => ['WafUrlWhiteList', 'WafUrlBlockList'].includes(props.task));
const urlAction = computed(() => (props.task === 'WafUrlWhiteList' ? 'allow' : 'deny'));
const privacyTask = computed(() => ['PrivateInfo', 'WafLdpUrlList'].includes(props.task));
const identityMode = computed(() =>
  props.task === 'WafAccessAccount' ? 'accounts' : props.task === 'WafAccessSession' ? 'sessions' : null,
);
const draft = ref<any>(),
  baseline = ref(''),
  error = ref(''),
  saving = ref(false),
  identityBusy = ref(false),
  identityEditing = ref(false),
  ruleAssistantEditing = ref(false),
  ruleAssistantBusy = ref(false),
  privacySecret = ref(''),
  clearPrivacy = ref(false);
const invalid = reactive<Record<string, boolean>>({});
const scopeVisible = ref(false),
  scopeRule = ref<any>();
const busy = computed(() => saving.value || identityBusy.value || ruleAssistantBusy.value);
const invalidDraft = computed(() => Object.values(invalid).some(Boolean));
function snapshot() {
  const { revision, ...value } = draft.value;
  return JSON.stringify(value);
}
function reset() {
  // Token rotation commits immediately and advances the same service revision.
  // Reverting unsaved fields must not put that known revision back in the past.
  const currentRevision = draft.value?.revision ?? props.config.revision;
  draft.value = clone(props.config);
  draft.value.revision = Math.max(currentRevision, props.config.revision);
  draft.value.policy.ip_group.entries ||= [];
  if (props.task === 'WafSensitiveList')
    draft.value.policy.ruleset.sensitive ||= { mode: 'off', replacement: '', rules: [] };
  if (privacyTask.value) {
    draft.value.policy.network.response ||= {};
    draft.value.policy.network.response.privacy ||= { mode: 'off', urls: [] };
  }
  baseline.value = snapshot();
  error.value = '';
  privacySecret.value = '';
  clearPrivacy.value = false;
  for (const key of Object.keys(invalid)) delete invalid[key];
}
reset();
const dirty = computed(
  () =>
    snapshot() !== baseline.value ||
    !!privacySecret.value ||
    clearPrivacy.value ||
    invalidDraft.value ||
    identityEditing.value ||
    ruleAssistantEditing.value ||
    scopeVisible.value,
);
function subset(key: string, field: string, value: () => string) {
  const target = () => (key === 'entries' ? draft.value.policy.ip_group : draft.value.policy.ruleset);
  return computed({
    get: () => (target()[key] || []).filter((row) => row[field] === value()),
    set: (rows: any[]) => {
      const all = target()[key] || [];
      target()[key] = [
        ...all.filter((row) => row[field] !== value() || rows.includes(row)),
        ...rows.filter((row) => !all.includes(row)),
      ];
    },
  });
}
const ipEntries = subset('entries', 'type', () => ipType.value),
  urlEntries = subset('urls', 'action', () => urlAction.value);
const matching = [
  ['exact', '精确匹配'],
  ['prefix', '前缀匹配'],
  ['suffix', '后缀匹配'],
  ['contains', '包含'],
];
const ipSchema = computed<CollectionField[]>(() => [
  { key: 'value', label: 'IP / CIDR / 地址范围' },
  ...(ipType.value === 'allow'
    ? [
        {
          key: 'trust',
          label: '允许方式',
          options: [
            ['allow_only', '允许后继续检测'],
            ['trusted', '可信免检'],
          ],
        },
      ]
    : []),
  { key: 'expires_at', label: '到期时间（留空为不限）', kind: 'datetime' },
  { key: 'remark', label: '备注' },
]);
const urlSchema: CollectionField[] = [
  { key: 'value', label: '访问路径' },
  { key: 'match', label: '匹配方式', options: matching },
  { key: 'mode', label: '生效模式', options: modes },
];
const scriptSchema: CollectionField[] = [
  { key: 'id', label: '规则 ID' },
  { key: 'mode', label: '生效模式', options: modes },
  {
    key: 'severity',
    label: '严重度',
    options: [
      ['low', '低'],
      ['medium', '中'],
      ['high', '高'],
      ['critical', '严重'],
    ],
  },
  { key: 'source', label: '规则源码', multiline: true },
];
const sensitiveSchema: CollectionField[] = [
  { key: 'content', label: '检测内容' },
  {
    key: 'check_direction',
    label: '检测方向',
    options: [
      ['in', '请求'],
      ['out', '响应'],
      ['all', '请求与响应'],
    ],
  },
  {
    key: 'action',
    label: '命中动作',
    options: [
      ['deny', '拒绝'],
      ['replace', '替换'],
    ],
  },
];
const privacySchema: CollectionField[] = [
  { key: 'value', label: '访问路径' },
  { key: 'match', label: '匹配方式', options: matching },
];
const routeSchema: CollectionField[] = [
  { key: 'hosts', label: '主机名（每行一个）', lines: true, multiline: true },
  { key: 'path_prefix', label: '路径前缀' },
  { key: 'upstream', label: '上游地址' },
  { key: 'dial_ip', label: '固定连接 IP（可选）' },
];
function openScope(row: any) {
  row.scope ||= {};
  row.params ||= {};
  scopeRule.value = row;
  scopeVisible.value = true;
}
function addPreparedRule(rule: any) {
  draft.value.policy.ruleset.scripts ||= [];
  draft.value.policy.ruleset.scripts.push(rule);
}
function setRevision(revision: number) {
  draft.value.revision = revision;
}
async function setGateLogo(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (file.size > 65536 || !['image/png', 'image/jpeg'].includes(file.type)) {
    error.value = '请选择64KiB以内的PNG/JPEG';
    return;
  }
  try {
    draft.value.policy.gate.logo_data = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(new Error('Logo读取失败'));
      reader.readAsDataURL(file);
    });
    input.value = '';
  } catch (cause: any) {
    error.value = cause.message;
  }
}
async function save() {
  if (busy.value || invalidDraft.value || !dirty.value) return;
  saving.value = true;
  error.value = '';
  try {
    const saved = await pipelinePost<PortConfig>('ports/save', {
      config: clone(draft.value),
      ...(privacyTask.value
        ? { privacy_engine_config: privacySecret.value, clear_privacy_engine_config: clearPrivacy.value }
        : {}),
    });
    draft.value = clone(saved);
    baseline.value = snapshot();
    privacySecret.value = '';
    clearPrivacy.value = false;
    emit('saved', saved);
    ElMessage.success(saved.listen_enabled ? '配置已保存并应用到当前服务' : '配置已保存，服务启用后生效');
  } catch (cause: any) {
    error.value = (cause.message || '保存失败') + '。修改已保留，请检查后重试。';
  } finally {
    saving.value = false;
  }
}
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
defineExpose({ dirty });
</script>
<style scoped>
.service-task-editor {
  display: grid;
  gap: 24px;
  min-width: 0;
}
.task-section-heading h3 {
  font-size: 16px;
  margin: 0 0 8px;
  font-weight: 600;
}
.task-section-heading p,
.rule-help {
  margin: 6px 0 0;
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
}
.task-advanced {
  border-top: 1px solid var(--hw-line);
  padding-top: 16px;
}
.task-advanced summary {
  cursor: pointer;
  min-height: 32px;
  font-weight: 500;
  margin-bottom: 12px;
}
.task-savebar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px 0 0;
  border-top: 1px solid var(--hw-line);
}
.task-savebar > span {
  margin-right: auto;
  font-size: 13px;
  color: var(--hw-muted);
}
.task-error {
  color: var(--hw-danger);
  margin: 0;
}
.secret-label,
.logo-upload {
  display: grid;
  gap: 8px;
  margin-bottom: 16px;
}
.gate-logo {
  max-width: 96px;
  max-height: 96px;
  display: block;
  margin-bottom: 12px;
}
@media (max-width: 620px) {
  .task-savebar > span {
    width: 100%;
  }
}
</style>
