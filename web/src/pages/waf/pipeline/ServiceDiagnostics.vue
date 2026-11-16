<template>
  <div class="service-diagnostics" data-testid="service-diagnostics">
    <PageActions>
      <ElButton :disabled="busy" @click="goBack"
        ><ArrowLeft class="action-icon" aria-hidden="true" />返回我的服务</ElButton
      >
      <ElButton :loading="refreshing" :disabled="busy || !validPort" @click="refreshAll"
        ><Refresh class="action-icon" aria-hidden="true" />刷新诊断</ElButton
      >
    </PageActions>
    <ElAlert
      v-if="!validPort"
      title="服务端口无效，请返回我的服务重新选择。"
      type="error"
      :closable="false"
      show-icon
    />
    <div v-else-if="!service && loading.overview" class="diagnostic-empty" role="status">正在读取服务状态…</div>
    <div v-else-if="!service" class="diagnostic-empty" role="alert">
      <h2>{{ errors.overview ? '服务状态读取失败' : '未找到此服务' }}</h2>
      <p>{{ errors.overview || `端口 ${port} 的服务可能已删除，请返回我的服务核对。` }}</p>
      <ElButton v-if="errors.overview" :loading="loading.overview" @click="loadOverview">重试读取服务</ElButton>
      <ElButton @click="goBack">返回我的服务</ElButton>
    </div>
    <template v-else>
      <ElAlert
        v-if="errors.overview"
        :title="`服务状态刷新失败，以下保留上次结果：${errors.overview}`"
        type="warning"
        :closable="false"
        show-icon
      />
      <section class="diagnostic-service" aria-labelledby="diagnostic-service-name">
        <div class="service-identity">
          <span class="service-symbol"><Connection aria-hidden="true" /></span>
          <div>
            <h2 id="diagnostic-service-name" ref="serviceHeading" tabindex="-1">
              {{ service.config.name || `服务 ${port}` }}
            </h2>
            <p>
              <span>{{ service.config.protocol.toUpperCase() }} · :{{ port }}</span
              ><span class="upstream">上游 {{ service.config.upstream }}</span>
            </p>
          </div>
        </div>
        <div class="service-actions">
          <ElButton :disabled="busy" @click="openLogs">查看访问记录</ElButton>
          <ElButton :disabled="busy || !!errors.overview" @click="configure">配置服务</ElButton>
        </div>
        <dl class="service-facts">
          <div>
            <dt>监听状态</dt>
            <dd>
              <ElTag :type="listenerTone">{{ listenerLabel }}</ElTag>
            </dd>
          </div>
          <div>
            <dt>访问开关</dt>
            <dd>
              <ElTag :type="service.config.policy.access.enabled ? 'info' : 'warning'">{{
                service.config.policy.access.enabled ? '已开启' : '已关闭'
              }}</ElTag>
            </dd>
          </div>
          <div>
            <dt>检测模式</dt>
            <dd>{{ modes.block }} 项拦截 · {{ modes.observe }} 项观察</dd>
          </div>
          <div>
            <dt>状态更新</dt>
            <dd>{{ updated.overview ? diagnosticTime(updated.overview) : '尚未更新' }}</dd>
          </div>
        </dl>
      </section>
      <div class="diagnostic-notices">
        <ElAlert
          v-if="service.listener?.error"
          :title="`监听异常：${service.listener.error}`"
          type="error"
          :closable="false"
          show-icon
        />
        <ElAlert v-if="issue" :title="issue" type="warning" :closable="false" show-icon />
        <ElAlert
          v-if="!service.listener?.running"
          title="当前未确认监听运行。请核对服务配置和启用状态；历史请求不代表当前入口可访问。"
          type="info"
          :closable="false"
          show-icon
        />
        <p v-else class="diagnostic-guidance">监听已启动；请通过 HomeWaf 入口实际访问，并在记录中确认外部请求。</p>
        <ElAlert
          v-if="!service.config.policy.access.enabled"
          title="访问开关已关闭。仅释放处罚不会重新开放服务，请在配置中核对访问设置。"
          type="warning"
          :closable="false"
          show-icon
        />
        <p v-if="modes.observe" class="diagnostic-guidance">
          观察项只记录对应命中；其他拦截项、访问限制、IP 名单和登录要求仍有效。
        </p>
        <ElAlert
          v-if="service.locked"
          title="此服务存在端口锁定，请在“当前处罚”中核对原因与影响范围。"
          type="warning"
          :closable="false"
          show-icon
        />
        <div v-if="service.config.defense_state || service.defense?.error" class="defense-notice" role="status">
          <div>
            <strong>{{ service.defense?.error || defenseLabel }}</strong>
            <p v-if="service.config.defense_state">
              恢复时间：{{ diagnosticTime(service.config.defense_state.expires_at * 1000) }}。该状态由异常访问处置管理。
            </p>
          </div>
          <ElButton :disabled="busy || !!errors.overview" @click="configure('defense')">查看与恢复</ElButton>
        </div>
      </div>
      <ElAlert
        v-if="errors.stats"
        :title="`运行指标读取失败${stats ? '，以下为上次结果' : ''}：${errors.stats}`"
        type="warning"
        :closable="false"
        show-icon
      />
      <dl class="diagnostic-totals" :aria-busy="loading.stats">
        <div v-for="item in dailyMetrics" :key="item.key">
          <dt>{{ item.label }}</dt>
          <dd>
            {{ loading.stats && !stats ? '读取中…' : diagnosticNumber(stats?.port?.[item.key])
            }}<small v-if="typeof stats?.port?.[item.key] === 'number'">次</small>
          </dd>
        </div>
      </dl>
      <p class="snapshot-note">
        今日计数按服务器本地日期重置。{{
          updated.stats ? `指标更新于 ${diagnosticTime(updated.stats)}` : '指标尚未读取成功'
        }}。本页手动刷新。
      </p>
      <ElTabs v-model="activeTab" class="diagnostic-tabs" :before-leave="allowTabChange">
        <ElTabPane label="请求与事件" name="events">
          <section aria-label="请求与状态事件" class="diagnostic-panel">
            <div class="section-heading">
              <div>
                <h2>最近请求与状态变化</h2>
                <p>最近 100 条记录，按时间从新到旧。展开一条记录查看处理原因与检查链。</p>
              </div>
              <ElButton :loading="loading.logs" @click="loadLogs">刷新事件</ElButton>
            </div>
            <ElAlert
              v-if="errors.logs"
              :title="`事件读取失败${logs !== null ? '，当前保留上次结果' : ''}：${errors.logs}`"
              type="error"
              :closable="false"
              show-icon
            />
            <div class="event-filters">
              <label
                >查找来源或原因<ElInput
                  v-model="eventQuery"
                  clearable
                  placeholder="IP、请求标识、规则或原因"
                  aria-label="查找来源或原因"
              /></label>
              <label
                >记录类型<ElSelect v-model="eventKind" aria-label="记录类型" placeholder="全部记录"
                  ><ElOption label="全部记录" value="" /><ElOption label="请求处理" value="request" /><ElOption
                    label="状态变化"
                    value="state" /></ElSelect
              ></label>
              <span role="status">{{
                logs === null ? '尚未读取记录' : `${filteredEvents.length} 条匹配 / ${logs.length} 条已读取`
              }}</span>
              <ElButton v-if="eventQuery || eventKind" text @click="clearFilters">清除筛选</ElButton>
            </div>
            <div v-if="!logs?.length || !filteredEvents.length" class="diagnostic-empty" role="status">
              <p>{{ eventEmptyText }}</p>
              <ElButton v-if="errors.logs" :loading="loading.logs" @click="loadLogs">重试读取事件</ElButton>
              <ElButton v-else-if="logs?.length" @click="clearFilters">清除筛选</ElButton>
              <p v-else-if="logs !== null && !loading.logs">
                通过 HomeWaf 入口发起一次访问，再刷新记录。无记录本身不能证明服务没有流量。
              </p>
            </div>
            <ElTable
              v-else
              ref="eventsTable"
              :data="pagedEvents"
              :row-key="eventKey"
              :expand-row-keys="expandedEvents"
              @expand-change="(_row, rows) => (expandedEvents = rows.map(eventKey))"
              stripe
              class="events-table"
              aria-label="最近请求与状态变化"
            >
              <ElTableColumn type="expand" width="1" class-name="trace-expander"
                ><template #default="{ row }"><DiagnosticEventDetail :event="row" /></template
              ></ElTableColumn>
              <ElTableColumn label="时间 / 类型" min-width="170"
                ><template #default="{ row }"
                  ><span>{{ diagnosticTime(row.time_ms) }}</span
                  ><small>{{
                    row.kind === 'request' ? '请求处理' : row.kind === 'state' ? '状态变化' : row.kind
                  }}</small></template
                ></ElTableColumn
              >
              <ElTableColumn label="来源" min-width="150"
                ><template #default="{ row }"
                  ><code>{{ row.client_ip || '未记录来源' }}</code></template
                ></ElTableColumn
              >
              <ElTableColumn label="处理结果" min-width="160"
                ><template #default="{ row }"
                  ><ElTag :type="eventTone(row)">{{ diagnosticAction(row) }}</ElTag></template
                ></ElTableColumn
              >
              <ElTableColumn label="原因" min-width="260"
                ><template #default="{ row }"
                  ><span class="event-reason">{{ diagnosticReason(row) }}</span
                  ><small v-if="row.rule_id">规则 {{ row.rule_id }}</small></template
                ></ElTableColumn
              >
              <ElTableColumn label="详情" width="100"
                ><template #default="{ row }"
                  ><ElButton
                    link
                    type="primary"
                    :aria-expanded="expandedEvents.includes(eventKey(row))"
                    @click="eventsTable?.toggleRowExpansion(row)"
                    >{{ expandedEvents.includes(eventKey(row)) ? '收起详情' : '查看详情' }}</ElButton
                  ></template
                ></ElTableColumn
              >
            </ElTable>
            <div v-if="pagedEvents.length" class="event-cards">
              <article v-for="event in pagedEvents" :key="eventKey(event)">
                <header>
                  <span>{{ diagnosticTime(event.time_ms) }}</span
                  ><span>{{
                    event.kind === 'state' ? '状态变化' : event.kind === 'request' ? '请求处理' : event.kind
                  }}</span>
                </header>
                <code>{{ event.client_ip || '未记录来源' }}</code>
                <ElTag :type="eventTone(event)">{{ diagnosticAction(event) }}</ElTag>
                <p>{{ diagnosticReason(event) }}</p>
                <details>
                  <summary>查看处理详情</summary>
                  <DiagnosticEventDetail :event="event" />
                </details>
              </article>
            </div>
            <div class="event-footer">
              <span>{{ updated.logs ? `事件更新于 ${diagnosticTime(updated.logs)}` : '' }}</span
              ><ElPagination
                v-if="filteredEvents.length > pageSize"
                v-model:current-page="eventPage"
                :page-size="pageSize"
                :total="filteredEvents.length"
                layout="prev, pager, next"
                :pager-count="5"
                aria-label="事件分页"
              />
            </div>
          </section>
        </ElTabPane>
        <ElTabPane :label="punishmentCount === null ? '当前处罚' : `当前处罚 · ${punishmentCount}`" name="state">
          <section class="diagnostic-panel" aria-label="当前处罚管理">
            <div class="section-heading">
              <div>
                <h2>当前生效的处罚</h2>
                <p>核对来源、触发原因和影响范围后再操作。释放立即生效，再次触发规则仍可能重新处罚。</p>
              </div>
            </div>
            <ServicePunishments
              :key="port"
              ref="punishments"
              :port="port"
              :disabled="!!errors.overview"
              @changed="punishmentChanged"
              @busy-change="busy = $event"
              @loaded="punishmentCount = $event"
            />
          </section>
        </ElTabPane>
        <ElTabPane label="运行指标" name="metrics">
          <section class="diagnostic-panel" aria-label="运行指标">
            <div class="section-heading">
              <div>
                <h2>服务与实例运行指标</h2>
                <p>先检查本服务的连接与上游，再查看实例共享资源。未提供的数据不会补成零。</p>
              </div>
              <ElButton :loading="loading.stats" @click="loadStats">刷新指标</ElButton>
            </div>
            <div v-if="!stats" class="diagnostic-empty" role="status">
              <p>{{ loading.stats ? '正在读取运行指标…' : '暂无可用指标，请重试读取。' }}</p>
              <ElButton v-if="!loading.stats" @click="loadStats">重试读取指标</ElButton>
            </div>
            <template v-else>
              <section
                v-for="group in diagnosticMetricGroups"
                :key="group.title"
                class="metric-group"
                :data-scope="group.scope"
              >
                <h3>{{ group.title }}</h3>
                <p>{{ group.description }}</p>
                <dl>
                  <div v-for="[key, label] in group.fields" :key="key">
                    <dt>{{ label }}</dt>
                    <dd>{{ diagnosticNumber(metricValue(stats, key)) }}</dd>
                  </div>
                </dl>
              </section>
              <details class="raw-metrics">
                <summary>高级：完整指标与最后一次判定</summary>
                <p>保留插件池、协议和缓存等原始字段，供进一步排障。</p>
                <pre>{{ JSON.stringify(stats, null, 2) }}</pre>
              </details>
            </template>
          </section>
        </ElTabPane>
      </ElTabs>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElInput,
  ElMessage,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTabPane,
  ElTabs,
  ElTag,
} from 'element-plus';
import { ArrowLeft, Connection, Refresh } from '@element-plus/icons-vue';
import { pipelineGet } from '@/apis/pipeline';
import PageActions from '@/ui/PageActions.vue';
import ServicePunishments from './ServicePunishments.vue';
import DiagnosticEventDetail from './DiagnosticEventDetail.vue';
import {
  diagnosticAction,
  diagnosticMetricGroups,
  diagnosticModes,
  diagnosticNumber,
  diagnosticReason,
  diagnosticTime,
  filterDiagnosticEvents,
  metricValue,
  serviceReturnPath,
  type DiagnosticEvent,
} from './diagnostics';
const route = useRoute(),
  router = useRouter();
const port = computed(() => Number(route.params.port));
const validPort = computed(() => Number.isInteger(port.value) && port.value > 0 && port.value <= 65535);
const service = ref<any>(null),
  issue = ref(''),
  stats = ref<Record<string, any> | null>(null),
  logs = ref<DiagnosticEvent[] | null>(null);
const loading = reactive({ overview: false, stats: false, logs: false });
const errors = reactive({ overview: '', stats: '', logs: '' });
const updated = reactive({ overview: 0, stats: 0, logs: 0 });
const generations = { overview: 0, stats: 0, logs: 0 };
const busy = ref(false),
  punishments = ref<InstanceType<typeof ServicePunishments>>(),
  punishmentCount = ref<number | null>(null),
  serviceHeading = ref<HTMLElement>();
const refreshing = computed(() => Object.values(loading).some(Boolean));
const eventQuery = ref(''),
  eventKind = ref(''),
  eventPage = ref(1),
  pageSize = 20;
const eventsTable = ref<any>(),
  expandedEvents = ref<string[]>([]);
const activeTab = computed({
  get: () => (['state', 'metrics'].includes(String(route.query.tab)) ? String(route.query.tab) : 'events'),
  set: (tab: string) => {
    if (!busy.value) void router.replace({ query: { ...route.query, tab } });
  },
});
const dailyMetrics = [
  { key: 'today_requests', label: '今日请求' },
  { key: 'today_denied', label: '今日拦截' },
  { key: 'today_hits', label: '今日检测命中' },
];
const listenerLabel = computed(() =>
  service.value?.listener?.error
    ? '监听异常'
    : service.value?.listener?.running
      ? '正在监听'
      : service.value?.config.listen_enabled
        ? '已启用 · 未确认监听'
        : '未启用',
);
const listenerTone = computed(() =>
  service.value?.listener?.error ? 'danger' : service.value?.listener?.running ? 'success' : 'info',
);
const modes = computed(() => diagnosticModes(service.value?.config.policy || {}));
const defenseLabel = computed(
  () =>
    ({ quiet: '自动处置：无提示拒绝', restrict: '自动处置：仅历史成功网段可访问', close: '自动处置：访问已关闭' })[
      service.value?.config.defense_state?.mode
    ] || '异常访问处置已生效',
);
const filteredEvents = computed(() => filterDiagnosticEvents(logs.value || [], eventQuery.value, eventKind.value));
const pagedEvents = computed(() =>
  filteredEvents.value.slice((eventPage.value - 1) * pageSize, eventPage.value * pageSize),
);
const eventEmptyText = computed(() =>
  loading.logs && logs.value === null
    ? '正在读取事件…'
    : errors.logs && logs.value === null
      ? '事件读取失败，请重试。'
      : logs.value === null
        ? '事件尚未读取。'
        : logs.value.length
          ? '没有匹配的记录，请调整筛选条件。'
          : '最近记录中没有请求或状态变化。',
);
function clearFilters() {
  eventQuery.value = '';
  eventKind.value = '';
}
function eventKey(row: DiagnosticEvent) {
  return row.id || `${row.time_ms}-${row.req_id}-${row.kind}-${row.client_ip}`;
}
function eventTone(row: DiagnosticEvent) {
  return row.kind === 'state' || row.observe
    ? 'info'
    : ['deny', 'drop', 'ban'].includes(row.verdict || '')
      ? 'danger'
      : ['challenge', 'honeypot'].includes(row.verdict || '')
        ? 'warning'
        : 'info';
}
async function read(key: keyof typeof loading, endpoint: string, apply: (data: any) => void) {
  if (!validPort.value || loading[key]) return;
  const generation = ++generations[key],
    requestedPort = port.value;
  loading[key] = true;
  errors[key] = '';
  try {
    const data = await pipelineGet(endpoint, endpoint === 'overview' ? {} : { port: requestedPort });
    if (generation !== generations[key] || port.value !== requestedPort) return;
    apply(data);
    updated[key] = Date.now();
  } catch (error: any) {
    if (generation === generations[key]) errors[key] = error?.message || '请求失败，请重试。';
  } finally {
    if (generation === generations[key]) loading[key] = false;
  }
}
function loadOverview() {
  return read('overview', 'overview', (data) => {
    if (!Array.isArray(data?.ports)) throw new Error('服务状态响应不完整，请重试。');
    service.value = data.ports.find((row) => row.config?.port === port.value) || null;
    issue.value = data.issues?.[port.value] || '';
  });
}
function loadStats() {
  return read('stats', 'stats', (data) => {
    if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('指标响应不完整，请重试。');
    stats.value = data;
  });
}
function loadLogs() {
  return read('logs', 'logs', (data) => {
    if (!Array.isArray(data)) throw new Error('事件响应不完整，请重试。');
    logs.value = data;
    eventPage.value = Math.min(eventPage.value, Math.max(1, Math.ceil(filteredEvents.value.length / pageSize)));
  });
}
async function refreshAll() {
  if (busy.value || refreshing.value || !validPort.value) return;
  await Promise.allSettled([loadOverview(), loadStats(), loadLogs(), punishments.value?.refresh()]);
}
function punishmentChanged() {
  void Promise.allSettled([loadOverview(), loadStats(), loadLogs()]);
}
function goBack() {
  if (!busy.value) void router.push(serviceReturnPath(route.query.returnTo));
}
async function rememberContext() {
  const path = router.resolve({
    path: route.path,
    query: { ...route.query, q: eventQuery.value || undefined, kind: eventKind.value || undefined },
  }).fullPath;
  if (route.fullPath !== path) await router.replace(path);
  return path;
}
async function openLogs() {
  await rememberContext();
  void router.push({
    name: 'WafvisitLog',
    query: { host_code: service.value.config.host_code || `pipeline-${port.value}` },
  });
}
async function configure(tab?: string) {
  const returnTo = await rememberContext();
  void router.push({
    name: 'WafPipeline',
    query: { edit: port.value, ...(tab === 'defense' ? { tab } : {}), returnTo },
  });
}
function allowTabChange() {
  return !busy.value;
}
function allowNavigation() {
  if (!busy.value) return true;
  ElMessage.warning('处罚操作尚未完成，请先完成或取消确认。');
  return false;
}
onBeforeRouteLeave(allowNavigation);
onBeforeRouteUpdate((to, from) => to.params.port === from.params.port || allowNavigation());
watch([eventQuery, eventKind], () => {
  eventPage.value = 1;
});
watch(
  port,
  async () => {
    for (const key of ['overview', 'stats', 'logs'] as const) {
      generations[key]++;
      loading[key] = false;
      errors[key] = '';
      updated[key] = 0;
    }
    service.value = null;
    stats.value = null;
    logs.value = null;
    issue.value = '';
    punishmentCount.value = null;
    eventQuery.value = typeof route.query.q === 'string' ? route.query.q : '';
    eventKind.value = ['request', 'state'].includes(String(route.query.kind)) ? String(route.query.kind) : '';
    expandedEvents.value = [];
    eventPage.value = 1;
    await refreshAll();
    await nextTick();
    serviceHeading.value?.focus({ preventScroll: true });
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  for (const key of ['overview', 'stats', 'logs'] as const) generations[key]++;
});
</script>
<style scoped>
.service-diagnostics {
  display: grid;
  gap: 16px;
  min-width: 0;
  color: var(--hw-ink);
}
.action-icon {
  width: 16px;
  height: 16px;
  margin-right: 6px;
}
.diagnostic-service,
.diagnostic-panel {
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  padding: 24px;
  min-width: 0;
}
.diagnostic-service {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 24px;
}
.service-identity {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  min-width: 0;
}
.service-identity > div {
  min-width: 0;
}
.service-symbol {
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  color: var(--hw-accent);
  background: var(--hw-accent-soft);
  border-radius: var(--hw-r-md);
}
.service-symbol svg {
  width: 22px;
  height: 22px;
}
h2,
h3,
p,
dl {
  margin: 0;
}
h2 {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
h3 {
  font-size: 15px;
  font-weight: 600;
}
.service-identity p {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 20px;
  margin-top: 5px;
  color: var(--hw-muted);
  font-size: 13px;
}
.upstream {
  overflow-wrap: anywhere;
}
.service-actions {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  flex-wrap: wrap;
}
.service-actions .el-button {
  margin: 0;
}
.service-facts {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px 24px;
  padding-top: 20px;
  border-top: 1px solid var(--hw-line);
}
dt {
  color: var(--hw-muted);
  font-size: 13px;
}
dd {
  margin: 8px 0 0;
  overflow-wrap: anywhere;
  font-variant-numeric: tabular-nums;
}
.service-facts dd {
  font-size: 14px;
}
.diagnostic-notices {
  display: grid;
  gap: 10px;
}
.diagnostic-guidance {
  font-size: 13px;
  color: var(--hw-muted);
  line-height: 1.7;
}
.event-cards {
  display: none;
}
.events-table :deep(.trace-expander .cell) {
  padding: 0;
}
.events-table :deep(.trace-expander .el-table__expand-icon) {
  display: none;
}
.defense-notice {
  padding: 16px;
  border: 1px solid var(--hw-line);
  border-left: 3px solid var(--hw-warning);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: var(--hw-panel);
}
.defense-notice p {
  font-size: 13px;
  color: var(--hw-muted);
  margin-top: 8px;
}
.diagnostic-totals {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  background: var(--hw-panel);
}
.diagnostic-totals > div {
  padding: 20px 24px;
}
.diagnostic-totals > div + div {
  border-left: 1px solid var(--hw-line);
}
.diagnostic-totals dd {
  font-size: clamp(20px, 2vw, 28px);
  font-weight: 600;
}
.diagnostic-totals small {
  font-size: 13px;
  font-weight: 400;
  margin-left: 8px;
  color: var(--hw-muted);
}
.snapshot-note {
  font-size: 12px;
  line-height: 1.7;
  color: var(--hw-muted);
}
.diagnostic-tabs {
  min-width: 0;
}
.diagnostic-tabs :deep(.el-tabs__item) {
  min-height: 44px;
}
.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.section-heading p,
.metric-group > p {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
  margin-top: 6px;
}
.event-filters {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 12px 16px;
  margin: 20px 0;
}
.event-filters label {
  display: grid;
  gap: 8px;
  font-size: 13px;
  width: min(100%, 320px);
}
.event-filters label:nth-child(2) {
  width: 160px;
}
.event-filters > span {
  font-size: 13px;
  color: var(--hw-muted);
  padding-bottom: 10px;
}
.events-table small {
  display: block;
  color: var(--hw-muted);
  font-size: 12px;
  margin-top: 4px;
}
.event-reason,
code {
  overflow-wrap: anywhere;
}
.events-table :deep(.el-table__expand-icon) {
  width: 32px;
  height: 44px;
}
.event-footer {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  font-size: 12px;
  color: var(--hw-muted);
}
.diagnostic-empty {
  padding: 32px 20px;
  text-align: center;
  background: var(--hw-subtle);
  border-radius: var(--hw-r-md);
  line-height: 1.8;
}
.diagnostic-empty p {
  color: var(--hw-muted);
  margin: 8px 0 16px;
  overflow-wrap: anywhere;
}
.metric-group + .metric-group {
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--hw-line);
}
.metric-group dl {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 32px;
  margin-top: 12px;
}
.metric-group dl > div {
  padding: 14px 0;
  border-bottom: 1px solid var(--hw-line);
}
.metric-group dd {
  font-size: 20px;
}
.raw-metrics {
  margin-top: 24px;
  border-top: 1px solid var(--hw-line);
}
.raw-metrics summary {
  cursor: pointer;
  padding: 16px 0;
  font-size: 14px;
}
.raw-metrics p {
  font-size: 13px;
  color: var(--hw-muted);
}
.raw-metrics pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: 12px/1.7 var(--font-mono);
  max-height: 600px;
  overflow: auto;
}
@media (max-width: 1000px) {
  .service-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .metric-group dl {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 1000px) {
  .events-table {
    display: none;
  }
  .event-cards {
    display: grid;
  }
  .event-cards article {
    display: grid;
    gap: 12px;
    padding: 20px 0;
    border-top: 1px solid var(--hw-line);
    min-width: 0;
  }
  .event-cards header {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 12px;
    justify-content: space-between;
    font-size: 12px;
    color: var(--hw-muted);
  }
  .event-cards .el-tag {
    justify-self: start;
    height: auto;
    white-space: normal;
    padding: 4px 8px;
  }
  .event-cards p {
    overflow-wrap: anywhere;
    line-height: 1.7;
    font-size: 14px;
  }
  .event-cards summary {
    cursor: pointer;
    color: var(--hw-accent);
    min-height: 44px;
    display: flex;
    align-items: center;
  }
  .event-cards details {
    min-width: 0;
  }
  .event-cards :deep(.event-detail) {
    padding: 12px 0;
  }
  .diagnostic-service,
  .diagnostic-panel {
    padding: 16px;
  }
  .diagnostic-service {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .service-symbol {
    display: none;
  }
  .service-actions .el-button {
    flex: 1;
  }
  .service-facts {
    gap: 20px 12px;
  }
  .diagnostic-totals > div {
    padding: 16px 10px;
  }
  .diagnostic-totals dd {
    font-size: 20px;
  }
  .section-heading,
  .defense-notice {
    flex-direction: column;
  }
  .event-filters label,
  .event-filters label:nth-child(2) {
    width: 100%;
  }
  .metric-group dl {
    column-gap: 16px;
  }
  .service-diagnostics :deep(.el-button) {
    min-height: 44px;
  }
}
</style>
