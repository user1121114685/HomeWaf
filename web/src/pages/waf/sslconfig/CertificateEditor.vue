<!-- HomeWaf modification (2026-11-16): certificate editing and recovery. -->
<template>
  <ElDialog
    v-model="visible"
    :title="t(isEdit ? 'page.ssl.editor.edit' : 'page.ssl.editor.add')"
    width="820px"
    class="certificate-editor"
    data-testid="certificate-editor"
    append-to-body
    :close-on-click-modal="false"
    :close-on-press-escape="!busy && !reading"
    :show-close="!busy && !reading"
    :before-close="beforeClose"
    destroy-on-close
    @closed="reset"
  >
    <section v-if="isEdit" class="certificate-identity" :aria-label="t('page.ssl.editor.current')">
      <h3>{{ current.domains || current.subject || current.id }}</h3>
      <dl>
        <div>
          <dt>{{ t('page.ssl.label_valid_to') }}</dt>
          <dd>{{ current.valid_to }} · {{ current.expiration_info }}</dd>
        </div>
        <div>
          <dt>{{ t('page.ssl.label_issuer') }}</dt>
          <dd>{{ current.issuer || '—' }}</dd>
        </div>
        <div class="certificate-identity-wide">
          <dt>{{ t('page.ssl.label_bind_hosts') }}</dt>
          <dd>{{ current.bind_hosts?.join('、') || t('page.ssl.editor.unbound') }}</dd>
        </div>
      </dl>
      <p v-if="contentChanged" class="certificate-hint" role="status">{{ t('page.ssl.editor.metadataNotice') }}</p>
    </section>
    <div v-if="saveError" ref="errorSummary" class="certificate-error" role="alert" tabindex="-1">
      <strong>{{ saveError }}</strong>
      <p>{{ t('page.ssl.editor.retryHint') }}</p>
      <a
        v-for="(message, field) in fieldErrors"
        :key="field"
        :href="`#certificate-${field}`"
        @click.prevent="focusField(field)"
        >{{ message }}</a
      >
    </div>
    <ElForm
      id="certificate-editor-form"
      ref="form"
      :model="draft"
      :rules="rules"
      :disabled="busy || reading"
      label-position="top"
      :aria-busy="busy || reading"
      @submit.prevent="save"
    >
      <ElFormItem
        :label="t('page.ssl.editor.certificate')"
        prop="cert_content"
        :error="fieldErrors.cert_content"
        for="certificate-cert_content"
      >
        <div class="certificate-field">
          <div class="certificate-field-tools">
            <p id="certificate-cert-help" class="certificate-hint">{{ t('page.ssl.editor.certHint') }}</p>
            <ElButton @click="certFile?.click()">{{ t('page.ssl.import_cert_file') }}</ElButton>
          </div>
          <ElInput
            id="certificate-cert_content"
            v-model="draft.cert_content"
            type="textarea"
            :rows="7"
            class="certificate-pem"
            spellcheck="false"
            autocomplete="off"
            aria-describedby="certificate-cert-help certificate-cert-error"
            :placeholder="certExample"
            @input="clearError('cert_content')"
          />
        </div>
        <template #error="{ error }">
          <p id="certificate-cert-error" class="el-form-item__error certificate-inline-error" role="alert">
            {{ error }}
          </p>
        </template>
      </ElFormItem>
      <ElFormItem
        :label="t('page.ssl.editor.key')"
        prop="key_content"
        :error="fieldErrors.key_content"
        for="certificate-key_content"
      >
        <div class="certificate-field">
          <div class="certificate-field-tools">
            <p id="certificate-key-help" class="certificate-hint">{{ t('page.ssl.editor.keyHint') }}</p>
            <ElButton @click="keyFile?.click()">{{ t('page.ssl.import_key_file') }}</ElButton>
          </div>
          <ElInput
            id="certificate-key_content"
            v-model="draft.key_content"
            type="textarea"
            :rows="5"
            class="certificate-pem"
            spellcheck="false"
            autocomplete="off"
            aria-describedby="certificate-key-help certificate-key-error"
            :placeholder="keyExample"
            @input="clearError('key_content')"
          />
        </div>
        <template #error="{ error }">
          <p id="certificate-key-error" class="el-form-item__error certificate-inline-error" role="alert">
            {{ error }}
          </p>
        </template>
      </ElFormItem>
      <p v-if="readNotice || reading" class="certificate-hint" role="status">
        {{ reading ? t('page.ssl.editor.reading') : readNotice }}
      </p>
      <template v-if="isEdit">
        <details class="certificate-settings">
          <summary>
            <span>{{ t('page.ssl.editor.fileSettings') }}</span>
            <span class="certificate-hint"
              >{{ t(draft.auto_load_path ? 'page.ssl.editor.loadOn' : 'page.ssl.editor.loadOff') }} ·
              {{ t(exportEnabled ? 'page.ssl.editor.exportOn' : 'page.ssl.editor.exportOff') }}</span
            >
          </summary>
          <EditorSection :title="t('page.ssl.editor.autoLoad')" :description="t('page.ssl.label_import_tip')">
            <ElFormItem prop="auto_load_path">
              <ElCheckbox
                v-model="draft.auto_load_path"
                :true-value="1"
                :false-value="0"
                aria-describedby="certificate-auto-help"
                >{{ t('page.ssl.label_auto_load_path_switch') }}</ElCheckbox
              >
              <p id="certificate-auto-help" class="certificate-hint">{{ t('page.ssl.label_auto_load_path_tip') }}</p>
            </ElFormItem>
            <ElFormItem :label="t('page.ssl.label_auto_crt_path')" prop="cert_path"
              ><ElInput v-model="draft.cert_path" clearable
            /></ElFormItem>
            <ElFormItem :label="t('page.ssl.label_auto_key_path')" prop="key_path"
              ><ElInput v-model="draft.key_path" clearable
            /></ElFormItem>
          </EditorSection>
          <EditorSection :title="t('page.ssl.editor.export')" :description="t('page.ssl.label_export_tip')">
            <ElFormItem :label="t('page.ssl.label_export_crt_path')" prop="export_cert_path"
              ><ElInput v-model="draft.export_cert_path" clearable aria-describedby="certificate-export-help"
            /></ElFormItem>
            <ElFormItem :label="t('page.ssl.label_export_key_path')" prop="export_key_path"
              ><ElInput v-model="draft.export_key_path" clearable aria-describedby="certificate-export-help"
            /></ElFormItem>
            <p id="certificate-export-help" class="certificate-hint">{{ t('page.ssl.label_export_path_tip') }}</p>
          </EditorSection>
        </details>
        <p v-if="current.export_status" class="certificate-export-result">
          <strong>{{ t('page.ssl.label_export_status') }}：</strong>{{ current.export_status }}
        </p>
      </template>
    </ElForm>
    <input ref="certFile" type="file" accept=".crt,.pem,.cer" hidden @change="readFile($event, 'cert_content')" />
    <input ref="keyFile" type="file" accept=".key,.pem" hidden @change="readFile($event, 'key_content')" />
    <template #footer>
      <p class="certificate-save-hint">{{ t('page.ssl.editor.saveHint') }}</p>
      <div class="certificate-actions">
        <ElButton :disabled="busy || reading" @click="beforeClose(() => (visible = false))">{{
          t('common.cancel')
        }}</ElButton>
        <ElButton
          type="primary"
          native-type="submit"
          form="certificate-editor-form"
          :loading="busy"
          :disabled="reading"
          >{{ t('page.ssl.editor.save') }}</ElButton
        >
      </div>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { onBeforeRouteLeave } from 'vue-router';
import { useI18n } from 'vue-i18n';
import {
  ElButton,
  ElCheckbox,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessageBox,
  type FormInstance,
  type FormRules,
} from 'element-plus';
import EditorSection from '@/ui/EditorSection.vue';
import { sslConfigAddApi, sslConfigEditApi } from '@/apis/sslconfig';

type PemField = 'cert_content' | 'key_content';
interface CertificateDraft {
  cert_content: string;
  key_content: string;
  cert_path: string;
  key_path: string;
  auto_load_path: number;
  export_cert_path: string;
  export_key_path: string;
}
interface CertificateRecord extends CertificateDraft {
  id: string;
  domains?: string;
  subject?: string;
  issuer?: string;
  valid_to?: string;
  expiration_info?: string;
  bind_hosts?: string[];
  export_status?: string;
}
const { t } = useI18n();
const emit = defineEmits<{
  saved: [result: { id?: string; message: string; exportFailed: boolean }];
  'visibility-change': [visible: boolean];
}>();
const emptyDraft = (): CertificateDraft => ({
  cert_content: '',
  key_content: '',
  cert_path: '',
  key_path: '',
  auto_load_path: 1,
  export_cert_path: '',
  export_key_path: '',
});
const draft = reactive(emptyDraft());
const current = ref<Partial<CertificateRecord>>({});
const visible = ref(false),
  busy = ref(false),
  reading = ref(false);
const snapshot = ref(''),
  originalContent = ref('');
watch(visible, (value) => emit('visibility-change', value), { flush: 'sync' });
const saveError = ref(''),
  readNotice = ref('');
const fieldErrors = ref<Partial<Record<PemField, string>>>({});
const form = ref<FormInstance>();
const errorSummary = ref<HTMLElement>();
const certFile = ref<HTMLInputElement>(),
  keyFile = ref<HTMLInputElement>();
let readVersion = 0;
let discardPrompt: Promise<boolean> | undefined;
const isEdit = computed(() => Boolean(current.value.id));
const dirty = computed(() => visible.value && JSON.stringify(draft) !== snapshot.value);
const contentChanged = computed(
  () => isEdit.value && JSON.stringify([draft.cert_content, draft.key_content]) !== originalContent.value,
);
const exportEnabled = computed(() => Boolean(draft.export_cert_path.trim() || draft.export_key_path.trim()));
const certExample = '-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----';
const keyExample = '-----BEGIN PRIVATE KEY-----\n…\n-----END PRIVATE KEY-----';
const rules = computed<FormRules>(() => ({
  cert_content: [
    { required: true, message: t('page.ssl.editor.certRequired'), trigger: 'blur' },
    {
      pattern: /-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/,
      message: t('page.ssl.editor.certInvalid'),
      trigger: 'blur',
    },
  ],
  key_content: [
    { required: true, message: t('page.ssl.editor.keyRequired'), trigger: 'blur' },
    {
      pattern: /-----BEGIN ((?:RSA |EC )?PRIVATE KEY)-----[\s\S]+?-----END \1-----/,
      message: t('page.ssl.editor.keyInvalid'),
      trigger: 'blur',
    },
  ],
}));

async function open(record?: CertificateRecord) {
  if (visible.value) return;
  current.value = record ? { ...record, bind_hosts: [...(record.bind_hosts || [])] } : {};
  Object.assign(draft, emptyDraft());
  if (record)
    for (const key of Object.keys(draft) as (keyof CertificateDraft)[]) {
      const value = record[key];
      if (key === 'auto_load_path') draft[key] = value === 0 ? 0 : 1;
      else draft[key] = typeof value === 'string' ? value : '';
    }
  snapshot.value = JSON.stringify(draft);
  originalContent.value = JSON.stringify([draft.cert_content, draft.key_content]);
  saveError.value = '';
  fieldErrors.value = {};
  readNotice.value = '';
  visible.value = true;
  await nextTick();
  form.value?.clearValidate();
}
function reset() {
  if (visible.value) return;
  readVersion++;
  Object.assign(draft, emptyDraft());
  current.value = {};
  snapshot.value = originalContent.value = '';
  saveError.value = readNotice.value = '';
  fieldErrors.value = {};
}
function clearError(field: PemField) {
  delete fieldErrors.value[field];
  saveError.value = '';
  readNotice.value = '';
}
function focusField(field: string) {
  const element = document.getElementById(`certificate-${field}`);
  element?.focus();
  element?.scrollIntoView({ block: 'nearest' });
}
async function showError(message: string) {
  saveError.value = message;
  await nextTick();
  errorSummary.value?.focus();
  errorSummary.value?.scrollIntoView({ block: 'nearest' });
}
async function save() {
  if (busy.value || reading.value) return;
  busy.value = true;
  saveError.value = '';
  fieldErrors.value = {};
  try {
    try {
      await form.value!.validate();
    } catch (invalid: any) {
      for (const field of ['cert_content', 'key_content'] as const) {
        if (invalid[field]?.[0]?.message) fieldErrors.value[field] = invalid[field][0].message;
      }
      await showError(t('page.ssl.editor.checkFields'));
      return;
    }
    const payload = { ...draft };
    // New certificates retain the existing server defaults for path loading.
    const result = isEdit.value
      ? await sslConfigEditApi({ id: current.value.id, ...payload }, { background: true })
      : await sslConfigAddApi(payload, { background: true });
    if (result.code !== 0) {
      const errors = result.data?.field_errors || {};
      for (const field of ['cert_content', 'key_content'] as const) {
        if (typeof errors[field] === 'string') fieldErrors.value[field] = errors[field];
      }
      await showError(result.msg || t('page.ssl.editor.saveFailed'));
      return;
    }
    snapshot.value = JSON.stringify(draft);
    emit('saved', {
      id: result.data?.id || current.value.id,
      message: result.msg || t('page.ssl.editor.saved'),
      exportFailed: result.data?.export_failed === true || Boolean(result.msg?.includes('但证书导出失败')),
    });
    visible.value = false;
  } catch {
    await showError(t('page.ssl.editor.networkError'));
  } finally {
    busy.value = false;
  }
}
async function readFile(event: Event, field: PemField) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file || busy.value || reading.value) return;
  clearError(field);
  if (file.size > 2 * 1024 * 1024) {
    fieldErrors.value[field] = t('page.ssl.editor.fileTooLarge');
    await showError(t('page.ssl.editor.readFailed'));
    return;
  }
  reading.value = true;
  const version = ++readVersion;
  try {
    const content = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result || ''));
      reader.onerror = () => reject(reader.error);
      reader.onabort = () => reject(new Error('Aborted'));
      reader.readAsText(file);
    });
    if (version !== readVersion || !visible.value) return;
    draft[field] = content;
    readNotice.value = t('page.ssl.editor.fileRead', { name: file.name });
    await form.value?.validateField(field).catch(() => undefined);
  } catch {
    if (version === readVersion && visible.value) await showError(t('page.ssl.editor.readFailed'));
  } finally {
    if (version === readVersion) reading.value = false;
  }
}
async function canClose(): Promise<boolean> {
  if (busy.value || reading.value) return false;
  if (!dirty.value) return true;
  if (!discardPrompt) {
    discardPrompt = ElMessageBox.confirm(t('page.ssl.editor.discardHint'), t('page.ssl.editor.unsaved'), {
      confirmButtonText: t('page.ssl.editor.discard'),
      cancelButtonText: t('page.ssl.editor.continue'),
      type: 'warning',
      closeOnClickModal: false,
      closeOnHashChange: false,
      distinguishCancelAndClose: true,
    })
      .then(
        () => true,
        () => false,
      )
      .finally(() => {
        discardPrompt = undefined;
      });
  }
  return discardPrompt;
}
async function beforeClose(done: () => void) {
  if (await canClose()) done();
}
onBeforeRouteLeave(async () => {
  if (!visible.value) return true;
  if (!(await canClose())) return false;
  visible.value = false;
  reset();
  return true;
});
function beforeUnload(event: BeforeUnloadEvent) {
  if (visible.value && (dirty.value || busy.value || reading.value)) {
    event.preventDefault();
    event.returnValue = '';
  }
}
window.addEventListener('beforeunload', beforeUnload);
onBeforeUnmount(() => {
  readVersion++;
  window.removeEventListener('beforeunload', beforeUnload);
});
defineExpose({ open });
</script>

<style scoped>
:global(.certificate-editor.el-dialog) {
  display: flex;
  flex-direction: column;
  max-height: 88dvh;
}
:global(.certificate-editor .el-dialog__body) {
  min-height: 0;
  flex: 1;
  max-height: none;
}
:global(.certificate-editor .el-dialog__header),
:global(.certificate-editor .el-dialog__footer) {
  flex-shrink: 0;
}
:global(.certificate-editor .el-dialog__footer) {
  align-items: center;
}
.certificate-identity {
  margin-bottom: 24px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--hw-line);
}
.certificate-identity h3 {
  margin: 0 0 16px;
  font-size: 18px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.certificate-identity dl {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 24px;
  margin: 0;
}
.certificate-identity-wide {
  grid-column: 1 / -1;
}
.certificate-identity dt,
.certificate-hint {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.65;
}
.certificate-identity dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}
.certificate-field {
  width: 100%;
  min-width: 0;
}
.certificate-field-tools {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 10px;
}
.certificate-field-tools .certificate-hint {
  flex: 1;
  margin: 0;
}
.certificate-field-tools .el-button {
  flex: none;
}
.certificate-pem :deep(textarea) {
  font-family: var(--font-mono);
  line-height: 1.65;
  resize: vertical;
}
.certificate-hint {
  margin: 8px 0 0;
  width: 100%;
  overflow-wrap: anywhere;
}
.certificate-error {
  margin-bottom: 20px;
  padding: 12px 16px;
  border-left: 3px solid var(--hw-danger);
  background: var(--hw-subtle);
  overflow-wrap: anywhere;
}
.certificate-error strong {
  color: var(--hw-danger);
}
.certificate-error p {
  margin: 6px 0;
}
.certificate-error a {
  display: block;
  color: var(--hw-accent);
  text-decoration: underline;
}
.certificate-inline-error {
  position: static;
  width: 100%;
  margin: 0;
  padding-top: 6px;
  line-height: 1.5;
}
.certificate-settings {
  border-top: 1px solid var(--hw-line);
  margin-top: 24px;
}
.certificate-settings summary {
  padding: 18px 0;
  cursor: pointer;
  line-height: 1.7;
  font-weight: 500;
}
.certificate-settings summary .certificate-hint {
  display: block;
  margin-top: 4px;
  font-weight: 400;
}
.certificate-export-result {
  margin: 0 0 8px;
  color: var(--hw-ink);
  font-size: 13px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}
.certificate-save-hint {
  flex: 1 1 260px;
  margin: 0;
  color: var(--hw-muted);
  font-size: 13px;
  text-align: left;
  line-height: 1.6;
}
.certificate-actions {
  display: flex;
  gap: 8px;
}
.certificate-actions .el-button + .el-button {
  margin-left: 0;
}
@media (max-width: 700px) {
  .certificate-identity dl {
    grid-template-columns: minmax(0, 1fr);
  }
  .certificate-field-tools {
    flex-direction: column;
    gap: 10px;
  }
  .certificate-actions {
    width: 100%;
  }
  .certificate-actions .el-button {
    flex: 1;
    min-height: 44px;
  }
  .certificate-field-tools .el-button {
    min-height: 44px;
  }
}
</style>
