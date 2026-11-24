<template>
  <div class="protocol-defense" data-testid="protocol-defense" :aria-busy="loading || saving">
    <PageActions>
      <ElButton :loading="loading" :disabled="saving" @click="loadAll">{{ label('重新读取', 'Reload') }}</ElButton>
      <ElButton :loading="eventsLoading" :disabled="saving" @click="loadEvents">{{ label('刷新评分', 'Refresh scores') }}</ElButton>
    </PageActions>

    <ElAlert v-if="error" :title="error" type="error" :closable="false" show-icon>
      <ElButton text :disabled="saving" @click="loadAll">{{ label('重试', 'Retry') }}</ElButton>
    </ElAlert>

    <ElSkeleton v-if="loading && !draft" :rows="8" animated />
    <template v-else-if="draft">
      <section class="settings-panel" aria-labelledby="camouflage-settings-title">
        <div class="section-heading">
          <div>
            <h2 id="camouflage-settings-title">{{ label('协议伪装', 'Protocol camouflage') }}</h2>
            <p>{{ label('伪装监听独立于业务服务端口。保存并启用后才会接受探针连接。', 'The decoy listener is independent from business service ports and only accepts probes after it is enabled.') }}</p>
          </div>
          <div class="settings-controls">
            <ElTag v-if="draft.enabled" :type="status.running ? 'success' : 'warning'">{{ statusLabel }}</ElTag>
            <ElSwitch v-model="draft.enabled" :active-text="label('已启用', 'Enabled')" :inactive-text="label('已停用', 'Disabled')" />
          </div>
        </div>

        <ElForm label-position="top" class="settings-form">
          <div class="form-grid">
            <ElFormItem :label="label('协议画像', 'Protocol profile')">
              <ElSelect v-model="draft.profile" filterable style="width: 100%" @change="profileChanged">
                <ElOptionGroup v-for="group in groupedProfiles" :key="group.group" :label="group.label">
                  <ElOption v-for="profile in group.items" :key="profile.code" :value="profile.code" :label="profile.name">
                    <span class="profile-option"><strong>{{ profile.name }}</strong><small>{{ profile.description }}</small></span>
                  </ElOption>
                </ElOptionGroup>
              </ElSelect>
              <p class="field-help">{{ selectedProfile?.description || label('选择一个协议画像', 'Choose a protocol profile') }}</p>
            </ElFormItem>
            <ElFormItem :label="label('监听端口', 'Listen port')">
              <ElInputNumber v-model="draft.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
              <p class="field-help">{{ label('家庭 HTTP 默认 8080，HTTPS 默认 8443，可按实际空闲端口修改。', 'Home HTTP defaults to 8080 and HTTPS to 8443. Choose any unused port.') }}</p>
            </ElFormItem>
            <ElFormItem :label="label('伪装版本', 'Reported version')">
              <ElInput v-model="draft.version" maxlength="128" show-word-limit />
              <p class="field-help">{{ label('只影响伪装响应中的版本字段，不会改变 HomeWaf 自身版本。', 'This changes reported response fields only.') }}</p>
            </ElFormItem>
            <ElFormItem :label="label('服务名称', 'Service name')">
              <ElInput v-model="draft.server_name" maxlength="128" />
            </ElFormItem>
            <ElFormItem :label="label('监听地址族', 'IP version')">
              <ElSelect v-model="draft.listen_ip_version" style="width: 100%">
                <ElOption value="dual" :label="label('IPv4 + IPv6', 'IPv4 + IPv6')" />
                <ElOption value="ipv4" label="IPv4" />
                <ElOption value="ipv6" label="IPv6" />
              </ElSelect>
            </ElFormItem>
            <ElFormItem :label="label('非本地地址命中处置', 'Non-local hit action')">
              <div class="block-setting">
                <ElTag type="danger">{{ label('自动封禁', 'Automatic block') }}</ElTag>
                <span>{{ label('不在本机 IPv4 /24（IPv6 /64）内的地址命中后固定封禁 90 天', 'Hits outside the local IPv4 /24 (IPv6 /64) are blocked for 90 days') }}</span>
              </div>
            </ElFormItem>
          </div>
        </ElForm>

        <div class="source-note">
          <strong>{{ label('规则来源', 'Rule source') }}</strong>
          <span>{{ label('协议探针名称、请求结构和版本识别思路来自 Nmap service detection 数据库。HomeWaf 使用 Go 重新实现响应，厂商画像可单独选择。', 'Probe names, request structures and version-identification ideas are based on the Nmap service detection database. HomeWaf reimplements the responses in Go and keeps vendor profiles selectable.') }}</span>
        </div>

        <div class="save-row">
          <div>
            <strong>{{ dirty ? label('有未保存修改', 'Unsaved changes') : statusLabel }}</strong>
            <small v-if="status.local_cidrs?.length">{{ label('当前本机局域网判定', 'Local LAN prefixes') }} {{ status.local_cidrs.join('，') }}</small>
            <small v-else>{{ label('尚未读取到本机局域网地址，命中会按非本地地址处理。', 'No local LAN prefix was detected. Hits are treated as non-local.') }}</small>
          </div>
          <ElButton type="primary" :loading="saving" @click="save">{{ label('保存并应用', 'Save and apply') }}</ElButton>
        </div>
      </section>

      <section class="score-panel" aria-labelledby="score-title">
        <div class="section-heading">
          <div>
            <h2 id="score-title">{{ label('评分检测', 'Score detection') }}</h2>
            <p>{{ label('命中记录保留协议、探针、版本置信度、局域网判定和封禁结果。', 'Each hit keeps protocol, probe, version confidence, LAN classification and block result.') }}</p>
          </div>
          <div class="score-summary"><strong>{{ status.total_hits || 0 }}</strong><span>{{ label('累计命中', 'total hits') }}</span></div>
        </div>
        <ElAlert v-if="eventsError" :title="eventsError" type="warning" :closable="false" show-icon />
        <ElSkeleton v-if="eventsLoading && !events.length" :rows="4" animated />
        <ElTable v-else :data="events" row-key="id" class="event-table" :empty-text="label('暂无协议探针命中记录', 'No protocol probe hits yet')">
          <ElTableColumn :label="label('时间', 'Time')" min-width="165">
            <template #default="{ row }">{{ formatTime(row.time) }}</template>
          </ElTableColumn>
          <ElTableColumn prop="client_ip" :label="label('来源 IP', 'Client IP')" min-width="150" />
          <ElTableColumn prop="protocol" :label="label('协议', 'Protocol')" min-width="130" />
          <ElTableColumn prop="probe" :label="label('探针', 'Probe')" min-width="150" />
          <ElTableColumn prop="version" :label="label('版本', 'Version')" min-width="150" />
          <ElTableColumn :label="label('评分', 'Score')" width="100" align="center">
            <template #default="{ row }"><ElTag :type="scoreType(row.score)">{{ row.score }}</ElTag></template>
          </ElTableColumn>
          <ElTableColumn :label="label('状态', 'State')" width="125">
            <template #default="{ row }">
              <ElTag v-if="row.blocked" type="danger">{{ label('已封禁', 'Blocked') }}</ElTag>
              <ElTag v-else-if="row.local_lan" type="success">{{ label('本地局域网', 'Local LAN') }}</ElTag>
              <ElTag v-else type="info">{{ label('已记录', 'Observed') }}</ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn type="expand" width="48">
            <template #default="{ row }"><pre class="event-detail">{{ detail(row) }}</pre></template>
          </ElTableColumn>
        </ElTable>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, ElInputNumber, ElMessage, ElMessageBox, ElOption, ElOptionGroup, ElSelect, ElSkeleton, ElSwitch, ElTable, ElTableColumn, ElTag } from 'element-plus';
import PageActions from '@/ui/PageActions.vue';
import { camouflageGet, camouflagePost } from '@/apis/camouflage';

const { locale } = useI18n();
const label = (zh: string, en: string) => (locale.value === 'en_US' ? en : zh);
const loading = ref(false);
const saving = ref(false);
const eventsLoading = ref(false);
const error = ref('');
const eventsError = ref('');
const profiles = ref<any[]>([]);
const events = ref<any[]>([]);
const draft = ref<any>(null);
const baseline = ref('');
const status = ref<any>({});
const lastProfile = ref('');
let live = true;

const groupedProfiles = computed(() => {
  const order = ['常用协议', '家庭常用', 'NAS 厂商画像', '数据库', '基础服务', '远程管理'];
  const labels: Record<string, [string, string]> = {
    常用协议: ['常用协议', 'Common protocols'],
    家庭常用: ['家庭常用', 'Home services'],
    'NAS 厂商画像': ['NAS 厂商画像', 'NAS profiles'],
    数据库: ['数据库', 'Databases'],
    基础服务: ['基础服务', 'Core services'],
    远程管理: ['远程管理', 'Remote access'],
  };
  const groups = new Map<string, any[]>();
  profiles.value.forEach((profile) => groups.set(profile.group, [...(groups.get(profile.group) || []), profile]));
  return [...groups.entries()]
    .sort((a, b) => order.indexOf(a[0]) - order.indexOf(b[0]))
    .map(([group, items]) => {
      const groupLabel = labels[group] || [group, group];
      return { group, label: label(groupLabel[0], groupLabel[1]), items };
    });
});
const selectedProfile = computed(() => profiles.value.find((profile) => profile.code === draft.value?.profile));
const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== baseline.value);
const statusLabel = computed(() => {
  if (!draft.value?.enabled) return label('已停用', 'Disabled');
  if (status.value.running) return label(`运行中 · :${status.value.port}`, `Running · :${status.value.port}`);
  return label('已启用但未确认监听', 'Enabled, listener not confirmed');
});

function clone(value: any) {
  return JSON.parse(JSON.stringify(value));
}
async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const [config, profileRows] = await Promise.all([camouflageGet<any>('config'), camouflageGet<any[]>('profiles')]);
    if (!live) return;
    draft.value = clone(config.config);
    status.value = config.status || {};
    profiles.value = profileRows || [];
    lastProfile.value = draft.value.profile;
    baseline.value = JSON.stringify(draft.value);
    await loadEvents();
  } catch (cause: any) {
    if (live) error.value = cause?.message || label('协议防御配置读取失败', 'Failed to load protocol defense');
  } finally {
    if (live) loading.value = false;
  }
}
async function loadEvents() {
  eventsLoading.value = true;
  eventsError.value = '';
  try {
    events.value = await camouflageGet<any[]>('events', { limit: 100 });
  } catch (cause: any) {
    if (live) eventsError.value = cause?.message || label('评分记录读取失败', 'Failed to load score events');
  } finally {
    if (live) eventsLoading.value = false;
  }
}
function profileChanged() {
  const profile = selectedProfile.value;
  if (!profile || !draft.value) return;
  const previous = profiles.value.find((item) => item.code === lastProfile.value);
  if (!draft.value.version || draft.value.version === previous?.default_version) draft.value.version = profile.default_version;
  if (draft.value.port === 8080 || draft.value.port === 8443 || draft.value.port === previous?.default_port) {
    draft.value.port = profile.tls ? 8443 : profile.default_port === 80 || profile.default_port === 443 ? 8080 : profile.default_port;
  }
  lastProfile.value = profile.code;
}
async function save() {
  if (!draft.value || saving.value) return;
  if (draft.value.enabled) {
    try {
      await ElMessageBox.confirm(
        label('启用后，本机局域网 /24 之外的地址命中会按 90 天自动写入系统防火墙封禁。确认保存？', 'Enabling this will block hits outside the local /24 in the system firewall for 90 days. Save?'),
        label('应用协议伪装', 'Apply protocol camouflage'),
        { type: 'warning' },
      );
    } catch {
      return;
    }
  }
  saving.value = true;
  try {
    const result = await camouflagePost<any>('save', draft.value);
    draft.value = clone(result.config);
    status.value = result.status || {};
    baseline.value = JSON.stringify(draft.value);
    ElMessage.success(label('协议伪装已应用', 'Protocol camouflage applied'));
    await loadEvents();
  } catch (cause: any) {
    ElMessage.error(cause?.message || label('保存失败，修改仍保留在页面中', 'Save failed. Your changes remain in the form'));
  } finally {
    saving.value = false;
  }
}
function formatTime(value: number) {
  return value ? new Date(value * 1000).toLocaleString() : '-';
}
function scoreType(score: number) {
  if (score >= 90) return 'danger';
  if (score >= 70) return 'warning';
  return 'info';
}
function detail(row: any) {
  return JSON.stringify({ evidence: row.evidence, confidence: row.confidence, local_lan: row.local_lan, public: row.public, blocked: row.blocked, block_error: row.block_error }, null, 2);
}
onMounted(loadAll);
onBeforeUnmount(() => {
  live = false;
});
</script>

<style scoped>
.protocol-defense { display: grid; gap: 20px; min-width: 0; }
.section-heading, .save-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
h2, p { margin-top: 0; }
h2 { margin-bottom: 6px; font-size: 18px; }
.section-heading p, .field-help, .save-row small { color: var(--hw-muted); line-height: 1.6; }
.settings-panel, .score-panel { padding: 22px; border: 1px solid var(--hw-line); border-radius: var(--hw-r-lg); background: var(--hw-panel); min-width: 0; }
.settings-form { margin-top: 22px; }
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px 20px; }
.block-setting { display: flex; align-items: center; gap: 12px; min-height: 32px; color: var(--hw-ink); }
.settings-controls { display: flex; align-items: center; gap: 12px; }
.field-help { margin: 6px 0 0; font-size: 12px; }
.profile-option { display: grid; gap: 2px; line-height: 1.3; }
.profile-option small { color: var(--hw-muted); }
.source-note { display: grid; grid-template-columns: 100px minmax(0, 1fr); gap: 12px; padding: 14px 16px; border-left: 3px solid var(--hw-accent); background: var(--hw-subtle); color: var(--hw-muted); line-height: 1.6; }
.source-note strong { color: var(--hw-ink); }
.save-row { align-items: center; margin-top: 22px; padding-top: 18px; border-top: 1px solid var(--hw-line); }
.save-row > div:first-child { display: grid; gap: 4px; }
.score-summary { display: grid; justify-items: end; gap: 2px; color: var(--hw-muted); }
.score-summary strong { color: var(--hw-ink); font: 700 24px var(--font-mono); }
.event-table { width: 100%; margin-top: 18px; }
.event-detail { margin: 0; padding: 12px; background: var(--hw-subtle); color: var(--hw-muted); white-space: pre-wrap; overflow-wrap: anywhere; }
@media (max-width: 760px) {
  .section-heading, .save-row { flex-direction: column; align-items: stretch; }
  .settings-controls { justify-content: space-between; min-height: 36px; }
  .form-grid { grid-template-columns: 1fr; }
  .settings-panel, .score-panel { padding: 16px; }
  .source-note { grid-template-columns: 1fr; gap: 4px; }
  .save-row .el-button { width: 100%; }
}
</style>
