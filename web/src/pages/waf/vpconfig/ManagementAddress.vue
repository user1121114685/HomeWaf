<template>
  <section class="management-address" data-testid="management-address">
    <h3>管理地址与自动证书</h3>
    <p>默认用服务器 IP 访问，自动生成对应的 HTTPS 证书，无需配置域名。</p>
    <hw-loading :loading="loading">
      <hw-alert v-if="loadError" theme="error" :message="loadError" />
      <hw-button v-if="loadError" variant="outline" @click="load">重试读取配置</hw-button>
      <hw-form v-if="loaded && !applied" :data="draft" layout="vertical" @submit="apply" :disabled="saving">
        <hw-form-item label="专用管理域名（可选）">
          <hw-input v-model="draft.domain" placeholder="留空使用 IP，例如 waf.lan" autocomplete="off" />
          <template #help
            >填写后仅接受这个域名的 HTTPS 管理请求，其他域名和 IP 将被拒绝。请先在路由器 DNS 或访问设备的 hosts
            中将域名指向服务器 IP。</template
          >
        </hw-form-item>
        <hw-checkbox v-if="draft.domain.trim()" v-model="confirmed"
          >我已配置该域名解析，并确认切换后不再通过 IP 管理</hw-checkbox
        >
        <hw-alert v-if="error" theme="error" :message="error" />
        <div class="management-address-actions">
          <hw-button
            theme="primary"
            type="submit"
            :loading="saving"
            :disabled="saving || (!!draft.domain.trim() && !confirmed)"
            >生成证书并应用管理地址</hw-button
          >
        </div>
      </hw-form>
      <hw-alert
        v-if="applied"
        theme="success"
        message="配置已保存，管理端正在重启。请先下载并信任根证书，再打开新入口验证；跨域访问需要重新登录。"
      />
      <div v-if="state?.https_enabled && state?.urls?.length" class="management-address-current">
        <span>{{ applied ? '新 HTTPS 入口' : '已配置的 HTTPS 入口' }}</span>
        <code v-for="url in state.urls" :key="url">{{ url }}</code>
      </div>
      <ManagementTrustGuide
        v-if="state?.is_local && state?.ca_pem"
        :ca-pem="state.ca_pem"
        :fingerprint="state.ca?.fingerprint"
        :https-url="state.urls?.[0]"
      />
      <p v-if="state?.is_local && state?.ca_pem" class="management-address-help">
        更换 IP 或域名会重新签发服务器证书，但沿用此实例的根证书；已经信任相同指纹的设备通常无需重复导入。
      </p>
    </hw-loading>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, watch } from 'vue';
import { ElMessageBox } from 'element-plus';
import request, { preserveManagementTransition } from '@/utils/request';
import ManagementTrustGuide from '@/components/ManagementTrustGuide.vue';
const emit = defineEmits<{ changed: [state: any] }>();
const draft = reactive({ domain: '' });
const state = ref<any>(null);
const loading = ref(false);
const loaded = ref(false);
const saving = ref(false);
const confirmed = ref(false);
const applied = ref(false);
const error = ref('');
const loadError = ref('');
watch(
  () => draft.domain,
  () => {
    confirmed.value = false;
  },
);
async function load() {
  loading.value = true;
  loadError.value = '';
  try {
    const result = await request({ url: 'vipconfig/managementAddress', method: 'get' });
    if (result.code !== 0) throw new Error(result.msg || '无法读取管理地址');
    state.value = result.data;
    draft.domain = result.data.domain || '';
    loaded.value = true;
    emit('changed', result.data);
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '无法读取管理地址，请重试';
  } finally {
    loading.value = false;
  }
}
async function apply() {
  if (saving.value) return;
  const domain = draft.domain.trim().toLowerCase();
  error.value = '';
  if (domain && !confirmed.value) {
    error.value = '请先确认域名解析及管理入口切换';
    return;
  }
  try {
    await ElMessageBox.confirm(
      domain
        ? `将重新签发 ${domain} 的证书，重启管理端，并拒绝其他域名与 IP 的管理请求。请确认域名已指向此服务器。`
        : '将为服务器 IP 重新生成证书，恢复 IP 访问，并同时支持 HTTP 引导入口和 HTTPS 管理入口。',
      '应用管理地址',
      { confirmButtonText: '生成并应用', cancelButtonText: '取消', type: 'warning' },
    );
  } catch {
    return;
  }
  saving.value = true;
  try {
    const result = await request({
      url: 'vipconfig/managementAddress',
      method: 'post',
      data: { domain, confirm_domain: domain },
      timeout: 30000,
    });
    if (result.code !== 0) throw new Error(result.msg || '配置保存失败');
    state.value = result.data;
    applied.value = true;
    preserveManagementTransition();
    emit('changed', result.data);
  } catch (e) {
    error.value = e instanceof Error ? e.message : '配置未保存，请检查连接后重试';
  } finally {
    saving.value = false;
  }
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
