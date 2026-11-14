<template>
  <section class="rule-collection">
    <div class="collection-heading">
      <div>
        <h3>{{ title }}</h3>
        <p v-if="description">{{ description }}</p>
      </div>
      <ElButton :disabled="disabled" @click="add">{{ addLabel || '添加条目' }}</ElButton>
    </div>
    <p v-if="!modelValue?.length" class="collection-empty">还没有条目。添加后保存，才会更新服务配置。</p>
    <details
      v-for="(row, index) in modelValue || []"
      :key="row.id || index"
      :open="opened === index"
      class="collection-entry"
    >
      <summary @click.prevent="opened = opened === index ? -1 : index">
        <span class="entry-number">{{ index + 1 }}</span
        ><span>{{ summary(row) || '待填写的条目' }}</span
        ><span class="entry-edit">{{ opened === index ? '收起' : '编辑' }}</span>
      </summary>
      <div class="entry-fields">
        <label v-for="field in schema" :key="field.key" :class="{ 'full-field': field.multiline }">
          <span>{{ field.label }}</span>
          <ElSelect v-if="field.options" v-model="row[field.key]" :disabled="disabled" :aria-label="field.label"
            ><ElOption v-for="option in field.options" :key="option[0]" :value="option[0]" :label="option[1]"
          /></ElSelect>
          <ElInput
            v-else-if="field.lines"
            :model-value="(row[field.key] || []).join('\n')"
            type="textarea"
            :rows="3"
            :disabled="disabled"
            :aria-label="field.label"
            @update:model-value="
              row[field.key] = String($event)
                .split(/\r?\n/)
                .map((v) => v.trim())
                .filter(Boolean)
            "
          />
          <input
            v-else-if="field.kind === 'datetime'"
            type="datetime-local"
            :value="localDate(row[field.key])"
            :disabled="disabled"
            :aria-label="field.label"
            @change="row[field.key] = epoch(($event.target as HTMLInputElement).value)"
          />
          <ElInput
            v-else
            v-model="row[field.key]"
            :type="field.multiline ? 'textarea' : 'text'"
            :rows="field.multiline ? 5 : undefined"
            :disabled="disabled"
            :aria-label="field.label"
          />
          <small v-if="field.help">{{ field.help }}</small>
        </label>
        <div class="entry-actions">
          <ElButton type="danger" plain :disabled="disabled" @click="remove(index)">移除此条目</ElButton
          ><span>保存前仍可撤销修改。</span>
        </div>
      </div>
    </details>
  </section>
</template>
<script setup lang="ts">
import { ref, type PropType } from 'vue';
import { v4 as uuidv4 } from 'uuid';
import { ElButton, ElInput, ElSelect, ElOption } from 'element-plus';
export interface CollectionField {
  key: string;
  label: string;
  multiline?: boolean;
  lines?: boolean;
  help?: string;
  options?: string[][];
  kind?: 'datetime';
}
const props = defineProps({
  modelValue: Array as PropType<Record<string, any>[]>,
  title: { type: String, required: true },
  description: String,
  schema: { type: Array as PropType<CollectionField[]>, required: true },
  defaults: { type: Object as PropType<Record<string, any>>, required: true },
  summaryKey: String,
  addLabel: String,
  disabled: Boolean,
});
const emit = defineEmits(['update:modelValue']);
const opened = ref(-1);
const localDate = (value: number) => {
  if (!value) return '';
  const date = new Date(value * 1000);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
};
const epoch = (value: string) => (value ? Math.floor(new Date(value).getTime() / 1000) : 0);
function summary(row: Record<string, any>) {
  return row[props.summaryKey || 'value'] || row.content || row.id || row.upstream;
}
function add() {
  const row = JSON.parse(JSON.stringify(props.defaults));
  if (Object.prototype.hasOwnProperty.call(row, 'id')) row.id = uuidv4();
  opened.value = props.modelValue?.length || 0;
  emit('update:modelValue', [...(props.modelValue || []), row]);
}
function remove(index: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, i) => i !== index),
  );
  opened.value = -1;
}
</script>
<style scoped>
.collection-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}
h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
p {
  margin: 6px 0 0;
  color: var(--hw-muted);
  font-size: 13px;
}
.collection-empty {
  padding: 24px;
  background: var(--hw-subtle);
  border-radius: 6px;
}
.collection-entry {
  margin-top: 10px;
  border: 1px solid var(--hw-line);
  border-radius: 6px;
}
summary {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  cursor: pointer;
  list-style: none;
  overflow-wrap: anywhere;
}
summary::-webkit-details-marker {
  display: none;
}
.entry-number {
  color: var(--hw-muted);
  font-family: var(--font-mono);
}
.entry-edit {
  margin-left: auto;
  flex-shrink: 0;
  color: var(--hw-accent);
  font-size: 13px;
}
.entry-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  padding: 20px;
  border-top: 1px solid var(--hw-line);
}
label {
  display: grid;
  gap: 8px;
  min-width: 0;
}
label > span {
  font-size: 13px;
  font-weight: 500;
}
small,
.entry-actions > span {
  color: var(--hw-muted);
  font-size: 12px;
}
input[type='datetime-local'] {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  padding: 9px 12px;
  color: var(--hw-ink);
  background: var(--hw-panel);
}
.full-field,
.entry-actions {
  grid-column: 1/-1;
}
.entry-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
@media (max-width: 620px) {
  .entry-fields {
    grid-template-columns: 1fr;
    padding: 16px;
  }
  .collection-heading {
    flex-wrap: wrap;
  }
}
</style>
