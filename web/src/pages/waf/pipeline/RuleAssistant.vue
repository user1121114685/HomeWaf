<template>
  <div class="rule-assistant">
    <ElButton :disabled="disabled" @click="visible = true">构建 / AI 生成规则</ElButton>
    <ElDialog
      v-model="visible"
      title="准备自定义防护规则"
      width="min(920px,96vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!busy"
      :show-close="!busy"
    >
      <p>
        服务 {{ config.name || config.port }} · :{{
          config.port
        }}。生成结果默认“仅观察”；验证后加入当前草稿，保存服务配置后才生效。
      </p>
      <ElRadioGroup v-model="method" :disabled="busy" aria-label="规则准备方式"
        ><ElRadioButton value="builder">条件构建</ElRadioButton><ElRadioButton value="ai">AI 生成</ElRadioButton
        ><ElRadioButton value="source">输入源码</ElRadioButton></ElRadioGroup
      >
      <section v-if="method === 'builder'" class="rule-preparation">
        <label
          >条件关系<ElSelect v-model="relation" :disabled="busy" aria-label="条件关系"
            ><ElOption value="&&" label="满足全部条件" /><ElOption value="||" label="满足任一条件" /></ElSelect
        ></label>
        <div v-for="(condition, index) in conditions" :key="index" class="condition-row">
          <label
            >请求字段<ElSelect v-model="condition.field" :disabled="busy" :aria-label="'条件字段 ' + (index + 1)"
              ><ElOption v-for="field in attributes" :key="field[0]" :value="field[0]" :label="field[1]" /></ElSelect
          ></label>
          <label
            >匹配方式<ElSelect v-model="condition.operator" :disabled="busy" :aria-label="'匹配方式 ' + (index + 1)"
              ><ElOption value="==" label="等于" /><ElOption value="!=" label="不等于" /><ElOption
                value="system.Contains"
                label="包含" /><ElOption value="system.HasPrefix" label="开头为" /></ElSelect
          ></label>
          <label
            >匹配内容<ElInput
              v-model="condition.value"
              :disabled="busy"
              :aria-label="'匹配内容 ' + (index + 1)" /></label
          ><ElButton :disabled="busy || conditions.length === 1" @click="conditions.splice(index, 1)"
            >移除条件</ElButton
          >
        </div>
        <div class="assistant-actions">
          <ElButton
            :disabled="busy || conditions.length >= 20"
            @click="conditions.push({ field: 'URL', operator: '==', value: '' })"
            >添加条件</ElButton
          ><ElButton :loading="busy" @click="build">生成规则预览</ElButton>
        </div>
      </section>
      <section v-if="method === 'ai'" class="rule-preparation">
        <label
          >需要防护的请求<ElInput
            v-model="intent"
            type="textarea"
            :rows="3"
            :disabled="busy"
            aria-label="防护需求"
            placeholder="例如：观察访问 /private 且方法为 POST 的请求"
        /></label>
        <div class="assistant-actions">
          <ElButton :loading="busy" @click="generate">生成规则</ElButton
          ><ElButton :disabled="busy" @click="configVisible = true">AI 参数设置</ElButton
          ><ElButton :disabled="busy" @click="loadPrompt">查看生成提示词</ElButton>
        </div>
        <details v-if="prompt" open>
          <summary>后端生成提示词（可选中复制）</summary>
          <ElInput :model-value="prompt" readonly type="textarea" :rows="5" aria-label="规则生成提示词" />
        </details>
      </section>
      <section class="rule-preparation">
        <label
          >规则源码预览<ElInput v-model="source" type="textarea" :rows="8" :disabled="busy" aria-label="生成规则源码"
        /></label>
        <label>样例请求路径<ElInput v-model="samplePath" :disabled="busy" aria-label="规则样例路径" /></label>
        <ElAlert v-if="error" ref="errorAlert" tabindex="-1" :title="error" type="error" :closable="false" />
        <p v-if="validation && validatedSource === source" role="status">{{ validation }}</p>
        <Trace v-if="preview && validatedSource === source" :result="preview" />
      </section>
      <template #footer
        ><ElButton :disabled="busy" @click="visible = false">取消</ElButton
        ><ElButton :loading="busy" :disabled="!source.trim()" @click="validate">验证并解释草稿</ElButton
        ><ElButton type="primary" :disabled="busy || !source.trim() || validatedSource !== source" @click="add"
          >加入当前服务草稿</ElButton
        ></template
      >
    </ElDialog>
    <GptConfigDialog v-model:visible="configVisible" />
  </div>
</template>
<script setup lang="ts">
import { nextTick, ref, watch, type PropType } from 'vue';
import { v4 as uuidv4 } from 'uuid';
import { ElAlert, ElButton, ElDialog, ElInput, ElOption, ElRadioButton, ElRadioGroup, ElSelect } from 'element-plus';
import { wafRuleFormatApi, wafRuleAiGenApi, wafRuleAiPromptApi } from '@/apis/rules';
import { pipelinePost } from '@/apis/pipeline';
import GptConfigDialog from '@/components/gpt-config/index.vue';
import Trace from './Trace.vue';
import { clone, type PortConfig } from './model';
const props = defineProps({ config: { type: Object as PropType<PortConfig>, required: true }, disabled: Boolean });
const emit = defineEmits(['add', 'editing', 'busy']);
const visible = ref(false),
  configVisible = ref(false),
  busy = ref(false),
  method = ref('builder');
const conditions = ref([{ field: 'URL', operator: '==', value: '' }]),
  relation = ref('&&');
const source = ref(''),
  intent = ref(''),
  prompt = ref(''),
  error = ref(''),
  validation = ref(''),
  validatedSource = ref(''),
  preview = ref<any>(),
  samplePath = ref('/');
const id = ref(uuidv4());
const errorAlert = ref<any>();
watch(error, async (message) => {
  if (message) {
    await nextTick();
    errorAlert.value?.$el?.focus();
  }
});
const attributes = [
  ['URL', '请求路径'],
  ['METHOD', '请求方法'],
  ['SRC_IP', '来源 IP'],
  ['USER_AGENT', 'User-Agent'],
  ['HOST', '主机名'],
  ['COUNTRY', '国家/地区'],
];
function script() {
  return { id: id.value, source: source.value, mode: 'observe', severity: 'medium' };
}
async function run(action: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await action();
  } catch (cause: any) {
    error.value = cause.message || '操作失败，输入已保留，请重试。';
  } finally {
    busy.value = false;
  }
}
async function build() {
  await run(async () => {
    if (conditions.value.some((row) => !row.value.trim())) throw new Error('请填写每个条件的匹配内容。');
    const result = await wafRuleFormatApi({
      rule_code: id.value,
      is_manual_rule: 0,
      form_source: 'builder',
      rule_json: JSON.stringify({
        rule_base: { rule_name: '服务自定义规则', rule_domain_code: '', salience: 10 },
        rule_action: 'deny',
        rule_condition: {
          relation_symbol: relation.value,
          relation_detail: conditions.value.map((row) => ({
            fact_name: 'MF',
            attr: row.field,
            attr_type: 'string',
            attr_judge: row.operator,
            attr_val: row.value,
            attr_val2: 'true',
          })),
        },
      }),
    });
    if (result.code !== 0 || !result.data?.rule_content) throw new Error(result.msg || '规则预览生成失败');
    source.value = result.data.rule_content;
    validatedSource.value = '';
    preview.value = undefined;
  });
}
async function generate() {
  await run(async () => {
    if (!intent.value.trim()) throw new Error('请填写防护需求。');
    const result = await wafRuleAiGenApi({ intent: intent.value, lang: 'zh' });
    if (result.code !== 0 || !result.data?.rule_content) throw new Error(result.msg || 'AI 生成失败，请检查 AI 参数。');
    source.value = result.data.rule_content;
    validatedSource.value = '';
    preview.value = undefined;
    if (!result.data.valid) throw new Error(result.data.error || '生成内容未通过后端校验，请修改后重试。');
  });
}
async function loadPrompt() {
  await run(async () => {
    const result = await wafRuleAiPromptApi({ lang: 'zh' });
    if (result.code !== 0) throw new Error(result.msg);
    prompt.value = typeof result.data === 'string' ? result.data : JSON.stringify(result.data, null, 2);
  });
}
async function validate() {
  await run(async () => {
    validatedSource.value = '';
    validation.value = '';
    preview.value = undefined;
    const config = clone(props.config);
    config.policy.ruleset.scripts = [...(config.policy.ruleset.scripts || []), script()];
    preview.value = await pipelinePost('explain', {
      config,
      client_ip: '203.0.113.10',
      method: 'GET',
      path: samplePath.value,
      headers: {},
      body: '',
    });
    validatedSource.value = source.value;
    validation.value =
      '已通过当前服务的配置与规则编译校验。以下显示整条策略的判定；前置登录或其他限制可能使请求尚未进入自定义规则。未保存、未产生处罚。';
  });
}
function add() {
  if (validatedSource.value !== source.value || !source.value.trim()) return;
  emit('add', script());
  visible.value = false;
  source.value = '';
  intent.value = '';
  preview.value = undefined;
  validatedSource.value = '';
  id.value = uuidv4();
}
watch(samplePath, () => {
  validatedSource.value = '';
  preview.value = undefined;
});
watch(visible, (value) => emit('editing', value));
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
</script>
<style scoped>
.rule-preparation {
  display: grid;
  gap: 16px;
  margin-top: 22px;
}
.rule-preparation label {
  display: grid;
  gap: 8px;
}
.condition-row {
  display: grid;
  grid-template-columns: 1fr 1fr 1.5fr auto;
  gap: 12px;
  align-items: end;
}
.assistant-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
p {
  color: var(--hw-muted);
  line-height: 1.7;
}
@media (max-width: 700px) {
  .condition-row {
    grid-template-columns: 1fr;
    padding-bottom: 18px;
    border-bottom: 1px solid var(--hw-line);
  }
}
</style>
