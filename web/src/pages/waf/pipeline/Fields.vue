<template>
  <div class="pipeline-fields">
    <label v-for="field in definitions" :key="field.path" class="pipeline-field">
      <span class="field-label">{{ field.label }}</span>
      <ElSwitch v-if="field.kind === 'bool'" :model-value="!!valueAt(model, field.path)" :aria-label="field.label" @update:model-value="setAt(model, field.path, $event)" />
      <ElInputNumber v-else-if="field.kind === 'number'" :model-value="Number(valueAt(model, field.path) || 0)" :min="field.min" :max="field.max" :aria-label="field.label" controls-position="right" @update:model-value="setAt(model, field.path, $event ?? 0)" />
      <ElSelect v-else-if="field.kind === 'select'" :model-value="valueAt(model, field.path) ?? ''" :aria-label="field.label" @update:model-value="setAt(model, field.path, $event)">
        <ElOption v-for="option in field.options" :key="option[0]" :label="option[1]" :value="option[0]" />
      </ElSelect>
      <ElInput v-else-if="field.kind === 'lines'" type="textarea" :rows="3" :model-value="drafts.linesDraft[field.path] ?? (valueAt(model, field.path) || []).join('\n')" :aria-label="field.label" @update:model-value="drafts.linesDraft[field.path] = $event; setAt(model, field.path, parseLines($event))" />
      <ElInput v-else-if="field.kind === 'json' || field.kind === 'json-array'" type="textarea" :rows="3" :model-value="drafts.jsonDraft[field.path] ?? JSON.stringify(valueAt(model, field.path) ?? (field.nullable ? null : field.kind === 'json-array' ? [] : {}), null, 2)" :aria-label="field.label" :aria-invalid="!!drafts.jsonErrors[field.path]" @update:model-value="updateJSON(field, $event)" />
      <ElInput v-else :model-value="valueAt(model, field.path) || ''" :aria-label="field.label" @update:model-value="setAt(model, field.path, $event)" />
      <small>{{ field.help }}</small><strong v-if="drafts.jsonErrors[field.path]" role="alert" class="field-error">{{ drafts.jsonErrors[field.path] }}</strong>
    </label>
  </div>
</template>
<script setup lang="ts">
import { computed, watch } from 'vue';
import type { PropType } from 'vue';
import { ElInput, ElInputNumber, ElSwitch, ElSelect, ElOption } from 'element-plus';
import { valueAt, setAt, parseLines, type Field } from './model';
import { fieldDraftFor, fieldsInvalid, updateJSONDraft } from './fieldDraft';
const props = defineProps({ model: {type:Object as PropType<Record<string,any>>,required:true}, definitions:{type:Array as PropType<Field[]>,required:true} });
const emit = defineEmits(['invalid']);
const drafts = computed(() => fieldDraftFor(props.model));
watch(() => fieldsInvalid(props.model, props.definitions), invalid => emit('invalid', invalid), { immediate: true });
function updateJSON(field: Field, text: string) { updateJSONDraft(props.model, field, text); }
</script>
<style scoped>
.pipeline-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px 28px}.pipeline-field{display:flex;flex-direction:column;align-items:flex-start;gap:8px;min-width:0}.field-label{font-weight: 500;color:var(--el-text-color-primary)}.pipeline-field :deep(.el-input-number),.pipeline-field :deep(.el-select),.pipeline-field :deep(.el-input),.pipeline-field :deep(.el-textarea){width:100%}.pipeline-field small{line-height:1.6;color:var(--el-text-color-secondary)}.field-error{font-size: 13px;color:var(--el-color-danger)}@media(max-width:760px){.pipeline-fields{grid-template-columns:1fr}}
</style>
