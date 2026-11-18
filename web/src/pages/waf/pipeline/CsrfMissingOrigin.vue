<template>
  <div class="csrf-missing-origin">
    <label :for="id">缺少 Origin / Referer</label>
    <ElSelect
      :id="id"
      v-model="missingOrigin"
      aria-label="缺少 Origin / Referer"
      :disabled="disabled || importedOff || !!optionError"
    >
      <ElOption value="allow" label="允许" />
      <ElOption v-if="!imported" value="observe" label="仅观察" />
      <ElOption value="block" label="跟随规则模式" />
    </ElSelect>
    <small>仅当 Origin 与 Referer 都缺失时生效；来源不匹配仍按 CSRF 规则模式处理。</small>
    <small v-if="optionError">请先修正检测选项中的 JSON。</small>
    <small v-else-if="importedOff">迁入开关已关闭，当前跳过此项检测。</small>
  </div>
</template>

<script setup lang="ts">
import { computed, useId, type PropType } from 'vue';
import { ElOption, ElSelect } from 'element-plus';
import { fieldDraftFor } from './fieldDraft';
import { detectorOptions, type Policy } from './model';

const props = defineProps({
  ruleset: { type: Object as PropType<Policy>, required: true },
  disabled: Boolean,
});
const id = useId();
const optionKey = 'csrf_options';
const imported = computed(() => detectorOptions(props.ruleset, 'csrf'));
const importedOff = computed(() => imported.value != null && imported.value.is_enable !== 1);
const optionError = computed(() => fieldDraftFor(props.ruleset).jsonErrors[optionKey]);

function setImported(value: number) {
  if (!imported.value || optionError.value) return;
  imported.value.allow_empty_ref = value;
  delete fieldDraftFor(props.ruleset).jsonDraft[optionKey];
}

const missingOrigin = computed<string>({
  get() {
    if (imported.value) return imported.value.allow_empty_ref === 1 ? 'allow' : 'block';
    return props.ruleset.csrf_missing_origin;
  },
  set(value) {
    if (imported.value) setImported(value === 'allow' ? 1 : 0);
    else props.ruleset.csrf_missing_origin = value;
  },
});
</script>

<style scoped>
.csrf-missing-origin {
  display: grid;
  gap: 6px;
  margin-bottom: 16px;
}

.csrf-missing-origin label,
.csrf-missing-origin small {
  color: var(--hw-muted);
  font-size: 12px;
  line-height: 1.5;
}
</style>
