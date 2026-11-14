<template>
  <section class="honeypot-settings" data-testid="honeypot-settings">
    <h3>蜜罐设置</h3>
    <p class="help">可随时修改并保存当前服务的蜜罐策略，无需重新创建服务。</p>
    <Fields :model="model" :definitions="group('enabled', 'capture_denied')" />

    <section class="settings-section">
      <div class="section-heading"><h4>仿真提示与权重</h4><ElButton :disabled="lures.length >= 32" @click="addLure">添加蜜罐提示</ElButton></div>
      <p class="help">同一来源稳定选择提示。文案只作为文本展示；移除全部提示后使用内置页面。</p>
      <p v-if="!lures.length" class="help">当前使用内置提示，可添加自己的标题、内容和展示权重。</p>
      <fieldset v-for="(lure, index) in lures" :key="lure.id || index" class="lure-row">
        <legend>提示 {{ index + 1 }}</legend>
        <div class="lure-heading">
          <label>页面标题<ElInput v-model="lure.title" :aria-label="`蜜罐提示${index + 1}标题`" /></label>
          <label>展示权重<ElInputNumber :model-value="lure.weight" :min="1" :max="10000" :aria-label="`蜜罐提示${index + 1}权重`" @update:model-value="lure.weight = $event ?? 1" /></label>
          <ElButton type="danger" plain :aria-label="`移除蜜罐提示${index + 1}`" @click="removeLure(index)">移除提示</ElButton>
        </div>
        <label>提示内容<ElInput v-model="lure.message" type="textarea" :rows="3" :aria-label="`蜜罐提示${index + 1}内容`" /></label>
      </fieldset>
      <p v-for="error in lureErrors" :key="error" class="field-error" role="alert">{{ error }}</p>
    </section>

    <section class="settings-section">
      <h4>命中后丢弃响应</h4>
      <ElAlert v-if="!hitPolicyEnabled" title="自动处罚当前关闭，以下命中阈值不会生效。" type="warning" :closable="false"><ElButton text @click="emit('open-punishment')">配置自动处罚</ElButton></ElAlert>
      <Fields :model="model" :definitions="dropFields" />
    </section>
    <section class="settings-section"><h4>请求归档</h4><Fields :model="model" :definitions="group('archive_body', 'archive_max_bytes')" /></section>
    <section class="settings-section"><h4>陷阱路径与运行限制</h4><Fields :model="model" :definitions="runtimeFields" /></section>
  </section>
</template>
<script setup lang="ts">
import { computed, watch } from 'vue';
import type { PropType } from 'vue';
import { ElAlert, ElButton, ElInput, ElInputNumber } from 'element-plus';
import Fields from './Fields.vue';
import { fields } from './model';
const props = defineProps({ model: { type: Object as PropType<Record<string, any>>, required: true }, hitPolicyEnabled: { type: Boolean, required: true } });
const emit = defineEmits(['invalid', 'open-punishment']);
const group = (...paths: string[]) => fields.honeypot.filter(field => paths.includes(field.path));
const dropFields = fields.honeypot.filter(field => field.path.startsWith('drop_') || field.path.startsWith('distributed_drop.'));
const runtimeFields = group('ttl_seconds', 'auth_fail_threshold', 'weak_password_threshold', 'max_concurrent', 'max_delay_ms', 'trap_paths');
const lures = computed<any[]>(() => Array.isArray(props.model.lures) ? props.model.lures : []);
const byteLength = (value: string) => new TextEncoder().encode(value).length;
const lureErrors = computed(() => {
  const errors: string[] = [], ids = new Set<string>();
  let total = 0;
  if (lures.value.length > 32) errors.push('最多配置32条蜜罐提示。');
  lures.value.forEach((lure, index) => {
    const title = String(lure.title || ''), message = String(lure.message || '');
    if (!lure.id || ids.has(lure.id)) errors.push(`提示${index + 1}的标识无效，请移除后重新添加。`);
    ids.add(lure.id);
    if (!title.trim() || byteLength(title) > 128) errors.push(`提示${index + 1}的标题不能为空且最多128字节。`);
    if (byteLength(message) > 4096) errors.push(`提示${index + 1}的内容最多4096字节。`);
    if (!Number.isInteger(lure.weight) || lure.weight < 1 || lure.weight > 10000) errors.push(`提示${index + 1}的权重必须是1–10000的整数。`);
    total += byteLength(title) + byteLength(message);
  });
  if (total > 32768) errors.push('蜜罐提示文案合计最多32KiB。');
  return errors;
});
watch(() => lureErrors.value.length > 0, invalid => emit('invalid', invalid), { immediate: true });
function addLure() {
  if (lures.value.length >= 32) return;
  props.model.lures = [...lures.value, { id: crypto.randomUUID(), title: '管理控制台', message: '请输入管理账号。', weight: 1 }];
}
function removeLure(index: number) { props.model.lures = lures.value.filter((_, i) => i !== index); }
</script>
<style scoped>
.honeypot-settings{min-width:0}.honeypot-settings h3{font-size:17px;margin:16px 0}.help{color:var(--el-text-color-regular);font-size:13px;line-height:1.8;margin:8px 0 20px}.settings-section{margin-top:28px;padding-top:24px;border-top:1px solid var(--el-border-color-light)}h4{font-size:16px;font-weight:500;margin:0 0 20px}.section-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:16px}.section-heading h4{margin:0}.lure-row{min-width:0;border:1px solid var(--el-border-color-light);border-radius:8px;margin:16px 0;padding:16px}.lure-row legend{font-size:13px;padding:0 8px;color:var(--el-text-color-regular)}.lure-row label{display:flex;flex-direction:column;gap:8px;min-width:0;font-size:13px}.lure-heading{display:grid;grid-template-columns:minmax(0,1fr) 160px auto;gap:16px;align-items:end;margin-bottom:16px}.lure-heading :deep(.el-input-number){width:100%}.honeypot-settings :deep(.el-button){min-height:44px}.field-error{font-size:13px;line-height:1.7;color:var(--el-color-danger)}@media(max-width:600px){.section-heading{flex-wrap:wrap}.lure-heading{grid-template-columns:minmax(0,1fr)}.lure-row{padding:12px}.section-heading .el-button{width:100%}}
</style>
