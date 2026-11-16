<template>
  <div class="defense-policy" data-testid="defense-policy">
    <label v-if="!required" class="switch-row">
      <span>启用异常访问处置</span>
      <ElSwitch v-model="model.enabled" :disabled="busy" aria-label="启用异常访问处置" />
    </label>
    <fieldset :disabled="busy" class="mode-fieldset">
      <legend>发现异常后，如何保护这个服务？<span v-if="required">必选一项</span></legend>
      <div class="mode-options">
        <label
          v-for="mode in defenseModes"
          :key="mode.value"
          class="mode-option"
          :class="{ selected: model.mode === mode.value }"
        >
          <input
            v-model="model.mode"
            type="radio"
            :name="`defense-mode-${config.port}`"
            :value="mode.value"
            :aria-label="mode.label"
          />
          <span
            ><strong>{{ mode.label }}</strong
            ><small>{{ mode.description }}</small></span
          >
        </label>
      </div>
    </fieldset>
    <div class="rule-summary" role="status" aria-live="polite">
      <strong>{{ required ? '服务启用后，这条规则会生效' : '保存后的处置规则' }}</strong>
      <p>{{ defenseSummary(model) }}</p>
    </div>
    <template v-if="model.mode && model.mode !== 'honeypot'">
      <div class="timing-fields">
        <label>
          <span>在最近多长时间内？</span>
          <div class="number-unit">
            <ElInputNumber
              :model-value="model.window_seconds / 60"
              :min="1"
              :max="1440"
              :disabled="busy"
              aria-label="累计异常的时间范围（分钟）"
              @update:model-value="model.window_seconds = Number($event) * 60"
            /><span>分钟</span>
          </div>
          <small>每次向前看这段时间，超过范围的异常不再计数。</small>
        </label>
        <label>
          <span>累计多少次异常后处置？</span>
          <div class="number-unit">
            <ElInputNumber
              v-model="model.count"
              :min="1"
              :max="100000"
              :disabled="busy"
              aria-label="触发处置的异常次数"
            /><span>次</span>
          </div>
          <small>同一端口、不同 IP 的异常一起累计；不要求连续发生。</small>
        </label>
        <label>
          <span>处置后多久自动恢复？</span>
          <div class="number-unit">
            <ElInputNumber
              :model-value="model.duration_seconds / 3600"
              :min="1"
              :max="168"
              :disabled="busy"
              aria-label="自动恢复等待时间（小时）"
              @update:model-value="model.duration_seconds = Number($event) * 3600"
            /><span>小时</span>
          </div>
          <small>从触发处置开始计时；新的异常不会推迟恢复。</small>
        </label>
      </div>
      <p class="count-help">
        “异常”指被安全策略拒绝的请求或认证失败。同一请求只算一次；正常访问不会清零，普通 404、500 和回源故障不计入。
      </p>
    </template>
    <template v-if="model.mode === 'quiet'">
      <label class="tcp-field">
        <span>如何断开异常连接？</span>
        <ElSelect v-model="model.tcp_action" :disabled="busy" aria-label="TCP拒绝方式">
          <ElOption value="reset" label="立即断开连接" />
          <ElOption
            value="drop"
            label="静默丢包，让对方等待超时"
            :disabled="!capabilities?.drop_supported || !!readError"
          />
        </ElSelect>
      </label>
      <p>只有未通过检查的访问会被断开，正常新 IP 和登录入口仍可使用。同一连接上的其他请求也可能中断。</p>
      <p v-if="capabilities && !capabilities.drop_supported">静默丢包暂不可用：{{ capabilities.drop_reason }}</p>
    </template>
    <ElAlert
      v-if="model.mode === 'restrict'"
      title="刚接入时通常还没有成功访问记录。此时一旦触发，会关闭整个服务访问，直到自动恢复或由你提前恢复。"
      type="warning"
      :closable="false"
    />
    <p v-if="model.mode === 'restrict'">
      按近 30 天成功来源的 IPv4 /16、IPv6 /64
      网段放行，可能包含其他设备；范围内仍需通过你设置的地区、IP、账号和安全检查。
    </p>
    <p v-if="model.mode === 'close'">
      关闭的是这个服务的业务访问，HomeWaf 管理控制台仍可使用。你可以在服务的异常访问处置页面提前恢复。
    </p>
    <div v-if="model.mode === 'honeypot'" class="prerequisite">
      <strong>{{
        checking
          ? '正在检查统一蜜罐…'
          : readError
            ? '无法确认统一蜜罐是否就绪'
            : honeyReady
              ? '统一蜜罐已就绪'
              : '先准备统一蜜罐内容'
      }}</strong>
      <p>
        多个服务共用一套仿真页面。<a :href="honeyLink" target="_blank" rel="noopener">在新标签页配置统一蜜罐</a
        >，当前草稿会留在这里；保存后返回继续。
      </p>
      <ElButton :loading="checking" :disabled="busy" @click="refreshPrerequisite">重新检查蜜罐</ElButton>
    </div>
    <ElAlert v-if="readError" :title="readError" type="error" :closable="false"
      ><ElButton text :disabled="busy" @click="refreshPrerequisite">重试读取</ElButton></ElAlert
    >
    <ElAlert
      v-if="capabilities?.runtime_error && model.mode === 'quiet'"
      :title="capabilities.runtime_error"
      type="error"
      :closable="false"
    />
    <p
      v-if="availabilityError || (showValidation && validationError)"
      ref="errorElement"
      class="field-error"
      role="alert"
      tabindex="-1"
    >
      {{ availabilityError || validationError }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { useRouter } from 'vue-router';
import { ElAlert, ElButton, ElInputNumber, ElOption, ElSelect, ElSwitch } from 'element-plus';
import { pipelineGet } from '@/apis/pipeline';
import type { PortConfig } from './model';
import { defenseModes, defenseSummary, defenseValidation, type DefensePolicy } from './defensePolicy';

const props = defineProps({
  config: { type: Object as PropType<PortConfig>, required: true },
  required: Boolean,
  busy: Boolean,
});
const emit = defineEmits(['invalid']);
const model = computed(() => props.config.policy.defense as DefensePolicy);
const router = useRouter();
const honeyLink = computed(
  () => router.resolve({ path: '/waf-pipeline/honeypot', query: { port: props.config.port } }).href,
);
const capabilities = ref<any>(null),
  honeyReady = ref(false),
  checking = ref(false),
  readError = ref('');
const showValidation = ref(false),
  errorElement = ref<HTMLElement>();
const validationError = computed(() => defenseValidation(model.value, props.required));
const availabilityError = computed(() => {
  if (!model.value.enabled) return '';
  if (model.value.mode === 'quiet') {
    if (props.config.policy.network.trusted_proxies?.length)
      return '当前服务配置了前置代理，断开连接可能影响多个访客，请选择其他处置方式。';
    if (model.value.tcp_action === 'drop' && (!capabilities.value?.drop_supported || readError.value))
      return '当前环境无法确认静默丢包可用，请重试或选择立即断开连接。';
  }
  if (model.value.mode === 'honeypot' && !honeyReady.value)
    return checking.value ? '请等待蜜罐检查完成。' : '请先保存统一蜜罐设置，再重新检查并继续。';
  return '';
});
watch(
  [validationError, availabilityError],
  () => emit('invalid', !!validationError.value || !!availabilityError.value),
  { immediate: true },
);
let generation = 0,
  mounted = true;
async function refreshPrerequisite() {
  const seq = ++generation,
    mode = model.value.mode;
  capabilities.value = null;
  honeyReady.value = false;
  readError.value = '';
  checking.value = mode === 'quiet' || mode === 'honeypot';
  if (!checking.value) return;
  try {
    if (mode === 'quiet') {
      const result = await pipelineGet('defense/capabilities', {
        ip_version: props.config.policy.network.listen_ip_version || '',
      });
      if (mounted && seq === generation) capabilities.value = result;
    } else {
      const result = await pipelineGet<any>('honeypot');
      if (mounted && seq === generation) honeyReady.value = !!result.shared?.configured;
    }
  } catch (error: any) {
    if (mounted && seq === generation) readError.value = error.message || '读取失败，请重试。';
  } finally {
    if (mounted && seq === generation) checking.value = false;
  }
}
watch(() => [model.value.mode, props.config.policy.network.listen_ip_version], refreshPrerequisite, {
  immediate: true,
});
function onFocus() {
  if (model.value.mode === 'honeypot') refreshPrerequisite();
}
onMounted(() => window.addEventListener('focus', onFocus));
onUnmounted(() => {
  mounted = false;
  generation++;
  window.removeEventListener('focus', onFocus);
});
async function validate() {
  showValidation.value = true;
  if (!validationError.value && !availabilityError.value) return true;
  await nextTick();
  errorElement.value?.focus();
  return false;
}
defineExpose({ validate });
</script>

<style scoped>
.defense-policy {
  display: grid;
  gap: 20px;
  min-width: 0;
}
.defense-policy p {
  margin: 0;
  color: var(--hw-muted);
  line-height: 1.7;
}
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.mode-fieldset {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.mode-fieldset legend {
  padding: 0;
  margin-bottom: 12px;
  font-weight: 500;
}
.mode-fieldset legend > span {
  margin-left: 12px;
  color: var(--hw-muted);
  font-size: 13px;
  font-weight: 400;
}
.mode-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.mode-option {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  border: 1px solid var(--hw-line);
  padding: 16px;
  border-radius: var(--hw-r-md);
  cursor: pointer;
}
.mode-option.selected {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.mode-option:focus-within {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.mode-option input {
  margin: 4px 0 0;
  accent-color: var(--el-color-primary);
  flex-shrink: 0;
}
.mode-option strong {
  font-weight: 500;
}
.mode-option small {
  display: block;
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--hw-muted);
}
.rule-summary {
  border-left: 3px solid var(--el-color-primary);
  padding: 12px 16px;
  background: var(--el-fill-color-light);
}
.rule-summary strong {
  display: block;
  font-weight: 500;
  margin-bottom: 8px;
}
.rule-summary p {
  color: var(--hw-ink);
}
.timing-fields {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}
.timing-fields label,
.tcp-field {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.timing-fields label {
  grid-template-rows: min-content min-content 1fr;
}
.timing-fields label > span {
  font-weight: 500;
}
.number-unit {
  display: flex;
  gap: 8px;
  align-items: center;
}
.number-unit :deep(.el-input-number) {
  width: 100%;
  min-width: 0;
  flex: 1;
}
.number-unit > span {
  white-space: nowrap;
}
.timing-fields small {
  color: var(--hw-muted);
  line-height: 1.7;
}
.tcp-field {
  max-width: 440px;
}
.prerequisite {
  display: grid;
  justify-items: start;
  gap: 12px;
}
.defense-policy a {
  color: var(--el-color-primary);
  text-decoration: underline;
}
.defense-policy .field-error {
  color: var(--el-color-danger);
}
.defense-policy :deep(.el-button),
.defense-policy :deep(.el-select__wrapper),
.defense-policy :deep(.el-input-number) {
  min-height: 44px;
}
.defense-policy :deep(.el-input-number .el-input__wrapper) {
  min-height: 44px;
}
@media (max-width: 700px) {
  .mode-options,
  .timing-fields {
    grid-template-columns: 1fr;
  }
}
</style>
