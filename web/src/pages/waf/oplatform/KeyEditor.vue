<template>
  <ElDialog
    :model-value="true"
    :title="title"
    width="min(560px, 94vw)"
    :close-on-click-modal="false"
    :close-on-press-escape="!saving"
    :show-close="!saving"
    @close="emit('close')"
  >
    <ElForm label-position="top" :disabled="saving" @submit.prevent="submit">
      <ElFormItem v-if="!mcpOnly" :label="t('page.oplatform.purpose')">
        <span v-if="initial">{{ purpose === 'api' ? t('page.oplatform.open_api') : permissionLabel }}</span>
        <ElRadioGroup v-else v-model="purpose" :aria-label="t('page.oplatform.purpose')">
          <ElRadio value="api">{{ t('page.oplatform.open_api') }}</ElRadio>
          <ElRadio value="mcp">MCP</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem :label="nameLabel" required :error="nameError">
        <ElInput v-model="name" :aria-label="nameLabel" :maxlength="mcpOnly ? 100 : 255" @input="nameError = ''" />
      </ElFormItem>
      <ElFormItem v-if="purpose === 'mcp' && !initial" :label="t('page.oplatform.permissions')">
        <ElRadioGroup v-model="access" :aria-label="t('page.oplatform.permissions')">
          <ElRadio value="read">{{ t('page.oplatform.mcp_read') }}</ElRadio>
          <ElRadio value="write">{{ t('page.oplatform.mcp_write') }}</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem v-if="initial" :label="t('page.oplatform.status')">
        <ElRadioGroup v-model="status" :aria-label="t('page.oplatform.status')">
          <ElRadio :value="1">{{ t('page.oplatform.status_enabled') }}</ElRadio>
          <ElRadio :value="0">{{ t('page.oplatform.status_disabled') }}</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem :label="t('page.oplatform.source')" :required="requireSource" :error="sourceError">
        <ElInput
          v-model="source"
          :aria-label="t('page.oplatform.source')"
          :aria-required="requireSource"
          aria-describedby="key-source-help"
          placeholder="192.0.2.0/24"
          @input="sourceError = ''"
        />
        <p id="key-source-help" class="hw-form-help">
          {{ purpose === 'api' ? t('page.oplatform.ip_whitelist_placeholder') : sourceHint }}
        </p>
      </ElFormItem>
      <ElFormItem :label="t('page.oplatform.rate_limit')">
        <ElInputNumber v-model="rate" :min="mcpOnly ? 1 : 0" :max="60000" :aria-label="t('page.oplatform.rate_limit')" />
      </ElFormItem>
      <ElFormItem :label="t('page.oplatform.expire_time') + ' (UTC)'">
        <ElInput v-model="expiry" :placeholder="t('page.oplatform.expire_time_placeholder')" />
      </ElFormItem>
      <ElFormItem v-if="!mcpOnly" :label="t('common.remarks')">
        <ElInput v-model="remark" type="textarea" :rows="2" />
      </ElFormItem>
      <ElAlert v-if="error" :title="error" type="error" :closable="false" show-icon />
      <div class="key-actions">
        <ElButton :disabled="saving" @click="emit('close')">{{ t('common.cancel') }}</ElButton>
        <ElButton native-type="submit" type="primary" :loading="saving">
          {{ initial ? t('common.confirm') : t('page.oplatform.create_key') }}
        </ElButton>
      </div>
    </ElForm>
  </ElDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { useI18n } from 'vue-i18n';
import { ElAlert, ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElRadio, ElRadioGroup } from 'element-plus';
import { oplatform_key_add_api, oplatform_key_edit_api } from '@/apis/oplatform';

interface KeyDetail {
  id: string;
  key_name: string;
  mcp_access: string;
  ip_whitelist: string;
  rate_limit: number;
  expire_time: string;
  status: number;
  remark: string;
}
const props = defineProps({
  initial: { type: Object as PropType<KeyDetail>, default: undefined },
  suggestedSource: { type: String, default: '' },
  mcpOnly: Boolean,
  defaultName: { type: String, default: '' },
});
const initial = computed(() => props.initial);
const mcpOnly = computed(() => props.mcpOnly);
const emit = defineEmits<{ close: []; saved: [secret?: string] }>();
const { t } = useI18n();
const purpose = ref(props.initial?.mcp_access || props.mcpOnly ? 'mcp' : 'api');
const access = ref(props.initial?.mcp_access || (props.mcpOnly ? 'write' : 'read'));
const name = ref(props.initial?.key_name ?? props.defaultName ?? '');
const status = ref(props.initial?.status ?? 1);
const rate = ref(props.initial?.rate_limit ?? (props.mcpOnly ? 120 : 0));
const expiry = ref(props.initial?.expire_time ?? '');
const remark = ref(props.initial?.remark ?? (props.mcpOnly ? 'MCP HTTP connection' : ''));
const apiSource = ref(props.initial?.ip_whitelist ?? '');
const mcpSource = ref(props.initial?.ip_whitelist ?? '');
const source = computed({
  get: () => purpose.value === 'api' ? apiSource.value : (mcpSource.value ?? props.suggestedSource ?? ''),
  set(value: string) {
    if (purpose.value === 'api') apiSource.value = value;
    else mcpSource.value = value;
  },
});
const requireSource = computed(() => purpose.value === 'mcp' && (!props.initial || status.value !== 0 || !!props.initial.ip_whitelist));
const nameLabel = computed(() => t(props.mcpOnly ? 'page.oplatform.connection_name' : 'page.oplatform.key_name'));
const permissionLabel = computed(() => 'MCP · ' + t(access.value === 'write' ? 'page.oplatform.mcp_write' : 'page.oplatform.mcp_read'));
const title = computed(() => t(props.initial ? 'common.edit' : props.mcpOnly ? 'page.oplatform.create_mcp' : 'page.oplatform.new_key'));
const sourceHint = computed(() => {
  if (props.initial) return t('page.oplatform.source_edit_hint');
  return props.suggestedSource ? t('page.oplatform.source_hint', { source: props.suggestedSource }) : t('page.oplatform.source_manual');
});
const saving = ref(false), nameError = ref(''), sourceError = ref(''), error = ref('');
watch([purpose, status], () => { sourceError.value = ''; error.value = ''; });
watch([purpose, () => props.suggestedSource], () => {
  if (purpose.value === 'mcp' && !mcpSource.value) mcpSource.value = props.suggestedSource || '';
}, { immediate: true });

async function submit() {
  if (saving.value) return;
  nameError.value = name.value.trim() ? '' : t(props.mcpOnly ? 'page.oplatform.connection_required' : 'page.oplatform.key_name_required');
  const sources = source.value.split(/[,，]/).map((item) => item.trim()).filter(Boolean);
  sourceError.value = requireSource.value && !sources.length ? t('page.oplatform.source_required') : '';
  if (nameError.value || sourceError.value) return;
  saving.value = true;
  error.value = '';
  const fields = { key_name: name.value.trim(), rate_limit: rate.value, expire_time: expiry.value.trim(), remark: remark.value, ip_whitelist: sources.join(',') };
  try {
    const res = props.initial
      ? await oplatform_key_edit_api({ ...fields, id: props.initial.id, status: status.value })
      : await oplatform_key_add_api({ ...fields, mcp_access: purpose.value === 'mcp' ? access.value : '' });
    if (res.code !== 0 || (!props.initial && !res.data?.api_key)) throw new Error(res.msg || t('common.tips.save_failed'));
    emit('saved', props.initial ? undefined : res.data.api_key);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('common.tips.save_failed');
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.key-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
.key-actions .el-button { margin: 0; }
#key-source-help { margin: 4px 0 0; }
</style>
