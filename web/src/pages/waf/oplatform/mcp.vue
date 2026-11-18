<template>
  <div class="mcp-page" data-testid="mcp-page">
    <PageActions>
      <ElButton type="primary" :loading="creating || copying" :disabled="!status || loading || testing" @click="token.trim() ? copyConfig() : generateKey()">{{ token.trim() ? '复制连接配置' : '自动生成密钥' }}</ElButton>
      <ElButton :disabled="!status || loading || creating || copying" @click="openCreate">自定义密钥</ElButton>
      <RouterLink class="el-button" to="/oplatform/key">管理密钥</RouterLink>
      <RouterLink class="el-button" to="/oplatform/log">调用记录</RouterLink>
      <ElButton :loading="loading" @click="load">刷新状态</ElButton>
    </PageActions>
    <ElAlert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <p v-if="loading && !status" role="status">正在读取 MCP 状态和可用工具…</p>
    <template v-if="status">
      <section class="mcp-panel mcp-status" aria-label="连接状态">
        <div>
          <span class="mcp-label">HTTP 接入</span>
          <h2>让 MCP 客户端管理 HomeWaf</h2>
          <p>排查服务、分析日志、添加和修改防护配置。使用外部客户端连接，无需在 HomeWaf 配置模型。</p>
        </div>
        <div class="mcp-availability">
          <ElTag :type="status.enabled ? 'success' : 'warning'">{{ status.enabled ? '允许远程接入' : '远程接入已关闭' }}</ElTag>
          <span>Streamable HTTP · Bearer 认证</span>
          <ElButton :loading="enabling" :disabled="loading" @click="toggleAccess">{{ status.enabled ? '关闭远程接入' : '启用远程接入' }}</ElButton>
        </div>
      </section>
      <p class="mcp-note">远程接入与开放 API 共用总开关；修改会立即影响两者。MCP 密钥始终只允许调用本页列出的工具。</p>

      <section class="mcp-panel" aria-labelledby="mcp-connect-title">
        <h2 id="mcp-connect-title">连接客户端</h2>
        <div class="mcp-connect-grid">
          <div class="mcp-fields">
            <label for="mcp-url">MCP 地址</label>
            <ElInput id="mcp-url" v-model="endpoint" :disabled="testing || creating || copying" />
            <p class="mcp-hint">从当前管理入口生成。远程连接请使用客户端能访问的 HTTPS 地址，反向代理需转发 /mcp。</p>
            <label for="mcp-token">MCP 专用密钥</label>
            <ElInput id="mcp-token" v-model="token" type="password" show-password autocomplete="off" :disabled="testing || creating || copying" placeholder="自动生成并填入，也可粘贴已有 MCP 密钥" />
            <p class="mcp-hint">默认自动生成“排障与配置”密钥，名称也会自动填写。只读权限、有效期和 IP 限制可在“自定义密钥”中设置。</p>
            <p class="mcp-hint">服务器只保存校验摘要，明文只在当前页面展示。离开页面不会让已配置的客户端失效；普通 API Key 不能用于 MCP。</p>
            <ElAlert v-if="created" title="密钥已自动生成并填入。点击“复制配置”即可使用；明文仅显示这一次，已有连接不受影响。" type="success" :closable="false" />
            <div class="mcp-actions">
              <ElButton :disabled="!status.enabled || !token.trim() || creating || copying" :loading="testing" @click="testConnection">验证连接</ElButton>
              <ElButton :disabled="!token || testing || creating || copying" @click="clearToken">清除本页密钥</ElButton>
            </div>
            <ElAlert v-if="testError" :title="testError" type="error" :closable="false" />
            <p v-if="testResult" role="status" class="mcp-test-result">{{ testResult }}</p>
          </div>
          <div class="mcp-config">
            <div class="mcp-config-heading"><h3>客户端配置</h3><ElButton :loading="copying" :disabled="loading || creating || testing" @click="copyConfig">{{ token.trim() ? '复制配置' : '生成并复制配置' }}</ElButton></div>
            <pre tabindex="0" aria-label="MCP 客户端配置示例">{{ displayedConfig }}</pre>
            <p class="mcp-hint">没有密钥时会先自动生成，再把完整密钥填入复制的配置。已有密钥会直接复用；屏幕示例始终隐藏密钥。</p>
          </div>
        </div>
      </section>

      <section class="mcp-panel" aria-labelledby="mcp-tools-title">
        <div class="mcp-tool-heading"><h2 id="mcp-tools-title">可调用的功能</h2><span>{{ status.tools.length }} 个工具 · 权限由密钥决定</span></div>
        <p>只读密钥可查询与预检；可配置密钥还可保存服务、维护 IP 组和证书策略。删除、关闭访问等操作需要明确确认，并记录调用来源。</p>
        <ElAlert title="禁止操作：控制台管理员、密码与双因素认证、管理密钥、实例安全设置、任意文件/SQL/命令、插件进程，以及证书私钥导出。日志中已知凭据字段会脱敏。" type="info" :closable="false" />
        <div class="mcp-filter"><label for="mcp-filter">查找功能</label><ElInput id="mcp-filter" v-model="query" clearable placeholder="工具名称、功能或说明" /></div>
        <ElTable :data="filteredTools" row-key="name" class="mcp-tools-table" :empty-text="status.tools.length ? '没有匹配的功能，请调整搜索。' : '服务端未提供工具，请刷新或检查实例。'">
          <ElTableColumn label="工具 / 范围" min-width="200"><template #default="{ row }"><strong>{{ row.group }}</strong><code>{{ row.name }}</code></template></ElTableColumn>
          <ElTableColumn label="权限" width="110"><template #default="{ row }"><ElTag :type="row.read_only ? 'info' : 'warning'">{{ row.read_only ? '查询 / 预检' : '配置变更' }}</ElTag></template></ElTableColumn>
          <ElTableColumn prop="description" label="作用与生效方式" min-width="320" />
        </ElTable>
      </section>
    </template>

    <KeyEditor v-if="createVisible" mcp-only :suggested-source="suggestedSource" :default-name="connectionName()"
      @close="createVisible = false" @saved="mcpKeySaved" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { RouterLink } from 'vue-router';
import { ElAlert, ElButton, ElInput, ElMessage, ElMessageBox, ElRadio, ElRadioGroup, ElTable, ElTableColumn, ElTag } from 'element-plus';
import { getMCPStatus, mcpConnectionConfig, testMCPConnection, type MCPStatus } from '@/apis/mcp';
import { oplatform_key_add_api } from '@/apis/oplatform';
import KeyEditor from './KeyEditor.vue';
import { edit_system_config_by_item_api } from '@/apis/systemconfig';
import { securityPath } from '@/config/host';

const status = ref<MCPStatus>();
const loading = ref(false), error = ref(''), enabling = ref(false);
const endpoint = ref(new URL(`${securityPath}/mcp`, window.location.origin).href), token = ref(''), created = ref(false);
const query = ref(''), testing = ref(false), testError = ref(''), testResult = ref('');
const createVisible = ref(false), creating = ref(false), copying = ref(false);
const suggestedSource = ref('');
let controller: AbortController | undefined;
let active = true;
const displayedConfig = computed(() => mcpConnectionConfig(endpoint.value, ''));
const filteredTools = computed(() => (status.value?.tools || []).filter((tool) => `${tool.name} ${tool.group} ${tool.description}`.toLowerCase().includes(query.value.trim().toLowerCase())));
watch([endpoint, token], () => { testError.value = ''; testResult.value = ''; });
async function load() {
  if (loading.value) return;
  loading.value = true; error.value = '';
  try {
    const response = await getMCPStatus();
    if (response.code !== 0 || !Array.isArray(response.data?.tools)) throw new Error(response.msg || 'MCP 状态读取失败。');
    if (active) {
      status.value = response.data;
      suggestedSource.value = response.data.suggested_source || '';
    }
  } catch (cause: any) { if (active) error.value = cause.message || '状态读取失败，请重试。'; }
  finally { loading.value = false; }
}
async function toggleAccess() {
  if (!status.value || enabling.value) return;
  const enabled = !status.value.enabled;
  try { await ElMessageBox.confirm(enabled ? '启用后，持有有效凭据的远程客户端可以调用开放 API 和 MCP。仍受来源 IP、管理域名和密钥权限限制。' : '关闭后，所有远程开放 API 和 MCP 连接立即失效。本机 CLI 密钥不受影响。', enabled ? '启用远程接入' : '关闭远程接入', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' }); } catch { return; }
  enabling.value = true; error.value = '';
  try {
    const response = await edit_system_config_by_item_api({ item: 'open_platform_enabled', value: enabled ? '1' : '0' });
    if (response.code !== 0) throw new Error(response.msg || '设置保存失败。');
    await load();
    testResult.value = '';
  } catch (cause: any) { error.value = cause.message || '设置保存失败，请重试。'; }
  finally { enabling.value = false; }
}
function openCreate() {
  createVisible.value = true;
}
function connectionName() {
  return `MCP 运维连接 ${new Date().toISOString().slice(0, 19).replace('T', ' ')} UTC`;
}
async function generateKey() {
  if (!status.value || loading.value || creating.value || !active) return false;
  if (token.value.trim()) return true;
  if (!suggestedSource.value) {
    error.value = '无法识别本次请求来源 IP。请从密钥管理中手动输入 MCP 来源网段。';
    return false;
  }
  return issueKey({ key_name: connectionName(), mcp_access: 'write', expire_time: '', ip_whitelist: suggestedSource.value, rate_limit: 120, remark: 'MCP HTTP connection' });
}
async function issueKey(values: { key_name: string; mcp_access: string; expire_time: string; ip_whitelist: string; rate_limit: number; remark: string }) {
  if (creating.value) return false;
  creating.value = true; error.value = '';
  try {
    const response = await oplatform_key_add_api(values);
    if (response.code !== 0 || !response.data?.api_key) throw new Error(response.msg || '创建失败。');
    if (!active) return false;
    token.value = response.data.api_key; created.value = true; createVisible.value = false;
    return true;
  } catch (cause: any) {
    if (active) {
      error.value = cause.message || '自动生成失败，请重试。';
    }
    return false;
  }
  finally { creating.value = false; }
}
function mcpKeySaved(secret?: string) {
  createVisible.value = false;
  if (!secret) return;
  token.value = secret;
  created.value = true;
}
function clearToken() { token.value = ''; created.value = false; }
async function copyConfig() {
  if (copying.value || creating.value) return;
  copying.value = true;
  try {
    if (!token.value.trim() && !(await generateKey())) return;
    if (!active) return;
    await navigator.clipboard.writeText(mcpConnectionConfig(endpoint.value, token.value.trim()));
    ElMessage.success('已复制完整连接配置，密钥已自动填入。');
  } catch { ElMessage.error('复制失败，密钥已保留。请再次点击“复制配置”，或分别复制地址和密钥。'); }
  finally { copying.value = false; }
}
async function testConnection() {
  if (testing.value) return;
  testing.value = true; testError.value = ''; testResult.value = '';
  controller = new AbortController();
  const timeout = window.setTimeout(() => controller?.abort(), 15000);
  try {
    const result = await testMCPConnection(endpoint.value, token.value, controller.signal);
    if (active) testResult.value = `连接通过：${result.name}，当前密钥可调用 ${result.tools} 个工具。仅完成协议与权限检查，未执行配置变更。`;
  } catch (cause: any) { if (active) testError.value = cause.name === 'AbortError' ? '连接超时，请检查网络和地址后重试。' : cause.message || '连接失败，请重试。'; }
  finally { window.clearTimeout(timeout); testing.value = false; }
}
onMounted(load);
onBeforeUnmount(() => { active = false; controller?.abort(); clearToken(); });
</script>

<style scoped>
.mcp-page { display: grid; gap: 20px; min-width: 0; color: var(--hw-ink); }
.mcp-panel { padding: 24px; border: 1px solid var(--hw-line); border-radius: var(--hw-radius, 8px); background: var(--hw-panel); min-width: 0; }
.mcp-status, .mcp-tool-heading, .mcp-config-heading { display: flex; justify-content: space-between; gap: 24px; align-items: flex-start; }
.mcp-panel h2 { font-size: 18px; margin: 0 0 12px; font-weight: 600; }
.mcp-panel h3 { margin: 0; font-size: 14px; }
.mcp-panel p { margin: 8px 0 16px; line-height: 1.7; }
.mcp-label { display: block; color: var(--hw-muted); font-size: 13px; margin-bottom: 10px; }
.mcp-availability { display: grid; justify-items: start; gap: 12px; flex-shrink: 0; font-size: 13px; }
.mcp-note, .mcp-hint, .mcp-tool-heading > span { color: var(--hw-muted); font-size: 13px; line-height: 1.7; }
.mcp-note { margin: -8px 0 0; }
.mcp-connect-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 32px; }
.mcp-fields, .mcp-config { min-width: 0; }
.mcp-fields > label { display: block; font-weight: 500; font-size: 14px; margin: 10px 0 8px; }
.mcp-config-heading { align-items: center; margin: 10px 0 12px; }
.mcp-config pre { white-space: pre-wrap; overflow-wrap: anywhere; padding: 18px; background: var(--hw-subtle); border: 1px solid var(--hw-line); border-radius: 6px; font-family: var(--font-mono); font-size: 12px; line-height: 1.7; }
.mcp-actions { display: flex; flex-wrap: wrap; gap: 12px; margin: 16px 0; }
.mcp-actions :deep(.el-button + .el-button) { margin-left: 0; }
.mcp-filter { display: grid; grid-template-columns: auto minmax(0, 360px); align-items: center; gap: 12px; margin: 24px 0 16px; }
.mcp-tools-table code { display: block; font-family: var(--font-mono); font-size: 12px; overflow-wrap: anywhere; color: var(--hw-muted); }
.mcp-tools-table strong { font-weight: 500; }
.mcp-test-result { color: var(--hw-success); }
@media (max-width: 900px) { .mcp-connect-grid { grid-template-columns: 1fr; gap: 12px; } .mcp-status { flex-wrap: wrap; } }
@media (max-width: 600px) { .mcp-panel { padding: 16px; } .mcp-tool-heading { flex-direction: column; gap: 0; } .mcp-filter { grid-template-columns: 1fr; } .mcp-page :deep(.el-button) { min-height: 40px; } }
</style>
