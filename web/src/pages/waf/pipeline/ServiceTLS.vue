<template>
  <section class="service-tls" aria-label="HTTPS 证书" :aria-busy="loading || checking">
    <h3>HTTPS 证书</h3>
    <p class="tls-help">
      使用证书管理中的证书，按域名或 IP 检查覆盖范围和有效期。服务保存后建立绑定，证书更新会同步到服务。
    </p>
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
      {{ loadError }} <ElButton :disabled="loading" @click="loadOptions">重新读取证书</ElButton>
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
        还没有已保存的证书。点击“新增证书”导入证书与私钥，保存后即可选择。
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
      <ElButton v-if="!loadError" :disabled="loading" @click="loadOptions">重新读取证书</ElButton>
    </template>
    <template v-else-if="mode === 'current' && !loading && !loadError">
      <p v-if="current" :class="current.valid ? 'tls-help' : 'tls-error'" role="status">
        {{ current.valid ? `当前证书检查通过；到期时间 ${expiry(current.expires_at)}` : current.reason }}
      </p>
      <p v-else class="tls-error" role="alert">当前服务没有证书，请选择已有证书或新增证书。</p>
    </template>
    <div class="tls-create">
      <ElButton :disabled="loading || !canCreate" @click="certificateEditor?.open()">新增证书</ElButton>
    </div>
    <p v-if="!loading && !loadError && !canCreate" class="tls-help">新增证书需要系统管理员权限；你可以选择已有证书。</p>
    <p class="tls-help">新增证书会立即保存到证书管理；服务绑定在保存服务时生效，取消服务草稿不会删除已新增的证书。</p>
    <p v-if="createdNotice" role="status">{{ createdNotice }}</p>
    <CertificateEditor ref="certificateEditor" @saved="certificateSaved" @visibility-change="emit('busy', $event)" />
    <div class="tls-check">
      <ElButton :loading="checking" :disabled="loading" @click="validate">检查证书</ElButton
      ><span v-if="checked" role="status">证书检查通过，保存时会再次校验。</span>
    </div>
    <p v-if="error" ref="errorSummary" tabindex="-1" class="tls-error" role="alert">{{ error }}</p>
    <p class="tls-help">检查通过不代表已启用 HTTPS，也不验证客户端是否信任颁发机构；启用后仍需实际访问确认。</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { ElButton, ElOption, ElRadio, ElRadioGroup, ElSelect } from 'element-plus';
import CertificateEditor from '../sslconfig/CertificateEditor.vue';
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
const emit = defineEmits(['update:mode', 'busy']);
const certificateEditor = ref<InstanceType<typeof CertificateEditor>>(),
  canCreate = ref(false),
  createdNotice = ref('');
const certificates = ref<Certificate[]>([]),
  current = ref<Certificate>();
const loading = ref(false),
  checking = ref(false),
  checked = ref(false),
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
    const result = await pipelinePost<{ certificates: Certificate[]; current?: Certificate; can_create: boolean }>(
      'tls/options',
      identity(),
    );
    if (request !== generation) return;
    certificates.value = result.certificates;
    current.value = result.current;
    canCreate.value = result.can_create;
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
    checked.value = false;
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
    checked.value = false;
    error.value = '';
  },
  { flush: 'sync' },
);
async function validate(): Promise<boolean> {
  if (checking.value) return false;
  checking.value = true;
  error.value = '';
  checked.value = false;
  const key = selectionKey();
  try {
    if (props.mode === 'stored' && !props.secrets.tls_certificate_id)
      throw new Error('请从证书管理选择匹配域名或 IP 的证书，或新增证书。');
    await pipelinePost('tls/check', {
      ...identity(),
      tls_certificate_id: props.mode === 'stored' ? props.secrets.tls_certificate_id : '',
    });
    if (key !== selectionKey()) return false;
    checked.value = true;
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
async function certificateSaved(result: { id?: string; message: string; exportFailed: boolean }) {
  createdNotice.value = result.exportFailed ? result.message : '证书已保存到证书管理，服务草稿已保留。';
  if (result.id) {
    props.secrets.tls_certificate_id = result.id;
    emit('update:mode', 'stored');
  }
  await loadOptions();
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
.tls-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}
.tls-columns p {
  grid-column: 1 / -1;
}
.tls-check {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
}
.tls-create {
  margin-top: 12px;
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
@media (max-width: 600px) {
  .tls-columns {
    grid-template-columns: 1fr;
  }
}
</style>
