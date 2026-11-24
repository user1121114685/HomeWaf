<template>
  <div class="protocol-defense" data-testid="protocol-defense" :aria-busy="loading || saving">
    <PageActions>
      <ElButton :loading="loading" :disabled="saving" @click="loadAll">{{ label('重新读取', 'Reload') }}</ElButton>
      <ElButton v-if="activeTab === 'profiles'" type="primary" :disabled="loading || saving" @click="createProfile">
        {{ label('添加伪装画像', 'Add camouflage profile') }}
      </ElButton>
      <ElButton v-else :loading="eventsLoading" :disabled="saving" @click="loadEvents">
        {{ label('刷新评分', 'Refresh scores') }}
      </ElButton>
    </PageActions>

    <ElAlert v-if="error" :title="error" type="error" :closable="false" show-icon>
      <ElButton text :disabled="saving" @click="loadAll">{{ label('重试', 'Retry') }}</ElButton>
    </ElAlert>

    <ElSkeleton v-if="loading && !draft" :rows="8" animated />
    <template v-else-if="draft">
      <ElTabs v-model="activeTab" class="defense-tabs" @tab-change="tabChanged">
        <ElTabPane :label="label('伪装画像', 'Camouflage profiles')" name="profiles">
          <section class="content-section" aria-labelledby="profiles-title">
            <div class="section-heading">
              <div>
                <h2 id="profiles-title">{{ label('独立探针监听', 'Independent probe listeners') }}</h2>
                <p>
                  {{
                    label(
                      '每条画像单独选择协议、端口和版本。仅保存并启用的画像会监听连接。',
                      'Each profile has its own protocol, port and reported version. Only saved, enabled profiles accept connections.',
                    )
                  }}
                </p>
              </div>
              <ElTag :type="enabledCount ? 'success' : 'info'">{{ enabledCountLabel }}</ElTag>
            </div>

            <ElTable
              v-if="draft.profiles.length"
              :data="draft.profiles"
              row-key="id"
              class="profile-table"
              :empty-text="label('暂无伪装画像', 'No camouflage profiles')"
            >
              <ElTableColumn :label="label('画像', 'Profile')" min-width="210">
                <template #default="{ row }">
                  <div class="profile-cell">
                    <strong>{{ row.name }}</strong>
                    <span>{{ catalogProfile(row.protocol_profile)?.name || row.protocol_profile }}</span>
                  </div>
                </template>
              </ElTableColumn>
              <ElTableColumn :label="label('监听', 'Listener')" min-width="150">
                <template #default="{ row }">
                  <span class="port-value">{{ transportName(row.protocol_profile) }} :{{ row.port }}</span>
                </template>
              </ElTableColumn>
              <ElTableColumn :label="label('Nmap 服务名', 'Nmap service')" min-width="145">
                <template #default="{ row }">
                  <div class="profile-cell">
                    <code>{{ catalogProfile(row.protocol_profile)?.nmap_service || '-' }}</code>
                    <small>{{ catalogProfile(row.protocol_profile)?.nmap_probe }}</small>
                  </div>
                </template>
              </ElTableColumn>
              <ElTableColumn :label="label('运行状态', 'Runtime')" min-width="150">
                <template #default="{ row }">
                  <ElTag v-if="profileStatus(row.id)?.running" type="success">{{ label('监听中', 'Listening') }}</ElTag>
                  <ElTag v-else-if="row.enabled" type="danger">{{ label('未运行', 'Not running') }}</ElTag>
                  <ElTag v-else type="info">{{ label('已停用', 'Disabled') }}</ElTag>
                  <p v-if="profileStatus(row.id)?.last_error" class="row-error">
                    {{ profileStatus(row.id)?.last_error }}
                  </p>
                </template>
              </ElTableColumn>
              <ElTableColumn :label="label('启用', 'Enabled')" width="100" align="center">
                <template #default="{ row }">
                  <ElSwitch v-model="row.enabled" :aria-label="label(`启用 ${row.name}`, `Enable ${row.name}`)" />
                </template>
              </ElTableColumn>
              <ElTableColumn :label="label('操作', 'Actions')" width="150" fixed="right">
                <template #default="{ row }">
                  <div class="row-actions">
                    <ElButton text @click="editProfile(row)">{{ label('编辑', 'Edit') }}</ElButton>
                    <ElButton text type="danger" @click="removeProfile(row.id)">{{ label('删除', 'Delete') }}</ElButton>
                  </div>
                </template>
              </ElTableColumn>
            </ElTable>

            <div v-else class="empty-profiles">
              <span>{{ label('还没有伪装画像', 'No camouflage profiles yet') }}</span>
              <ElButton type="primary" @click="createProfile">{{
                label('添加第一条画像', 'Add the first profile')
              }}</ElButton>
            </div>
          </section>

          <section class="content-section lan-section" aria-labelledby="lan-title">
            <div class="section-heading">
              <div>
                <h2 id="lan-title">{{ label('本地局域网范围', 'Local LAN scope') }}</h2>
                <p>
                  {{
                    label(
                      '范围内的探针只记录评分，不触发防火墙封禁。自动模式合并本机地址前缀与管理页面来源 IP 的 /24 或 /64。',
                      'Hits within these prefixes are scored without firewall blocks. Auto mode combines local interface prefixes with the management page client IP /24 or /64.',
                    )
                  }}
                </p>
              </div>
            </div>

            <ElRadioGroup v-model="draft.local_cidr_mode" class="mode-control" aria-label="局域网范围模式">
              <ElRadioButton value="auto">{{ label('自动识别', 'Automatic') }}</ElRadioButton>
              <ElRadioButton value="custom">{{ label('自定义网段', 'Custom prefixes') }}</ElRadioButton>
            </ElRadioGroup>
            <div v-if="draft.local_cidr_mode === 'auto'" class="prefix-list" aria-live="polite">
              <code v-for="prefix in status.local_cidrs" :key="prefix">{{ prefix }}</code>
              <span v-if="!status.local_cidrs?.length" class="muted-text">
                {{
                  label(
                    '尚未识别到本机网段，所有非回环地址都按非本地处理。',
                    'No local prefix was detected. All non-loopback addresses are treated as non-local.',
                  )
                }}
              </span>
            </div>
            <ElFormItem
              v-else
              :label="label('IPv4 /24 或更窄、IPv6 /64 或更窄', 'IPv4 /24 or narrower, IPv6 /64 or narrower')"
              class="cidr-input"
            >
              <ElInput
                v-model="localCIDRText"
                type="textarea"
                :rows="3"
                placeholder="192.168.1.0/24&#10;fdea:7581:2902::/64"
              />
            </ElFormItem>

            <div class="block-policy">
              <ElTag type="danger">{{ label('非本地命中封禁 90 天', 'Block non-local hits for 90 days') }}</ElTag>
              <span>{{
                label(
                  '所有不在上述局域网前缀内的有效来源 IP 都会写入系统防火墙封禁。',
                  'Every valid source IP outside these LAN prefixes is added to the system firewall block list.',
                )
              }}</span>
            </div>
          </section>

          <div class="source-note">
            <strong>{{ label('Nmap 识别依据', 'Nmap evidence') }}</strong>
            <span>{{
              label(
                '探针与服务名称参考 Nmap 项目的 service-probes，感谢 Nmap 项目及贡献者。响应由 HomeWaf 使用 Go 独立实现。HTTP 品牌外观不等于 Nmap 能识别该厂商，具体证据见协议画像。',
                'Probe and service names reference the Nmap Project service-probes. Thanks to the Nmap project and its contributors. HomeWaf implements responses independently in Go. An HTTP brand appearance does not imply Nmap will identify that vendor. See each profile for its evidence.',
              )
            }}</span>
          </div>

          <div class="save-row">
            <div>
              <strong>{{
                dirty
                  ? label('有未保存修改', 'Unsaved changes')
                  : label(`${enabledCount} 条画像已保存`, `${enabledCount} profiles enabled`)
              }}</strong>
              <small>{{
                label(
                  '保存后配置持久化；启用画像的监听也在本次保存时启动或停止。',
                  'Saving persists the configuration and starts or stops enabled listeners.',
                )
              }}</small>
            </div>
            <ElButton type="primary" :loading="saving" :disabled="!dirty" @click="save">{{
              label('保存并应用', 'Save and apply')
            }}</ElButton>
          </div>
        </ElTabPane>

        <ElTabPane :label="label('评分检测', 'Score detection')" name="scores">
          <section class="content-section" aria-labelledby="score-title">
            <div class="section-heading">
              <div>
                <h2 id="score-title">{{ label('探针命中记录', 'Probe hit records') }}</h2>
                <p>
                  {{
                    label(
                      '按连接内容识别协议、探针和版本线索，并显示局域网判定与封禁结果。',
                      'Classify protocol, probe and version evidence from each connection, with LAN classification and block outcome.',
                    )
                  }}
                </p>
              </div>
              <div class="score-actions">
                <div class="score-summary">
                  <strong>{{ events.length }}</strong
                  ><span>{{ label('当前记录', 'loaded records') }}</span>
                </div>
                <ElButton text type="danger" :disabled="eventsLoading || !events.length" @click="clearEvents">
                  {{ label('清空记录', 'Clear records') }}
                </ElButton>
              </div>
            </div>

            <ElAlert v-if="eventsError" :title="eventsError" type="warning" :closable="false" show-icon>
              <ElButton text @click="loadEvents">{{ label('重试', 'Retry') }}</ElButton>
            </ElAlert>
            <ElSkeleton v-if="eventsLoading && !events.length" :rows="4" animated />
            <ElTable
              v-else
              :data="events"
              row-key="id"
              class="event-table"
              :empty-text="label('暂无协议探针命中记录', 'No protocol probe hits yet')"
            >
              <ElTableColumn :label="label('时间', 'Time')" min-width="165">
                <template #default="{ row }">{{ formatTime(row.time) }}</template>
              </ElTableColumn>
              <ElTableColumn prop="client_ip" :label="label('来源 IP', 'Client IP')" min-width="150" />
              <ElTableColumn prop="port" :label="label('端口', 'Port')" width="90" />
              <ElTableColumn prop="protocol" :label="label('协议', 'Protocol')" min-width="130" />
              <ElTableColumn prop="probe" :label="label('探针', 'Probe')" min-width="175" />
              <ElTableColumn prop="version" :label="label('版本线索', 'Version evidence')" min-width="160" />
              <ElTableColumn :label="label('评分', 'Score')" width="95" align="center">
                <template #default="{ row }"
                  ><ElTag :type="scoreType(row.score)">{{ row.score }}</ElTag></template
                >
              </ElTableColumn>
              <ElTableColumn :label="label('处置', 'Action')" min-width="135">
                <template #default="{ row }">
                  <ElTag v-if="row.blocked" type="danger">{{ label('已封禁 90 天', 'Blocked 90 days') }}</ElTag>
                  <ElTag v-else-if="row.block_error" type="warning">{{ label('封禁失败', 'Block failed') }}</ElTag>
                  <ElTag v-else-if="row.local_lan" type="success">{{ label('本地局域网', 'Local LAN') }}</ElTag>
                  <ElTag v-else type="info">{{ label('已记录', 'Observed') }}</ElTag>
                </template>
              </ElTableColumn>
              <ElTableColumn type="expand" width="48">
                <template #default="{ row }">
                  <pre class="event-detail">{{ detail(row) }}</pre>
                </template>
              </ElTableColumn>
            </ElTable>
          </section>
        </ElTabPane>
      </ElTabs>
    </template>

    <ElDialog
      v-model="dialogVisible"
      :title="
        editingId ? label('编辑伪装画像', 'Edit camouflage profile') : label('添加伪装画像', 'Add camouflage profile')
      "
      width="min(720px, calc(100vw - 32px))"
      destroy-on-close
    >
      <ElForm v-if="profileDraft" label-position="top" class="form-grid">
        <ElFormItem :label="label('画像名称', 'Profile name')" required>
          <ElInput v-model="profileDraft.name" maxlength="80" />
        </ElFormItem>
        <ElFormItem :label="label('协议画像', 'Protocol profile')" required>
          <ElSelect v-model="profileDraft.protocol_profile" filterable style="width: 100%" @change="profileChanged">
            <ElOptionGroup v-for="group in groupedProfiles" :key="group.group" :label="group.label">
              <ElOption v-for="profile in group.items" :key="profile.code" :value="profile.code" :label="profile.name">
                <span class="profile-option"
                  ><strong>{{ profile.name }}</strong
                  ><small
                    >Nmap {{ profile.nmap_probe }} → {{ profile.nmap_service }} · {{ profile.nmap_evidence }}</small
                  ></span
                >
              </ElOption>
            </ElOptionGroup>
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="label('独立监听端口', 'Dedicated listener port')" required>
          <ElInputNumber
            v-model="profileDraft.port"
            :min="1"
            :max="65535"
            controls-position="right"
            style="width: 100%"
          />
          <p class="field-help">
            {{
              label(
                'HTTP 默认 8080，HTTPS 默认 8443。监听端口不占用业务服务配置。',
                'HTTP defaults to 8080 and HTTPS to 8443. This listener is independent from service ports.',
              )
            }}
          </p>
        </ElFormItem>
        <ElFormItem
          v-if="selectedCatalogProfile?.version_editable"
          :label="label('响应版本', 'Reported version')"
          required
        >
          <ElInput v-model="profileDraft.version" maxlength="128" show-word-limit />
        </ElFormItem>
        <ElFormItem :label="label('服务名称', 'Service name')">
          <ElInput v-model="profileDraft.server_name" maxlength="128" />
        </ElFormItem>
        <ElFormItem :label="label('监听地址族', 'IP version')">
          <ElSelect v-model="profileDraft.listen_ip_version" style="width: 100%">
            <ElOption value="dual" :label="label('IPv4 + IPv6', 'IPv4 + IPv6')" />
            <ElOption value="ipv4" label="IPv4" />
            <ElOption value="ipv6" label="IPv6" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="label('Nmap 服务识别', 'Nmap service recognition')" class="nmap-evidence">
          <div class="evidence-box">
            <code>{{ selectedCatalogProfile?.nmap_service || '-' }}</code>
            <div class="evidence-copy">
              <strong>{{ selectedCatalogProfile?.nmap_probe }}</strong>
              <span>{{ selectedCatalogProfile?.nmap_evidence }}</span>
            </div>
          </div>
        </ElFormItem>
        <ElFormItem :label="label('保存后启用监听', 'Enable listener after saving')">
          <ElSwitch
            v-model="profileDraft.enabled"
            :active-text="label('启用', 'Enabled')"
            :inactive-text="label('停用', 'Disabled')"
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">{{ label('取消', 'Cancel') }}</ElButton>
        <ElButton type="primary" @click="commitProfileDraft">{{ label('确认', 'Apply to draft') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import {
  ElAlert,
  ElButton,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElOptionGroup,
  ElRadioButton,
  ElRadioGroup,
  ElSelect,
  ElSkeleton,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
} from 'element-plus';
import PageActions from '@/ui/PageActions.vue';
import { camouflageGet, camouflagePost } from '@/apis/camouflage';

interface ProfileOption {
  code: string;
  name: string;
  group: string;
  transport: string;
  tls: boolean;
  default_port: number;
  default_version: string;
  version_editable: boolean;
  description: string;
  nmap_service: string;
  nmap_probe: string;
  nmap_evidence: string;
}

interface CamouflageProfile {
  id: string;
  name: string;
  enabled: boolean;
  port: number;
  protocol_profile: string;
  version: string;
  server_name: string;
  listen_ip_version: 'dual' | 'ipv4' | 'ipv6';
}

interface DefenseConfig {
  profiles: CamouflageProfile[];
  local_cidr_mode: 'auto' | 'custom';
  local_cidrs: string[];
}

interface ProbeEvent {
  id: string;
  time: number;
  client_ip: string;
  port: number;
  protocol: string;
  probe: string;
  version: string;
  score: number;
  confidence: string;
  evidence: string;
  local_lan: boolean;
  public: boolean;
  blocked: boolean;
  block_error?: string;
}

const { locale } = useI18n();
const label = (zh: string, en: string) => (locale.value === 'en_US' ? en : zh);
const loading = ref(false);
const saving = ref(false);
const eventsLoading = ref(false);
const error = ref('');
const eventsError = ref('');
const profiles = ref<ProfileOption[]>([]);
const events = ref<ProbeEvent[]>([]);
const draft = ref<DefenseConfig | null>(null);
const baseline = ref('');
const savedEnabled = ref<Record<string, boolean>>({});
const status = ref<{ profiles: Array<{ id: string; running: boolean; last_error?: string }>; local_cidrs: string[] }>({
  profiles: [],
  local_cidrs: [],
});
const activeTab = ref('profiles');
const dialogVisible = ref(false);
const profileDraft = ref<CamouflageProfile | null>(null);
const editingId = ref('');
const previousProtocol = ref('');
let live = true;

const groupedProfiles = computed(() => {
  const order = ['常用协议', '家庭常用', 'NAS 厂商画像', '数据库', '基础服务', '远程管理'];
  const labels: Record<string, [string, string]> = {
    常用协议: ['常用协议', 'Common protocols'],
    家庭常用: ['家庭常用', 'Home services'],
    'NAS 厂商画像': ['NAS 外观画像', 'NAS appearance profiles'],
    数据库: ['数据库', 'Databases'],
    基础服务: ['基础服务', 'Core services'],
    远程管理: ['远程管理', 'Remote access'],
  };
  const groups = new Map<string, ProfileOption[]>();
  profiles.value.forEach((profile) => groups.set(profile.group, [...(groups.get(profile.group) || []), profile]));
  return [...groups.entries()]
    .sort((a, b) => order.indexOf(a[0]) - order.indexOf(b[0]))
    .map(([group, items]) => {
      const groupLabel = labels[group] || [group, group];
      return { group, label: label(groupLabel[0], groupLabel[1]), items };
    });
});
const selectedCatalogProfile = computed(() =>
  profiles.value.find((profile) => profile.code === profileDraft.value?.protocol_profile),
);
const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== baseline.value);
const enabledCount = computed(() => draft.value?.profiles.filter((profile) => profile.enabled).length || 0);
const enabledCountLabel = computed(() => label(`${enabledCount.value} 条已启用`, `${enabledCount.value} enabled`));
const localCIDRText = computed({
  get: () => draft.value?.local_cidrs.join('\n') || '',
  set: (value: string) => {
    if (draft.value)
      draft.value.local_cidrs = value
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter(Boolean);
  },
});

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function makeId() {
  return crypto.randomUUID();
}

function catalogProfile(code: string) {
  return profiles.value.find((profile) => profile.code === code);
}

function profileStatus(id: string) {
  return status.value.profiles.find((item) => item.id === id);
}

function transportName(code: string) {
  return catalogProfile(code)?.transport.toUpperCase() || 'TCP';
}

function defaultPort(profile: ProfileOption) {
  if (profile.default_port === 80) return 8080;
  if (profile.default_port === 443) return 8443;
  return profile.default_port;
}

async function loadAll() {
  loading.value = true;
  error.value = '';
  try {
    const [result, profileRows] = await Promise.all([
      camouflageGet<{ config: DefenseConfig; status: typeof status.value }>('config'),
      camouflageGet<ProfileOption[]>('profiles'),
    ]);
    if (!live) return;
    draft.value = clone(result.config);
    status.value = result.status;
    profiles.value = profileRows || [];
    savedEnabled.value = Object.fromEntries(result.config.profiles.map((profile) => [profile.id, profile.enabled]));
    baseline.value = JSON.stringify(draft.value);
    if (activeTab.value === 'scores') await loadEvents();
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
    events.value = await camouflageGet<ProbeEvent[]>('events', { limit: 100 });
  } catch (cause: any) {
    if (live) eventsError.value = cause?.message || label('评分记录读取失败', 'Failed to load score events');
  } finally {
    if (live) eventsLoading.value = false;
  }
}

function tabChanged(tab: string | number) {
  if (tab === 'scores') void loadEvents();
}

function createProfile() {
  const selected = profiles.value.find((profile) => profile.code === 'http') || profiles.value[0];
  if (!selected) return;
  editingId.value = '';
  previousProtocol.value = selected.code;
  profileDraft.value = {
    id: makeId(),
    name: label('HTTP 伪装画像', 'HTTP camouflage profile'),
    enabled: false,
    port: defaultPort(selected),
    protocol_profile: selected.code,
    version: selected.default_version,
    server_name: 'nas',
    listen_ip_version: 'dual',
  };
  dialogVisible.value = true;
}

function editProfile(profile: CamouflageProfile) {
  editingId.value = profile.id;
  previousProtocol.value = profile.protocol_profile;
  profileDraft.value = clone(profile);
  dialogVisible.value = true;
}

function profileChanged(code: string) {
  const selected = catalogProfile(code);
  const previous = catalogProfile(previousProtocol.value);
  if (!selected || !profileDraft.value) return;
  if (!profileDraft.value.version || profileDraft.value.version === previous?.default_version) {
    profileDraft.value.version = selected.default_version;
  }
  if (previous && profileDraft.value.port === defaultPort(previous)) profileDraft.value.port = defaultPort(selected);
  previousProtocol.value = code;
}

function commitProfileDraft() {
  if (!draft.value || !profileDraft.value) return;
  profileDraft.value.name = profileDraft.value.name.trim();
  if (!profileDraft.value.name) {
    ElMessage.error(label('请输入画像名称', 'Enter a profile name'));
    return;
  }
  const index = draft.value.profiles.findIndex((profile) => profile.id === editingId.value);
  if (index < 0) draft.value.profiles.push(clone(profileDraft.value));
  else draft.value.profiles.splice(index, 1, clone(profileDraft.value));
  dialogVisible.value = false;
}

async function removeProfile(id: string) {
  if (!draft.value) return;
  const profile = draft.value.profiles.find((item) => item.id === id);
  if (!profile) return;
  try {
    await ElMessageBox.confirm(
      label(
        `删除“${profile.name}”画像后，保存时会停止对应监听。确认删除？`,
        `Delete “${profile.name}”? Saving will stop its listener.`,
      ),
      label('删除伪装画像', 'Delete camouflage profile'),
      { type: 'warning' },
    );
    draft.value.profiles = draft.value.profiles.filter((item) => item.id !== id);
  } catch {
    return;
  }
}

async function save() {
  if (!draft.value || saving.value || !dirty.value) return;
  const newlyEnabled = draft.value.profiles.some((profile) => profile.enabled && !savedEnabled.value[profile.id]);
  if (newlyEnabled) {
    try {
      await ElMessageBox.confirm(
        label(
          '启用画像后，局域网范围之外的有效来源 IP 命中会写入系统防火墙封禁 90 天。确认保存并应用？',
          'Enabling these profiles adds every valid hit source outside the configured LAN prefixes to the system firewall block list for 90 days. Save and apply?',
        ),
        label('启用伪装监听', 'Enable camouflage listeners'),
        { type: 'warning' },
      );
    } catch {
      return;
    }
  }
  saving.value = true;
  try {
    const result = await camouflagePost<{ config: DefenseConfig; status: typeof status.value }>('save', draft.value);
    draft.value = clone(result.config);
    status.value = result.status;
    savedEnabled.value = Object.fromEntries(result.config.profiles.map((profile) => [profile.id, profile.enabled]));
    baseline.value = JSON.stringify(draft.value);
    ElMessage.success(label('协议防御配置已保存', 'Protocol defense configuration saved'));
  } catch (cause: any) {
    ElMessage.error(
      cause?.message || label('保存失败，修改仍保留在页面中', 'Save failed. Your changes remain in the form'),
    );
  } finally {
    saving.value = false;
  }
}

async function clearEvents() {
  try {
    await ElMessageBox.confirm(
      label('清空全部评分检测记录？此操作无法撤销。', 'Clear all score detection records? This cannot be undone.'),
      label('清空记录', 'Clear records'),
      { type: 'warning' },
    );
    await camouflagePost('events/clear', {});
    events.value = [];
    ElMessage.success(label('评分记录已清空', 'Score records cleared'));
  } catch (cause: any) {
    if (cause !== 'cancel' && cause !== 'close')
      ElMessage.error(cause?.message || label('清空记录失败', 'Failed to clear records'));
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

function detail(row: ProbeEvent) {
  return JSON.stringify(
    {
      confidence: row.confidence,
      evidence: row.evidence,
      local_lan: row.local_lan,
      public: row.public,
      blocked: row.blocked,
      block_error: row.block_error,
    },
    null,
    2,
  );
}

onMounted(loadAll);
onBeforeUnmount(() => {
  live = false;
});
</script>

<style scoped>
.protocol-defense {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.defense-tabs {
  min-width: 0;
}
.content-section {
  padding: 20px 22px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  background: var(--hw-panel);
  min-width: 0;
}
.section-heading,
.save-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}
h2,
p {
  margin-top: 0;
}
h2 {
  margin-bottom: 6px;
  font-size: 18px;
}
.section-heading p,
.field-help,
.save-row small,
.muted-text {
  color: var(--hw-muted);
  line-height: 1.6;
}
.profile-table,
.event-table {
  width: 100%;
  margin-top: 18px;
}
.empty-profiles {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 22px 0 6px;
  color: var(--hw-muted);
}
.profile-cell {
  display: grid;
  gap: 3px;
}
.profile-cell span,
.muted-text {
  color: var(--hw-muted);
}
.profile-cell small {
  color: var(--hw-muted);
  font-size: 12px;
}
.port-value,
code {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.row-actions {
  display: flex;
  align-items: center;
}
.row-error {
  margin: 5px 0 0;
  color: var(--el-color-danger);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.lan-section {
  display: grid;
  gap: 14px;
  margin-top: 16px;
}
.mode-control {
  width: fit-content;
}
.prefix-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 32px;
}
.prefix-list code {
  padding: 5px 8px;
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  background: var(--hw-subtle);
}
.cidr-input {
  max-width: 620px;
  margin: 0;
}
.block-policy {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--hw-ink);
}
.source-note {
  display: grid;
  grid-template-columns: 145px minmax(0, 1fr);
  gap: 12px;
  padding: 14px 16px;
  border-left: 3px solid var(--hw-accent);
  background: var(--hw-subtle);
  color: var(--hw-muted);
  line-height: 1.6;
}
.source-note strong {
  color: var(--hw-ink);
}
.save-row {
  align-items: center;
  padding: 16px 2px 0;
  border-top: 1px solid var(--hw-line);
}
.save-row > div:first-child {
  display: grid;
  gap: 4px;
}
.score-actions {
  display: flex;
  align-items: center;
  gap: 18px;
}
.score-summary {
  display: grid;
  justify-items: end;
  gap: 2px;
  color: var(--hw-muted);
}
.score-summary strong {
  color: var(--hw-ink);
  font: 700 22px var(--font-mono);
}
.event-detail {
  margin: 0;
  padding: 12px;
  background: var(--hw-subtle);
  color: var(--hw-muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 18px;
}
.field-help {
  margin: 5px 0 0;
  font-size: 12px;
}
.profile-option {
  display: grid;
  gap: 3px;
  line-height: 1.35;
  white-space: normal;
}
.profile-option small {
  color: var(--hw-muted);
}
.nmap-evidence {
  grid-column: 1 / -1;
}
.evidence-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
  padding: 11px 12px;
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  background: var(--hw-subtle);
  line-height: 1.5;
}
.evidence-box span {
  color: var(--hw-muted);
}
.evidence-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}
@media (max-width: 760px) {
  .section-heading,
  .save-row {
    flex-direction: column;
    align-items: stretch;
  }
  .content-section {
    padding: 16px;
  }
  .empty-profiles {
    align-items: stretch;
    flex-direction: column;
  }
  .block-policy {
    align-items: flex-start;
    flex-direction: column;
    gap: 7px;
  }
  .source-note {
    grid-template-columns: 1fr;
    gap: 4px;
  }
  .score-actions {
    justify-content: space-between;
  }
  .form-grid {
    grid-template-columns: 1fr;
  }
  .nmap-evidence {
    grid-column: auto;
  }
  .save-row .el-button {
    width: 100%;
  }
}
</style>
