<template>
  <div class="pipeline-fields">
    <div
      v-for="field in definitions"
      :key="field.path"
      class="pipeline-field"
      :class="{
        'pipeline-field--wide': ['lines', 'json', 'json-array'].includes(field.kind),
        'pipeline-field--switch': field.kind === 'bool',
      }"
    >
      <label class="field-label" :for="controlId(field.path)">{{ field.label }}</label>
      <ElSwitch
        v-if="field.kind === 'bool'"
        :id="controlId(field.path)"
        :model-value="!!valueAt(model, field.path)"
        :aria-label="field.label"
        :aria-describedby="controlId(field.path) + '-help'"
        @update:model-value="setAt(model, field.path, $event)"
      />
      <ElInputNumber
        v-else-if="field.kind === 'number'"
        :id="controlId(field.path)"
        :model-value="Number(valueAt(model, field.path) || 0)"
        :min="field.min"
        :max="field.max"
        :aria-label="field.label"
        :aria-describedby="controlId(field.path) + '-help'"
        controls-position="right"
        @update:model-value="setAt(model, field.path, $event ?? 0)"
      />
      <ElSelect
        v-else-if="field.kind === 'select'"
        :id="controlId(field.path)"
        :model-value="valueAt(model, field.path) ?? ''"
        :aria-label="field.label"
        :aria-describedby="controlId(field.path) + '-help'"
        @update:model-value="setAt(model, field.path, $event)"
        ><ElOption v-for="option in field.options" :key="option[0]" :label="option[1]" :value="option[0]"
      /></ElSelect>
      <ElInput
        v-else-if="field.kind === 'lines'"
        :id="controlId(field.path)"
        type="textarea"
        :rows="3"
        :model-value="drafts.linesDraft[field.path] ?? (valueAt(model, field.path) || []).join('\n')"
        :aria-label="field.label"
        :aria-describedby="controlId(field.path) + '-help'"
        @update:model-value="
          drafts.linesDraft[field.path] = $event;
          setAt(model, field.path, parseLines($event));
        "
      />
      <ElInput
        v-else-if="field.kind === 'json' || field.kind === 'json-array'"
        :id="controlId(field.path)"
        type="textarea"
        :rows="4"
        :model-value="
          drafts.jsonDraft[field.path] ??
          JSON.stringify(
            valueAt(model, field.path) ?? (field.nullable ? null : field.kind === 'json-array' ? [] : {}),
            null,
            2,
          )
        "
        :aria-label="field.label"
        :aria-invalid="!!drafts.jsonErrors[field.path]"
        :aria-describedby="controlId(field.path) + '-help'"
        @update:model-value="updateJSON(field, $event)"
      />
      <ElInput
        v-else
        :id="controlId(field.path)"
        :model-value="valueAt(model, field.path) || ''"
        :aria-label="field.label"
        :aria-describedby="controlId(field.path) + '-help'"
        @update:model-value="setAt(model, field.path, $event)"
      />
      <small :id="controlId(field.path) + '-help'">{{ field.help }}</small>
      <strong v-if="drafts.jsonErrors[field.path]" role="alert" class="field-error">{{
        drafts.jsonErrors[field.path]
      }}</strong>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, useId, watch } from 'vue';
import type { PropType } from 'vue';
import { ElInput, ElInputNumber, ElSwitch, ElSelect, ElOption } from 'element-plus';
import { valueAt, setAt, parseLines, type Field } from './model';
import { fieldDraftFor, fieldsInvalid, updateJSONDraft } from './fieldDraft';
const props = defineProps({
  model: { type: Object as PropType<Record<string, any>>, required: true },
  definitions: { type: Array as PropType<Field[]>, required: true },
});
const emit = defineEmits(['invalid']);
const prefix = useId(),
  controlId = (path: string) => prefix + '-' + path.replace(/[^a-z0-9_-]/gi, '-');
const drafts = computed(() => fieldDraftFor(props.model));
watch(
  () => fieldsInvalid(props.model, props.definitions),
  (invalid) => emit('invalid', invalid),
  { immediate: true },
);
function updateJSON(field: Field, text: string) {
  updateJSONDraft(props.model, field, text);
}
</script>
<style scoped>
.pipeline-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.pipeline-field {
  display: grid;
  align-content: start;
  gap: 8px;
  min-width: 0;
}
.pipeline-field--wide,
.pipeline-field--switch {
  grid-column: 1/-1;
}
.field-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--hw-ink);
  line-height: 1.6;
}
.pipeline-field :deep(.el-input-number),
.pipeline-field :deep(.el-select),
.pipeline-field :deep(.el-input),
.pipeline-field :deep(.el-textarea) {
  width: 100%;
}
.pipeline-field small {
  line-height: 1.75;
  color: var(--hw-muted);
  font-size: 12px;
}
.pipeline-field--switch {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 4px 24px;
  padding: 4px 0 16px;
  border-bottom: 1px solid var(--hw-line);
}
.pipeline-field--switch .el-switch {
  grid-column: 2;
  grid-row: 1/3;
  align-self: center;
}
.pipeline-field--switch small {
  grid-column: 1;
}
.field-error {
  font-size: 13px;
  color: var(--hw-danger);
}
@media (max-width: 760px) {
  .pipeline-fields {
    grid-template-columns: 1fr;
    gap: 20px;
  }
}
</style>
