<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { zeroSSLAccountStatusApi, saveZeroSSLAccountApi, type ZeroSSLAccountStatus } from '@/apis/sslorder';

const props = defineProps({ revision: { type: Number, default: 0 } });
const emit = defineEmits(['status', 'saved']);
const account = ref<ZeroSSLAccountStatus | null>(null);
const loading = ref(false);
const error = ref('');
const editing = ref(false);
const saving = ref(false);
const saveError = ref('');
const draft = reactive({ mode: 'access_key', access_key: '', eab_kid: '', eab_hmac_key: '' });
const eabComplete = computed(() => account.value?.has_eab_kid && account.value?.has_eab_hmac_key);
let requestSequence = 0;

async function load() {
  const sequence = ++requestSequence;
  loading.value = true;
  error.value = '';
  emit('status', 'checking');
  try {
    const result = await zeroSSLAccountStatusApi();
    if (sequence !== requestSequence) return;
    if (result.code !== 0 || typeof result.data?.configured !== 'boolean') throw new Error('读取失败');
    account.value = result.data;
    emit('status', account.value.configured ? 'configured' : 'missing');
  } catch {
    if (sequence !== requestSequence) return;
    error.value = 'ZeroSSL 认证状态读取失败，请重试后再申请。';
    emit('status', 'error');
  } finally {
    if (sequence === requestSequence) loading.value = false;
  }
}

function openEditor() {
  Object.assign(draft, {
    mode: eabComplete.value ? 'eab' : 'access_key',
    access_key: '',
    eab_kid: '',
    eab_hmac_key: '',
  });
  saveError.value = '';
  editing.value = true;
}

async function save() {
  if (saving.value) return;
  const accessKey = draft.access_key.trim(),
    kid = draft.eab_kid.trim(),
    hmac = draft.eab_hmac_key.trim();
  if (draft.mode === 'access_key' ? !accessKey : !kid || !hmac) {
    saveError.value =
      draft.mode === 'access_key' ? '请填写 ZeroSSL API Access Key。' : '请同时填写 EAB Key ID 和 HMAC Key。';
    return;
  }
  saving.value = true;
  saveError.value = '';
  try {
    const result = await saveZeroSSLAccountApi(
      draft.mode === 'access_key'
        ? { mode: 'access_key', access_key: accessKey }
        : { mode: 'eab', eab_kid: kid, eab_hmac_key: hmac },
    );
    if (result.code !== 0) {
      saveError.value = result.msg || 'ZeroSSL 认证保存失败。';
      return;
    }
    account.value = result.data;
    Object.assign(draft, { access_key: '', eab_kid: '', eab_hmac_key: '' });
    editing.value = false;
    emit('status', 'configured');
    emit('saved');
  } catch {
    saveError.value = 'ZeroSSL 认证保存失败，输入已保留，请重试。';
  } finally {
    saving.value = false;
  }
}

watch(() => props.revision, load, { immediate: true });
</script>

<template>
  <section class="zerossl-account" aria-label="ZeroSSL 账户认证" data-testid="zerossl-account">
    <p v-if="loading" role="status">正在检查 ZeroSSL 账户认证配置…</p>
    <div v-else-if="error" role="alert">
      <p>{{ error }}</p>
      <hw-button variant="outline" @click="load">重试认证检查</hw-button>
    </div>
    <template v-else>
      <div v-if="!account?.configured" class="zerossl-warning" role="alert">
        <strong>ZeroSSL 尚未配置账户认证，暂时不能申请。</strong>
        <p v-if="account?.has_eab_kid || account?.has_eab_hmac_key">
          当前 EAB 凭据不完整，需要同时配置 Key ID 和 HMAC Key，或改用 API Access Key。
        </p>
        <p v-else>需要填写 ZeroSSL API Access Key，或一组 EAB Key ID / HMAC Key。</p>
      </div>
      <div v-else role="status">
        <strong>ZeroSSL 账户认证已配置</strong>
        <p>
          {{
            eabComplete ? '优先使用已保存的 EAB 凭据。' : '申请时通过 API Access Key 自动获取 EAB 凭据。'
          }}凭据有效性会在申请时校验。
        </p>
      </div>
      <p>这是证书机构的账户认证，与 DNS 服务商的 Token 不同。仅编辑 CA 名称和接口地址不会配置密钥。</p>
      <div class="zerossl-actions">
        <hw-button theme="primary" variant="outline" @click="openEditor">{{
          account?.configured ? '修改 ZeroSSL 认证' : '配置 ZeroSSL 认证'
        }}</hw-button>
        <hw-button variant="text" @click="load">重新检查</hw-button>
      </div>
      <p class="zerossl-note">
        申请页、CA 编辑页与“系统配置”中的 zerossl 使用同一组认证配置，任意入口保存后无需重复填写。由系统管理员保存。选择
        Let’s Encrypt 无需这项配置，DNS 验证仍无需入站 80/443。
      </p>
    </template>
  </section>
  <hw-dialog
    v-model:visible="editing"
    header="ZeroSSL 账户认证"
    :width="620"
    :footer="false"
    attach="body"
    :close-on-overlay-click="false"
    :close-on-esc-keydown="!saving"
  >
    <template #body>
      <div class="zerossl-editor">
        <p>此处保存认证配置，与 CA 的名称和接口地址分开保存。已保存的密钥不会回显；取消编辑会保留原配置。</p>
        <a
          class="zerossl-developer-link"
          href="https://app.zerossl.com/developer"
          target="_blank"
          rel="noopener noreferrer"
          >前往 ZeroSSL 获取 API Access Key（新标签页）</a
        >
        <label
          >配置方式<hw-select v-model="draft.mode" aria-label="ZeroSSL 配置方式" :disabled="saving">
            <hw-option value="access_key" label="API Access Key（自动获取 EAB）" />
            <hw-option value="eab" label="直接配置 EAB 凭据" /> </hw-select
        ></label>
        <template v-if="draft.mode === 'access_key'">
          <label
            >ZeroSSL API Access Key<hw-input
              v-model="draft.access_key"
              type="password"
              autocomplete="new-password"
              aria-label="ZeroSSL API Access Key"
              :disabled="saving"
          /></label>
          <p class="zerossl-note">
            对应系统配置 zerossl_access_key。保存新的 Access Key 会清除旧的 EAB
            缓存，下次申请时重新获取；不会改动已签发的证书。
          </p>
        </template>
        <template v-else>
          <label
            >EAB Key ID<hw-input
              v-model="draft.eab_kid"
              type="password"
              autocomplete="new-password"
              aria-label="EAB Key ID"
              :disabled="saving"
          /></label>
          <label
            >EAB HMAC Key<hw-input
              v-model="draft.eab_hmac_key"
              type="password"
              autocomplete="new-password"
              aria-label="EAB HMAC Key"
              :disabled="saving"
          /></label>
          <p class="zerossl-note">
            对应 zerossl_eab_kid 和 zerossl_eab_hmac_key，两项一起保存。完整的 EAB 凭据可直接用于认证。
          </p>
        </template>
        <p v-if="saveError" role="alert" class="zerossl-warning">{{ saveError }}</p>
        <div class="zerossl-actions zerossl-footer">
          <hw-button variant="outline" :disabled="saving" @click="editing = false">取消</hw-button>
          <hw-button theme="primary" :loading="saving" @click="save">保存 ZeroSSL 认证</hw-button>
        </div>
      </div>
    </template>
  </hw-dialog>
</template>

<style scoped>
.zerossl-account {
  margin-block: 16px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  line-height: 1.65;
}
.zerossl-account p {
  margin-block: 8px;
}
.zerossl-warning {
  color: var(--el-color-warning-dark-2);
}
.zerossl-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.zerossl-note {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  overflow-wrap: anywhere;
}
.zerossl-editor {
  line-height: 1.6;
}
.zerossl-developer-link {
  display: inline-block;
  padding-block: 8px;
  color: var(--el-color-primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}
.zerossl-developer-link:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
.zerossl-editor label {
  display: grid;
  gap: 8px;
  margin-block: 16px;
}
.zerossl-footer {
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
