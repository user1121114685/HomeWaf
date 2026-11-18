<template>
  <div class="rule-controls">
    <template v-if="imported">
      <label :for="id + '-enabled'">迁入配置中的检测开关</label>
      <ElSwitch
        :id="id + '-enabled'"
        :aria-label="name + '迁入检测开关'"
        :model-value="imported.is_enable === 1"
        :disabled="disabled || !!optionError"
        @update:model-value="setImported('is_enable', $event ? 1 : 0)"
      />
    </template>
    <ElSelect v-model="rule.mode" :aria-label="name + '模式'" :disabled="disabled || importedOff || !!optionError">
      <ElOption
        v-for="[value, label] in modes"
        :key="value"
        :value="value"
        :label="rule.category === 'ssrf' && value === 'off' ? '允许（关闭检测）' : label"
      />
    </ElSelect>
    <small v-if="optionError">请先修正检测选项中的 JSON。</small>
    <small v-else-if="importedOff">迁入开关已关闭，当前跳过此项检测。</small>
  </div>
</template>

<script setup lang="ts">
import { computed, useId, type PropType } from 'vue';
import { ElOption, ElSelect, ElSwitch } from 'element-plus';
import { fieldDraftFor } from './fieldDraft';
import { detectorOptions, modes, ruleNames, type Policy, type RuleMode } from './model';

const props = defineProps({
  rule: { type: Object as PropType<{ category: string; mode: RuleMode }>, required: true },
  ruleset: { type: Object as PropType<Policy>, required: true },
  disabled: Boolean,
});
const id = useId();
const name = computed(() => ruleNames[props.rule.category] || props.rule.category);
const optionKey = computed(() => props.rule.category + '_options');
const imported = computed(() => detectorOptions(props.ruleset, props.rule.category));
const importedOff = computed(() => imported.value != null && imported.value.is_enable !== 1);
const optionError = computed(() => fieldDraftFor(props.ruleset).jsonErrors[optionKey.value]);

function setImported(key: 'is_enable', value: number) {
  if (!imported.value || optionError.value) return;
  imported.value[key] = value;
  // The advanced JSON editor shares this draft and must display this edit too.
  delete fieldDraftFor(props.ruleset).jsonDraft[optionKey.value];
}

</script>

<style scoped>
.rule-controls {
  display: grid;
  gap: 6px;
}
.rule-controls label,
.rule-controls small {
  font-size: 12px;
  line-height: 1.5;
  color: var(--hw-muted);
}
</style>
