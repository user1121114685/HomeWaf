<template>
  <fieldset class="editor-section" :class="{ 'editor-section--compact': compact }">
    <legend v-if="title || $slots.title">
      <slot name="title">{{ title }}</slot>
    </legend>
    <p v-if="description" class="editor-section-description">{{ description }}</p>
    <div class="editor-section-fields" :class="{ 'editor-section-fields--two': columns === 2 }"><slot /></div>
  </fieldset>
</template>
<script setup lang="ts">
defineProps({ title: String, description: String, columns: { type: Number, default: 1 }, compact: Boolean });
</script>
<style scoped>
.editor-section {
  min-width: 0;
  margin: 0 0 24px;
  padding: 0 0 8px;
  border: 0;
  border-bottom: 1px solid var(--hw-line);
}
.editor-section:last-child {
  margin-bottom: 0;
  border-bottom: 0;
}
legend {
  width: 100%;
  padding: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.6;
  color: var(--hw-ink);
}
.editor-section-description {
  margin: 0 0 18px;
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.75;
  max-width: 78ch;
}
.editor-section-fields {
  min-width: 0;
}
.editor-section-fields--two {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 24px;
}
.editor-section-fields :deep(> *) {
  min-width: 0;
}
.editor-section-fields--two
  :deep(> :is(p, .form-tip, .form-item-tips, .editor-footer, .el-alert, .el-divider, .hw-loading)) {
  grid-column: 1/-1;
}
.editor-section-fields--two :deep(> .el-form-item:has(.el-textarea, .el-table, .el-upload)) {
  grid-column: 1/-1;
}
.editor-section-fields :deep(.el-form-item__content > :is(.el-input, .el-select, .el-textarea)) {
  width: 100%;
}
.editor-section--compact {
  margin-bottom: 18px;
}
@media (max-width: 700px) {
  .editor-section-fields--two {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
