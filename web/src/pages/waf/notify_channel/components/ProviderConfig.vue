<template>
  <div v-if="definition?.fields.length" class="provider-config">
    <hw-alert v-if="definition.hint" theme="info" :message="definition.hint" class="provider-hint" />
    <template v-for="field in definition.fields" :key="field.key">
      <hw-form-item v-if="fieldVisible(field, form.provider_config)" :label="field.label + (field.required ? ' *' : '')">
        <div class="provider-field" :data-testid="`provider-field-${field.key}`">
          <hw-select v-if="field.kind === 'select'" v-model="form.provider_config[field.key]" :placeholder="field.options?.includes('') ? '默认 / 关闭' : '请选择'">
            <hw-option v-for="option in field.options" :key="option" :value="option" :label="option || '默认 / 关闭'" />
          </hw-select>
          <hw-switch v-else-if="field.kind === 'boolean'" v-model="form.provider_config[field.key]" />
          <hw-textarea v-else-if="field.kind === 'textarea'" v-model="form.provider_config[field.key]" :rows="4" />
          <hw-input v-else :model-value="displayValue(field)" @update:model-value="setValue(field, $event)"
            :type="field.secret ? 'password' : 'text'" :autocomplete="field.secret ? 'new-password' : 'off'"
            :placeholder="placeholder(field)" />
          <div v-if="field.hint" class="provider-tip">{{ field.hint }}</div>
          <div v-if="field.secret && form.has_config_secrets?.[field.key]" class="provider-tip">
            已配置，留空保留原值。
            <hw-button variant="text" theme="danger" size="small" @click="clearSecret(field)">
              {{ form.provider_config[field.key] === CONFIG_CLEAR ? '取消清空' : '清空已存值' }}
            </hw-button>
          </div>
        </div>
      </hw-form-item>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue';
import { CONFIG_CLEAR, fieldVisible, type ProviderDefinition, type ProviderField } from '../provider_config';

const props = defineProps({
  definition: Object as PropType<ProviderDefinition>,
  form: { type: Object, required: true },
});
const displayValue = (field: ProviderField) => props.form.provider_config[field.key] === CONFIG_CLEAR ? '' : String(props.form.provider_config[field.key] ?? '');
const setValue = (field: ProviderField, value: string) => { props.form.provider_config[field.key] = value; };
const placeholder = (field: ProviderField) => {
  if (props.form.provider_config[field.key] === CONFIG_CLEAR) return '保存后清空';
  if (field.secret && props.form.has_config_secrets?.[field.key]) return '已配置，留空保留原值';
  return field.required ? `请填写${field.label}` : '可选';
};
const clearSecret = (field: ProviderField) => {
  props.form.provider_config[field.key] = props.form.provider_config[field.key] === CONFIG_CLEAR ? '' : CONFIG_CLEAR;
};
</script>

<style scoped>
.provider-field { width: 480px; max-width: 100%; }
.provider-hint { margin: 12px 0 20px; }
.provider-tip { margin-top: 6px; font-size: 13px; line-height: 1.6; color: var(--td-text-color-secondary); overflow-wrap: anywhere; }
</style>
