<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { sslDNSCredentialStatusApi, sslDNSProvidersApi, type DNSProvider } from '@/apis/sslorder';
import { wafPrivateGroupListByBelongCloudApi } from '@/apis/private_group';

const props = defineProps({
  provider: { type: String, required: true },
  group: { type: String, required: true },
  revision: { type: Number, required: true },
});
const emit = defineEmits(['update:provider', 'update:group', 'configure', 'add-group']);
const providers = ref<DNSProvider[]>([]);
const version = ref('');
const providerNames: Record<string, string> = {
  hurricane: 'dns.he.net（Hurricane Electric DNS）',
  alidns: '阿里云 Alibaba Cloud DNS',
  huaweicloud: '华为云 Huawei Cloud DNS',
  tencentcloud: '腾讯云 Tencent Cloud DNS',
  baiducloud: '百度智能云 Baidu Cloud DNS',
  dnspod: 'DNSPod（旧版 ID / Token）',
  volcengine: '火山引擎 Volcengine DNS',
  westcn: '西部数码 West.cn',
};
const loading = ref(false);
const error = ref('');
const groups = ref<string[]>([]);
const configured = ref(new Set<string>());
const statusError = ref('');
const statusLoading = ref(false);
const advanced = ref(false);
const selected = computed(() =>
  providers.value.find((p) => p.code === props.provider || p.aliases.includes(props.provider)),
);
const fields = computed(() => selected.value?.fields.filter((f) => advanced.value || !f.advanced) || []);
const availableCount = computed(() => providers.value.filter((p) => !p.unavailable).length);
const tips: Record<string, string> = {
  cloudflare:
    '建议使用 API Token，授权目标域名的 Zone / DNS / Edit 和 Zone / Zone / Read，填写 CF_DNS_API_TOKEN 即可。其他 API Key 和别名字段无需重复填写。',
  alidns:
    '在阿里云为目标域名创建有 DNS 解析管理权限的 RAM AccessKey，填写 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY。临时凭据才需要 SECURITY_TOKEN。',
  tencentcloud:
    '使用腾讯云 CAM 中有目标域名 DNSPod 解析权限的 SecretId 和 SecretKey。不要把旧版 DNSPod 的 ID/Token 填在这里；旧版请选 dnspod。',
  huaweicloud: '填写华为云 Access Key ID、Secret Access Key 和域名所属区域。账户需有该 DNS 区域的记录管理权限。',
  baiducloud: '填写百度智能云 Access Key ID 和 Secret Access Key，并授予目标域名 DNS 记录管理权限。',
  duckdns: '填写 Duck DNS 账户的 Token，申请域名使用你在 Duck DNS 注册的完整子域名。',
  acmedns:
    '先按说明注册 acme-dns，并将 _acme-challenge 的 CNAME 指向分配的验证域名；可用于原 DNS 服务商没有 API 的情况。',
  route53:
    '可填写 AWS Access Key / Secret Key，也可使用服务器上的 AWS 配置文件或 IAM 角色。使用临时凭据时还需 Session Token。按官方说明授予目标 Hosted Zone 的权限。',
  exec: '高级方式：先在 HomeWaf 所在服务器安装 DNS 更新程序，再填写绝对路径。程序以 HomeWaf 的服务账户执行；家庭用户优先选择实际 DNS 服务商。',
};

async function loadProviders() {
  loading.value = true;
  error.value = '';
  try {
    const result = await sslDNSProvidersApi();
    if (result.code !== 0) throw new Error(result.msg || '读取失败');
    providers.value = result.data.providers;
    version.value = result.data.version;
  } catch {
    error.value = 'DNS 服务商目录加载失败，请重试后再申请。';
  } finally {
    loading.value = false;
  }
}

let groupRequest = 0;
async function loadGroups() {
  const request = ++groupRequest;
  const provider = props.provider;
  groups.value = ['default'];
  if (!provider) return;
  try {
    const names = new Set(['default']);
    for (let page = 1; ; page++) {
      const result = await wafPrivateGroupListByBelongCloudApi({
        private_group_belong_cloud: provider,
        pageIndex: page,
        pageSize: 100,
      });
      if (request !== groupRequest) return;
      if (result.code !== 0) throw new Error(result.msg);
      const list = result.data.list || [];
      list.forEach((item) => names.add(item.private_group_name));
      if (list.length < 100 || page * 100 >= result.data.total) break;
    }
    // Renewal must retain its stored group, including imported/older groups.
    if (props.group) names.add(props.group);
    groups.value = [...names];
  } catch {
    if (request === groupRequest) statusError.value = '密钥分组加载失败，请重试。';
  }
}

let statusRequest = 0;
async function loadStatus() {
  const request = ++statusRequest;
  configured.value = new Set();
  statusError.value = '';
  statusLoading.value = false;
  if (!props.provider || !props.group) return;
  statusLoading.value = true;
  try {
    const result = await sslDNSCredentialStatusApi(props.provider, props.group);
    if (request !== statusRequest) return;
    if (result.code !== 0) throw new Error(result.msg);
    configured.value = new Set((result.data || []).map((item) => item.private_key));
  } catch {
    if (request === statusRequest) statusError.value = '配置状态加载失败，请重试；当前状态不能用于判断是否已保存。';
  } finally {
    if (request === statusRequest) statusLoading.value = false;
  }
}

function changeProvider(value: string) {
  emit('update:provider', value);
  emit('update:group', 'default');
  advanced.value = false;
}
watch(() => [props.provider, props.revision], loadGroups, { immediate: true });
watch(() => [props.provider, props.group, props.revision], loadStatus, { immediate: true });
void loadProviders();
</script>

<template>
  <section class="dns-config" aria-label="DNS 验证配置" data-testid="dns-config">
    <p class="dns-home-note">
      适合家庭网络：无需公网 IP，无需开放或转发入站 80 / 443 端口，支持泛域名和自动续期。需要一个可管理 DNS
      解析的域名，以及访问证书机构、DNS API 和 DNS 查询的出站网络。
    </p>
    <p class="dns-home-note">证书可以用于 HTTPS 8443 等自定义端口；证书申请不会改变路由器的端口映射或外部访问方式。</p>
    <hw-form-item label="DNS 服务商" name="apply_dns">
      <hw-select
        :model-value="provider"
        @update:model-value="changeProvider"
        filterable
        :loading="loading"
        placeholder="搜索服务商名称或代码"
        aria-label="DNS 服务商"
        style="width: 100%"
      >
        <hw-option
          v-for="p in providers"
          :key="p.code"
          :value="p.code"
          :label="`${providerNames[p.code] || p.name} · ${p.code}${p.unavailable ? '（仅限终端交互）' : ''}`"
          :disabled="!!p.unavailable"
        />
      </hw-select>
    </hw-form-item>
    <p v-if="error" role="alert">
      {{ error }} <hw-button variant="text" @click="loadProviders">重试加载目录</hw-button>
    </p>
    <p v-else class="dns-meta">
      lego {{ version }} · {{ availableCount }} 个自动验证选项。请选择实际托管 DNS
      解析的服务商，可能与域名购买平台不同。
    </p>
    <template v-if="selected">
      <p v-if="selected.unavailable" role="alert">{{ selected.unavailable }}</p>
      <hw-form-item label="密钥分组" name="private_group_name">
        <div class="dns-group-controls">
          <hw-select :model-value="group" @update:model-value="emit('update:group', $event)" aria-label="密钥分组">
            <hw-option
              v-for="name in groups"
              :key="name"
              :value="name"
              :label="name === 'default' ? '默认分组' : name"
            />
          </hw-select>
          <hw-button variant="text" @click="emit('add-group', provider)">新建分组</hw-button>
        </div>
      </hw-form-item>
      <div v-if="provider === 'hurricane'" class="dns-guide" data-testid="hurricane-guide">
        <h3>dns.he.net：先启用 TXT 动态更新</h3>
        <ol>
          <li>登录 dns.he.net，进入域名管理面板。</li>
          <li>
            新建 TXT 记录。申请 example.com 或 *.example.com 时，主机名填 <code>_acme-challenge</code>，完整记录为
            <code>_acme-challenge.example.com</code>；内容先填 <code>init</code>。
          </li>
          <li>
            勾选 <strong>Enable entry for dynamic dns</strong>，保存；点击记录旁的刷新/密钥图标，生成并复制 DDNS
            Key（Token）。
          </li>
          <li>
            在下方 HURRICANE_TOKENS 的配置中填写 <code>example.com</code> 和 DDNS Key。域名不要带 <code>*.</code> 或
            <code>_acme-challenge.</code>。
          </li>
        </ol>
        <p>
          根域名和泛域名共用一项密钥。申请 nas.example.com 时，需另建
          <code>_acme-challenge.nas.example.com</code> 并添加对应密钥。保留这些 TXT
          记录及动态更新权限，后续续期会自动更新验证内容。
        </p>
      </div>
      <p v-else-if="tips[provider]" class="dns-guide">{{ tips[provider] }}</p>
      <div class="dns-doc-links">
        <a :href="selected.documentation" target="_blank" rel="noopener noreferrer"
          >查看 {{ selected.name }} 官方配置说明</a
        >
        <label><input v-model="advanced" type="checkbox" /> 显示可选高级参数</label>
      </div>
      <p class="dns-meta">
        只填写所选认证方式需要的字段；可选项及别名无需全部填写。凭据保存在当前分组，申请和续期共用。
      </p>
      <p v-if="statusLoading" role="status">正在读取配置状态…</p>
      <p v-if="statusError" role="alert">
        {{ statusError }}
        <hw-button
          variant="text"
          @click="
            loadGroups();
            loadStatus();
          "
          >重试读取配置</hw-button
        >
      </p>
      <div class="dns-fields">
        <div v-for="field in fields" :key="field.name" class="dns-field">
          <div class="dns-field-copy">
            <strong>{{ field.name }}</strong>
            <p>{{ field.description }}</p>
          </div>
          <div class="dns-field-actions">
            <span>{{
              statusLoading || statusError ? '状态未知' : configured.has(field.name) ? '已配置' : '未设置'
            }}</span>
            <hw-button
              variant="outline"
              :disabled="statusLoading || !!statusError || !group"
              :aria-label="`配置 ${field.name}`"
              @click="emit('configure', field.name, provider, group, configured.has(field.name) ? 'edit' : 'add')"
              >{{ configured.has(field.name) ? '修改' : '配置' }}</hw-button
            >
          </div>
        </div>
      </div>
      <details class="dns-reference">
        <summary>本版本配置示例与认证方式说明</summary>
        <pre
          >{{ selected.example }}

{{ selected.instructions }}</pre>
      </details>
    </template>
  </section>
</template>

<style scoped>
.dns-config {
  min-width: 0;
  margin-block: 16px;
}
.dns-home-note,
.dns-guide {
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-radius: var(--el-border-radius-base);
  line-height: 1.65;
}
.dns-home-note + .dns-home-note {
  padding-top: 0;
  margin-top: -12px;
}
.dns-meta {
  color: var(--el-text-color-secondary);
  line-height: 1.6;
  font-size: 13px;
}
.dns-guide h3 {
  font-size: 15px;
  margin: 0 0 8px;
}
.dns-guide ol {
  padding-left: 22px;
  margin: 0;
}
.dns-guide li + li {
  margin-top: 8px;
}
.dns-guide li {
  list-style: decimal;
}
.dns-guide code {
  overflow-wrap: anywhere;
}
.dns-group-controls {
  display: flex;
  gap: 8px;
  width: 100%;
}
.dns-group-controls .el-select {
  flex: 1;
  min-width: 0;
}
.dns-doc-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
  margin-block: 16px;
}
.dns-doc-links a {
  color: var(--el-color-primary);
}
.dns-doc-links label {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  cursor: pointer;
}
.dns-field {
  display: flex;
  gap: 16px;
  padding-block: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.dns-field-copy {
  min-width: 0;
  flex: 1;
  overflow-wrap: anywhere;
}
.dns-field-copy strong {
  font-size: 13px;
}
.dns-field-copy p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}
.dns-field-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-shrink: 0;
  font-size: 13px;
}
.dns-reference {
  margin-top: 16px;
}
.dns-reference summary {
  cursor: pointer;
  padding-block: 8px;
}
.dns-reference pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 360px;
  overflow: auto;
  font-size: 13px;
  line-height: 1.6;
}
@media (max-width: 600px) {
  .dns-field {
    flex-direction: column;
    gap: 8px;
  }
  .dns-field-actions {
    justify-content: space-between;
  }
}
</style>
