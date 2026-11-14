<template>
  <div
    ref="element"
    class="editor-tabs"
    :class="{ 'editor-tabs--compact': compact, 'editor-tabs--select': compact && items.length }"
    v-bind="$attrs"
  >
    <label v-if="compact && items.length" class="editor-task-select"
      >{{ label }}
      <ElSelect :model-value="modelValue" :aria-label="label" @update:model-value="emit('update:modelValue', $event)">
        <ElOption v-for="item in items" :key="item.value" :value="item.value" :label="item.label" />
      </ElSelect>
    </label>
    <ElTabs
      :model-value="modelValue"
      :tab-position="compact ? 'top' : 'left'"
      :aria-label="label"
      @update:model-value="emit('update:modelValue', $event)"
      @tab-change="change"
      ><slot
    /></ElTabs>
  </div>
</template>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, type PropType } from 'vue';
import { ElTabs, ElSelect, ElOption } from 'element-plus';
defineOptions({ inheritAttrs: false });
defineProps({
  modelValue: [String, Number],
  label: { type: String, default: '配置分类' },
  items: { type: Array as PropType<{ value: string | number; label: string }[]>, default: () => [] },
});
const emit = defineEmits(['update:modelValue', 'change', 'tab-change']);
const element = ref<HTMLElement>(),
  compact = ref(window.innerWidth < 800);
let observer: ResizeObserver;
function change(value: string | number) {
  emit('change', value);
  emit('tab-change', value);
}
onMounted(() => {
  observer = new ResizeObserver(([entry]) => {
    if (entry.contentRect.width > 0) compact.value = entry.contentRect.width < 780;
  });
  if (element.value) observer.observe(element.value);
});
onBeforeUnmount(() => {
  observer?.disconnect();
});
</script>
<style scoped>
.editor-tabs {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.editor-task-select {
  display: grid;
  gap: 8px;
  flex-shrink: 0;
  padding: 12px 16px;
  font-size: 13px;
  border-bottom: 1px solid var(--hw-line);
}
.editor-tabs--select :deep(> .el-tabs > .el-tabs__header) {
  display: none;
}
.editor-tabs :deep(> .el-tabs) {
  min-width: 0;
}
.editor-tabs :deep(.el-tabs--left > .el-tabs__header) {
  margin-right: 24px;
  width: 176px;
}
.editor-tabs :deep(.el-tabs--left > .el-tabs__header .el-tabs__item) {
  justify-content: flex-start;
  text-align: left;
  min-height: 44px;
  height: auto;
  padding: 12px 16px;
  border-radius: 5px;
}
.editor-tabs :deep(.el-tabs--left > .el-tabs__header .el-tabs__item.is-active) {
  background: var(--hw-accent-soft);
}
.editor-tabs :deep(.el-tabs--left > .el-tabs__header .el-tabs__active-bar) {
  width: 2px;
}
.editor-tabs :deep(.el-tabs__content) {
  min-width: 0;
}
.editor-tabs :deep(.editor-tab-label) {
  display: grid;
  gap: 4px;
  white-space: normal;
  line-height: 1.5;
}
.editor-tabs :deep(.editor-tab-label small) {
  font-size: 12px;
  color: var(--hw-muted);
  font-weight: 400;
}
.editor-tabs--compact :deep(.editor-tab-label small) {
  display: none;
}
.editor-tabs--compact :deep(.editor-tab-label) {
  display: block;
  white-space: nowrap;
}
.editor-tabs--compact :deep(.el-tabs__item) {
  flex-shrink: 0;
  height: 44px;
}
.editor-tabs--compact :deep(.el-tabs__header) {
  margin-bottom: 20px;
}
</style>
