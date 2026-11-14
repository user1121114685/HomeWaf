<template>
  <ElCard v-if="sources.length" shadow="never" class="legacy-migration" data-testid="legacy-ports">
    <template #header><strong>现有业务端口 · 迁入五步配置</strong></template>
    <p>先核对同端口的策略与业务路由。迁入仅保存副本；在端口总览确认启用时接管原监听。</p>
    <ElButton @click="router.push({name:'LegacyWafHosts'})">查看原站点源记录</ElButton>
    <ElTable :data="sources" size="small">
      <ElTableColumn prop="port" label="监听端口" width="105" />
      <ElTableColumn label="原站点"><template #default="{ row }">{{ row.sources.map(s => s.name + (s.enabled ? '' : '（停用）')).join('、') }}</template></ElTableColumn>
      <ElTableColumn label="归属" width="170"><template #default="{ row }">{{ row.state === 'active' ? (row.imported ? '五步流水线已接管' : '已接管，当前无配置') : row.imported ? '已迁入，等待接管' : '原站点监听' }}</template></ElTableColumn>
      <ElTableColumn label="操作" width="150"><template #default="{ row }"><ElButton link type="primary" :disabled="busy || row.imported" @click="open(row)">{{ row.imported ? '在上方配置' : '预览迁入' }}</ElButton></template></ElTableColumn>
    </ElTable>
    <ElDialog v-model="visible" title="现有端口迁入预览" width="min(1100px,96vw)" :close-on-click-modal="false" :close-on-press-escape="!busy" :show-close="!busy" @closed="clear">
      <ElAlert v-if="error" :title="error" type="error" :closable="false" />
      <template v-if="selected">
        <p>端口 {{ selected.port }}：所有主机名共用下面选定来源的防护策略与账号。</p>
        <label class="source-label">统一策略来源<ElSelect v-model="policyHost" aria-label="迁入策略来源" :disabled="busy" @change="preview"><ElOption v-for="source in selected.sources" :key="source.code" :value="source.code" :label="source.name" /></ElSelect></label>
      </template>
      <template v-if="candidate">
        <ElAlert v-for="(item, i) in candidate.blockers" :key="'b' + i" :title="item" type="error" :closable="false" class="space" />
        <ElAlert v-for="(item, i) in candidate.warnings" :key="'w' + i" :title="item" type="warning" :closable="false" class="space" />
        <h3>将复制的业务路由</h3>
        <ElTable :data="candidate.config.routes || []" size="small"><ElTableColumn label="主机名"><template #default="{ row }">{{ row.hosts.join('、') }}</template></ElTableColumn><ElTableColumn prop="path_prefix" label="路径匹配" /><ElTableColumn label="业务目标"><template #default="{ row }">{{ row.target_type === 'static' ? row.static?.settings?.static_site_path : row.target_type === 'redirect' ? row.redirect_url : row.upstream }}</template></ElTableColumn><ElTableColumn prop="dial_ip" label="固定连接IP" /></ElTable>
        <p>监听：{{ candidate.config.protocol }} / {{ candidate.config.policy.network.listen_ip_version || 'both' }}；登录门：{{ candidate.config.policy.gate.enabled ? candidate.config.policy.gate.mode : '关闭' }}；复制账号：{{ candidate.account_names.join('、') || '无' }}。</p>
        <div v-if="candidate.config.policy.gate.enabled && !candidate.account_names.length" class="accounts"><label>初始账号<ElInput v-model="username" aria-label="迁入初始账号" autocomplete="off" /></label><label>初始口令<ElInput v-model="password" aria-label="迁入初始口令" type="password" show-password autocomplete="new-password" maxlength="72" /></label></div>
        <label v-if="candidate.requires_totp && !candidate.account_names.length">新的动态口令Base32密钥<ElInput v-model="totpSecret" type="password" show-password autocomplete="new-password" aria-label="迁入动态口令密钥" /><small>原认证已要求二次验证；不能在替换账号时丢弃此保护。请把密钥录入身份验证器。</small></label>
        <ElButton v-if="candidate.requires_totp && !candidate.account_names.length" @click="totpSecret = newTOTPSecret()">生成新的动态口令密钥</ElButton>
        <details><summary>完整五步配置（不含凭据）</summary><pre>{{ JSON.stringify(candidate.config, null, 2) }}</pre></details>
        <div class="actions"><ElButton :disabled="busy || !!candidate.blockers.length" @click="explain">解释陌生来源请求</ElButton><ElButton :disabled="busy || !!candidate.blockers.length" @click="checkUpstreams">检查上游TCP连通性</ElButton></div>
        <Trace :result="trace" />
        <p v-for="item in connectivity" :key="item">{{ item }}</p>
        <ElCheckbox v-model="accepted" :disabled="busy">我已核对策略差异、账号和路由，确认复制为未监听配置</ElCheckbox>
        <p>迁入后可在同一端口配置页调整。启用后停止或删除新配置也不会自动复活原监听。</p>
      </template>
      <template #footer><ElButton :disabled="busy" @click="visible = false">关闭</ElButton><ElButton type="primary" :loading="busy" :disabled="!canImport" @click="commit">确认迁入为未监听</ElButton></template>
    </ElDialog>
  </ElCard>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import type { PropType } from 'vue';
import { useRouter } from 'vue-router';
import { ElAlert, ElButton, ElCard, ElCheckbox, ElDialog, ElInput, ElMessage, ElOption, ElSelect, ElTable, ElTableColumn } from 'element-plus';
import { pipelinePost } from '@/apis/pipeline';
import { mapBounded, type LegacyPortPreview, type LegacyPortSummary, type PortConfig } from './model';
import Trace from './Trace.vue';
import { newTOTPSecret } from './model';
defineProps({ sources: { type: Array as PropType<LegacyPortSummary[]>, required: true } });
const emit = defineEmits(['imported']);
const router=useRouter();
const visible = ref(false), busy = ref(false), error = ref(''), policyHost = ref(''), accepted = ref(false);
const selected = ref<LegacyPortSummary | null>(null), candidate = ref<LegacyPortPreview | null>(null), trace = ref<any>(null);
const username = ref(''), password = ref(''), connectivity = ref<string[]>([]), checked = ref(false);
const totpSecret=ref('');
const canImport = computed(() => candidate.value && !candidate.value.blockers.length && !candidate.value.imported && trace.value && checked.value && accepted.value && (!candidate.value.config.policy.gate.enabled || candidate.value.account_names.length || (username.value.trim() && password.value.length >= 10 && (!candidate.value.requires_totp || totpSecret.value.trim()))));
async function run(action: () => Promise<void>) { if (busy.value) return; busy.value = true; error.value = ''; try { await action(); } catch (e) { error.value = e.message || String(e); } finally { busy.value = false; } }
function clear() { candidate.value = null; trace.value = null; password.value = ''; username.value = ''; totpSecret.value=''; accepted.value = false; connectivity.value = []; checked.value = false; }
async function open(source: LegacyPortSummary) { clear(); selected.value = source; policyHost.value = source.sources[0]?.code || ''; visible.value = true; await preview(); }
async function preview() { await run(async () => { clear(); candidate.value = await pipelinePost<LegacyPortPreview>('migration/preview', { port: selected.value.port, policy_host: policyHost.value }); }); }
async function explain() { await run(async () => { trace.value = await pipelinePost('explain', { config: candidate.value.config, client_ip: '198.51.100.25', method: 'GET', path: '/', headers: { Accept: 'text/html' }, body: '' }); }); }
async function checkUpstreams() { await run(async () => {
  const config = candidate.value.config;
  const endpoints = [{ upstream: config.upstream, dial_ip: config.upstream_dial_ip }, ...(config.routes || []).filter(route => !route.target_type || route.target_type === 'proxy').flatMap<{upstream:string;dial_ip?:string}>(route => route.backends?.length ? route.backends.filter(backend => backend.weight > 0) : [route])].filter(value => value.upstream);
  const unique = [...new Map(endpoints.map(value => [value.upstream + '\n' + (value.dial_ip || ''), value])).values()];
  connectivity.value = await mapBounded(unique, async value => { try { const result = await pipelinePost('upstream/test', { upstream: value.upstream, dial_ip: value.dial_ip || '' }); return value.upstream + '：' + result + '（TCP，不验证TLS与业务认证）'; } catch (e) { return value.upstream + '：连接失败；启用前请修正上游配置'; } }, 4);
  checked.value = true;
  if (!unique.length) connectivity.value = ['该端口仅提供本地响应（静态、重定向或停用状态），无需连接上游。'];
}); }
async function commit() { await run(async () => { if (!canImport.value) throw new Error('请完成预览、连通性检查及差异确认'); const config = await pipelinePost<PortConfig>('migration/import', { port: selected.value.port, policy_host: policyHost.value, preview_token: candidate.value.preview_token, accept_changes: true, initial_username: username.value, initial_password: password.value,initial_totp_secret:totpSecret.value }); password.value = '';totpSecret.value=''; visible.value = false; ElMessage.success('已迁入为未监听；在端口总览启用后接管原监听'); emit('imported', config); }); }
</script>

<style scoped>
.legacy-migration{margin-top:20px}.space{margin:12px 0}.accounts{display:grid;grid-template-columns:1fr 1fr;gap:16px}.accounts label,.source-label{display:flex;flex-direction:column;gap:8px}.actions{display:flex;gap:12px;margin:16px 0}pre{max-height:300px;overflow:auto;font-size: 13px}p{line-height:1.7}summary{cursor:pointer;margin:16px 0}@media(max-width:760px){.accounts{grid-template-columns:1fr}.actions{flex-wrap:wrap}}
</style>
