<template>
  <section class="service-tls" aria-label="HTTPS 证书" :aria-busy="loading || checking">
    <h3>HTTPS 证书</h3>
    <p class="tls-help">选择证书管理中的证书，保存服务时绑定，并检查有效期与域名/IP 匹配情况。</p>
    <p v-if="!hosts.length" class="tls-help">
      请先填写访客使用的域名或 IP，才能自动匹配证书；主机名留空时仅检查证书与有效期。
    </p>
    <ElRadioGroup
      v-if="revision"
      :model-value="mode"
      aria-label="证书来源"
      @update:model-value="emit('update:mode', String($event))"
    >
      <ElRadio v-if="revision" value="current">保留当前证书</ElRadio>
      <ElRadio value="stored">使用已有证书</ElRadio>
    </ElRadioGroup>
    <p v-if="loading" role="status">正在查找匹配证书…</p>
    <p v-if="loadError" class="tls-error" role="alert">
      {{ loadError }}
    </p>
    <template v-if="mode === 'stored'">
      <label class="tls-field"
        ><span>域名证书</span>
        <ElSelect
          v-model="secrets.tls_certificate_id"
          aria-label="域名证书"
          placeholder="选择匹配域名或 IP 的证书"
          :loading="loading"
          :disabled="loading || !!loadError"
          clearable
        >
          <ElOption
            v-for="certificate in certificates"
            :key="certificate.id"
            :value="certificate.id"
            :disabled="!certificate.valid"
            :label="optionLabel(certificate)"
          />
        </ElSelect>
      </label>
      <p v-if="!loading && !loadError && !certificates.length" role="status">
        还没有已保存的证书，请前往证书管理申请或导入。
      </p>
      <p v-else-if="!loading && !loadError && !certificates.some((certificate) => certificate.valid)" role="status">
        没有可用的匹配证书，请核对域名或 IP；已过期、尚未生效或不匹配的证书不能使用。
      </p>
      <p v-if="selected" :class="selected.valid ? 'tls-help' : 'tls-error'" role="status">
        {{
          selected.valid
            ? `已匹配：${selected.domains.join('、')}；到期时间 ${expiry(selected.expires_at)}`
            : selected.reason
        }}
      </p>
    </template>
    <template v-else-if="mode === 'current' && !loading && !loadError">
      <p v-if="current" :class="current.valid ? 'tls-help' : 'tls-error'" role="status">
        {{ current.valid ? `当前证书检查通过；到期时间 ${expiry(current.expires_at)}` : current.reason }}
      </p>
      <p v-else class="tls-error" role="alert">当前服务没有证书，请从证书管理准备证书后刷新列表。</p>
    </template>
    <div class="tls-actions">
      <RouterLink :to="certificateRoot" target="_blank" rel="noopener noreferrer">前往证书管理</RouterLink>
      <ElButton :loading="loading" @click="loadOptions">刷新证书列表</ElButton>
    </div>
    <p class="tls-help">证书管理将在新页面打开，当前草稿保留。完成后回到这里刷新证书列表。</p>
    <p v-if="error" ref="errorSummary" tabindex="-1" class="tls-error" role="alert">{{ error }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { RouterLink } from 'vue-router';
import { ElButton, ElOption, ElRadio, ElRadioGroup, ElSelect } from 'element-plus';
import { certificateRoot } from '@/apis/certificates';
import { pipelinePost } from '@/apis/pipeline';
type Certificate = { id: string; domains: string[]; expires_at: string; valid: boolean; reason?: string };
const props = defineProps({
  port: { type: Number, required: true },
  revision: { type: Number, required: true },
  hosts: { type: Array as PropType<string[]>, required: true },
  mode: { type: String, required: true },
  secrets: {
    type: Object as PropType<{ tls_certificate_id: string }>,
    required: true,
  },
});
const emit = defineEmits(['update:mode']);
const certificates = ref<Certificate[]>([]),
  current = ref<Certificate>();
const loading = ref(false),
  checking = ref(false),
  loadError = ref(''),
  error = ref('');
const errorSummary = ref<HTMLElement>();
const selected = computed(() =>
  certificates.value.find((certificate) => certificate.id === props.secrets.tls_certificate_id),
);
const expiry = (value: string) => (value ? new Date(value).toLocaleString() : '未知');
const optionLabel = (certificate: Certificate) =>
  `${certificate.domains.join('、') || '无法读取域名'} · ${certificate.valid ? `有效至 ${expiry(certificate.expires_at)}` : certificate.reason}`;
let generation = 0,
  timer: ReturnType<typeof setTimeout> | undefined;
const identity = () => ({ port: props.revision ? props.port : 0, revision: props.revision, host_names: props.hosts });
async function loadOptions() {
  clearTimeout(timer);
  const request = ++generation;
  loading.value = true;
  loadError.value = '';
  try {
    const result = await pipelinePost<{ certificates: Certificate[]; current?: Certificate }>(
      'tls/options',
      identity(),
    );
    if (request !== generation) return;
    certificates.value = result.certificates;
    current.value = result.current;
    const useStored = props.mode === 'stored' || (props.mode === 'current' && !result.current);
    if (props.mode === 'current' && !result.current) emit('update:mode', 'stored');
    const matches = result.certificates.filter((certificate) => certificate.valid);
    if (useStored && !props.secrets.tls_certificate_id && props.hosts.length && matches.length === 1)
      props.secrets.tls_certificate_id = matches[0].id;
  } catch (cause: any) {
    if (request === generation) loadError.value = cause?.message || '读取证书失败，请重试。';
  } finally {
    if (request === generation) loading.value = false;
  }
}
watch(
  () => JSON.stringify(identity()),
  () => {
    ++generation;
    loading.value = true;
    clearTimeout(timer);
    timer = setTimeout(loadOptions, 250);
  },
  { immediate: true, flush: 'sync' },
);
const selectionKey = () => JSON.stringify({ ...identity(), mode: props.mode, ...props.secrets });
watch(
  selectionKey,
  () => {
    error.value = '';
  },
  { flush: 'sync' },
);
async function validate(): Promise<boolean> {
  if (checking.value) return false;
  checking.value = true;
  error.value = '';
  const key = selectionKey();
  try {
    if (props.mode === 'stored' && !props.secrets.tls_certificate_id)
      throw new Error('请选择证书；没有可用证书时，请先前往证书管理。');
    await pipelinePost('tls/check', {
      ...identity(),
      tls_certificate_id: props.mode === 'stored' ? props.secrets.tls_certificate_id : '',
    });
    if (key !== selectionKey()) return false;
    return true;
  } catch (cause: any) {
    if (key !== selectionKey()) return false;
    error.value = cause?.message || '证书检查失败，请重试。';
    await nextTick();
    errorSummary.value?.focus();
    return false;
  } finally {
    checking.value = false;
  }
}
onBeforeUnmount(() => {
  ++generation;
  clearTimeout(timer);
});
function focusError() {
  if (error.value) errorSummary.value?.focus();
}
defineExpose({ validate, focusError });
</script>

<style scoped>
.service-tls {
  margin-top: 20px;
  min-width: 0;
}
.service-tls h3 {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
}
.service-tls p {
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.tls-help {
  color: var(--el-text-color-regular);
  font-size: 13px;
}
.tls-error {
  color: var(--el-color-danger);
}
.tls-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  margin: 16px 0;
}
.tls-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}
.tls-actions a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--el-color-primary);
}
.tls-actions a:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
.service-tls :deep(.el-radio-group) {
  display: flex;
  flex-wrap: wrap;
  height: auto;
}
.service-tls :deep(.el-radio) {
  min-height: 44px;
  margin-right: 20px;
  white-space: normal;
}
.service-tls :deep(.el-button),
.service-tls :deep(.el-select__wrapper) {
  min-height: 44px;
}
</style>
