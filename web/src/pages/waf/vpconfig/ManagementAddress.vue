<template>
  <section class="management-address" data-testid="management-address">
    <h3>管理地址与自动证书</h3>
    <p>先验证新 HTTPS 入口。用同一管理员账号登录并确认后，新的访问限制才会生效。</p>
    <hw-loading :loading="loading">
      <hw-alert v-if="loadError" theme="error" :message="loadError" />
      <hw-button v-if="loadError" variant="outline" @click="load">重试读取配置</hw-button>
      <hw-form v-if="loaded && !state?.pending" :data="draft" layout="vertical" @submit="prepare" :disabled="saving">
        <hw-form-item :label="mode === 'address' ? '专用管理域名（可选）' : '验证 HTTPS 的域名或 IP'">
          <hw-input v-model="draft.domain" placeholder="例如 waf.lan" autocomplete="off" />
          <template #help>{{ mode === 'address' ? '留空可恢复 IP 访问。填写域名时，请先将它解析到服务器 IP。' : '填写当前管理端证书覆盖的地址。' }}</template>
        </hw-form-item>
        <div class="management-address-actions">
          <hw-button theme="primary" type="submit" :loading="saving" :disabled="saving">准备验证新入口</hw-button>
          <hw-button v-if="mode === 'https_only'" variant="outline" @click="resetMode">取消</hw-button>
        </div>
      </hw-form>
      <div v-if="state?.pending" class="management-address-current">
        <strong>{{ state.pending.can_confirm ? '新入口登录已验证' : '等待在新入口登录' }}</strong>
        <p>{{ state.pending.can_confirm ? '确认后将启用新的管理访问设置。' : '原入口仍可用。请在新标签页登录，或在此取消。' }}
          验证截止时间为 {{ new Date(state.pending.expires).toLocaleTimeString() }}。</p>
        <a v-for="url in state.pending.urls" :key="url" :href="loginUrl(url)" target="_blank" rel="noopener noreferrer">{{ url }} · 登录新入口</a>
        <div class="management-address-actions">
          <hw-button v-if="state.pending.can_confirm" theme="primary" :loading="saving" @click="finish">确认切换管理入口</hw-button>
          <hw-button variant="outline" :disabled="saving" @click="change('cancel')">取消本次切换</hw-button>
          <hw-button variant="outline" :disabled="saving" @click="load">刷新状态</hw-button>
        </div>
      </div>
      <hw-alert v-if="notice" theme="success" :message="notice" />
      <hw-alert v-if="error" theme="error" :message="error" />
      <div v-if="state?.https_enabled && state?.urls?.length" class="management-address-current">
        <span>已配置的 HTTPS 入口</span>
        <code v-for="url in state.urls" :key="url">{{ url }}</code>
      </div>
      <ManagementTrustGuide
        v-if="state?.ca_pem && (state.is_local || state.pending?.mode === 'address')"
        :ca-pem="state.ca_pem"
        :fingerprint="state.ca?.fingerprint"
        :https-url="state.pending ? loginUrl(state.pending.urls[0]) : state.urls?.[0]"
      />
      <p v-if="state?.is_local && state?.ca_pem" class="management-address-help">
        更换 IP 或域名会重新签发服务器证书，但沿用此实例的根证书；已经信任相同指纹的设备通常无需重复导入。
      </p>
    </hw-loading>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, onBeforeUnmount } from 'vue';
import { ElMessageBox } from 'element-plus';
import request, { preserveManagementTransition } from '@/utils/request';
import ManagementTrustGuide from '@/components/ManagementTrustGuide.vue';
const emit = defineEmits<{ changed: [state: any] }>();
const draft = reactive({ domain: '' });
const state = ref<any>(null);
const loading = ref(false);
const loaded = ref(false);
const saving = ref(false);
const mode = ref<'address' | 'https_only'>('address');
const notice = ref('');
const error = ref('');
const loadError = ref('');
let expiryTimer: ReturnType<typeof setTimeout>;
const settingsPath = '/sys/VpConfig?section=access';
const loginUrl = (url: string) => url + '#/login?' + new URLSearchParams({redirect: settingsPath, management_preview: state.value.pending.id});
function resetMode() { mode.value = 'address'; draft.domain = state.value?.domain || ''; }
function prepareHttpsOnly() {
  mode.value = 'https_only';
  draft.domain = window.location.hostname.replace(/^\[|\]$/g, '');
  notice.value = '';
  document.querySelector('.management-address')?.scrollIntoView({block: 'center'});
}
defineExpose({ prepareHttpsOnly });
function received(data: any) {
  state.value = data;
  clearTimeout(expiryTimer);
  if (data.pending) expiryTimer = setTimeout(load, Math.max(0, Date.parse(data.pending.expires) - Date.now()) + 250);
  emit('changed', data);
}
onBeforeUnmount(() => clearTimeout(expiryTimer));
async function load() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await request({ url: 'vipconfig/managementAddress', method: 'get' });
    if (result.code !== 0) throw new Error(result.msg || '无法读取管理地址');
    received(result.data);
    if (!loaded.value) resetMode();
    loaded.value = true;
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '无法读取管理地址，请重试';
  } finally {
    loading.value = false;
  }
}
async function change(action: 'prepare' | 'confirm' | 'cancel') {
  if (saving.value) return;
  error.value = '';
  notice.value = '';
  saving.value = true;
  try {
    const result = await request({
      url: 'vipconfig/managementAddress',
      method: 'post',
      data: { action, mode: mode.value, domain: draft.domain.trim().toLowerCase(), id: state.value?.pending?.id },
      timeout: 30000,
    });
    if (result.code !== 0) throw new Error(result.msg || '配置保存失败');
    received(result.data);
    if (action === 'confirm') {
      notice.value = '已切换管理入口。请继续使用当前地址。';
      resetMode();
    } else if (action === 'prepare') preserveManagementTransition();
    else notice.value = '已取消切换，原管理入口继续可用。';
  } catch (e) {
    error.value = e instanceof Error ? e.message : '配置未保存，请检查连接后重试';
  } finally {
    saving.value = false;
  }
}
function prepare() { return change('prepare'); }
async function finish() {
  const pending = state.value?.pending;
  if (!pending?.can_confirm) return;
  const consequence = pending.mode === 'https_only' ? '此后管理端仅接受 HTTPS。' :
    pending.domain
      ? '此后只接受新域名的 HTTPS 管理请求，原 IP 入口将关闭。' : '此后恢复 IP 管理入口和 HTTP 引导访问。';
  try { await ElMessageBox.confirm(consequence, '切换管理入口', { confirmButtonText: '确认切换', cancelButtonText: '返回', type: 'warning' }); }
  catch { return; }
  return change('confirm');
}
onMounted(load);
</script>

<style scoped>
.management-address {
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--hw-line);
  min-width: 0;
}
.management-address h3 {
  margin: 0 0 8px;
  font-size: 18px;
  font-weight: 600;
}
.management-address p {
  color: var(--hw-muted);
  line-height: 1.7;
}
.management-address-actions {
  margin: 16px 0;
}
.management-address-current {
  display: grid;
  gap: 6px;
  margin-top: 16px;
}
.management-address-current code {
  overflow-wrap: anywhere;
}
.management-address-help {
  font-size: 13px;
}
.management-address :deep(.el-checkbox) {
  white-space: normal;
  height: auto;
  min-height: 32px;
}
.management-address :deep(.el-checkbox__label) {
  white-space: normal;
}
@media (max-width: 600px) {
  .management-address-actions :deep(.el-button) {
    width: 100%;
    white-space: normal;
    height: auto;
    min-height: 44px;
  }
}
</style>
