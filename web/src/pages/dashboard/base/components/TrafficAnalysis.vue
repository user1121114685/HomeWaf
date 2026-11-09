<template>
  <section
    class="traffic-analysis"
    :data-theme="dark ? 'dark' : 'light'"
    :aria-label="text('流量分析', 'Traffic analytics')"
    :aria-busy="loading"
  >
    <header class="traffic-toolbar">
      <nav class="traffic-navigation" :aria-label="text('仪表盘导航', 'Dashboard navigation')">
        <span class="traffic-tab active" aria-current="page">{{ text('流量分析', 'Traffic analytics') }}</span>
        <router-link class="traffic-tab" to="/waf/wafvisitlog">{{ text('访问日志', 'Access logs') }}</router-link>
        <router-link class="traffic-tab" to="/dashboard/screen" target="_blank" rel="noopener"
          >{{ text('监控大屏', 'Security screen') }} <span>↗</span></router-link
        >
      </nav>
      <div class="traffic-filters">
        <span class="scope-label">{{ text('全部站点', 'All sites') }}</span>
        <select v-model="preset" :aria-label="text('统计时间范围', 'Statistics period')" @change="changePreset">
          <option value="today">{{ text('今天', 'Today') }}</option>
          <option value="yesterday">{{ text('昨天', 'Yesterday') }}</option>
          <option value="7d">{{ text('近 7 天', 'Last 7 days') }}</option>
          <option value="30d">{{ text('近 30 天', 'Last 30 days') }}</option>
          <option value="custom">{{ text('自定义时间', 'Custom dates') }}</option>
        </select>
        <button
          class="refresh-button"
          type="button"
          :disabled="loading"
          :aria-label="text('刷新流量分析', 'Refresh traffic analytics')"
          @click="refreshAll"
        >
          <refresh-icon :class="{ spinning: loading }" />
        </button>
      </div>
    </header>
    <form v-if="preset === 'custom'" class="custom-period" @submit.prevent="applyCustom">
      <label
        >{{ text('开始日期', 'Start date') }}<input v-model="draftStart" type="date" required :max="draftEnd || today"
      /></label>
      <span>—</span>
      <label
        >{{ text('结束日期', 'End date')
        }}<input v-model="draftEnd" type="date" required :min="draftStart" :max="today"
      /></label>
      <button class="accent-button" type="submit">{{ text('应用', 'Apply') }}</button>
      <span v-if="dateError" role="alert">{{ dateError }}</span>
    </form>
    <div class="traffic-status" role="status">
      <span><i :class="{ warning: errors.length > 0 }"></i>{{ range[0] }} — {{ range[1] }}</span>
      <span v-if="errors.length" class="load-warning">{{
        text(
          '部分统计更新失败，已显示的旧数据仅供参考，请刷新重试。',
          'Some statistics could not update. Previous values may be stale; please retry.',
        )
      }}</span>
      <span class="updated-at">{{
        loading
          ? text('正在更新…', 'Updating…')
          : updatedAt
            ? text('更新于 ', 'Updated ') + updatedAt
            : text('等待数据', 'Waiting for data')
      }}</span>
    </div>

    <div class="traffic-main-grid">
      <div class="traffic-metrics">
        <div class="metric-group primary-metrics">
          <div v-for="metric in mainMetrics" :key="metric.label" class="traffic-metric" :title="metric.tip">
            <div class="metric-label">{{ metric.label }} <span v-if="metric.tip" class="metric-help">ⓘ</span></div>
            <button
              v-if="metric.action !== undefined"
              class="metric-number metric-link"
              type="button"
              @click="openLogs(metric.action)"
            >
              {{ metric.value }}
            </button>
            <strong v-else class="metric-number">{{ metric.value }}</strong>
          </div>
        </div>
        <div class="secondary-metrics">
          <div v-for="(group, index) in secondaryMetrics" :key="index" class="metric-group">
            <div v-for="metric in group" :key="metric.label" class="traffic-metric" :title="metric.tip">
              <div class="metric-label">{{ metric.label }}</div>
              <strong class="metric-number">{{ metric.value }}</strong>
            </div>
          </div>
        </div>
      </div>

      <article class="traffic-panel qps-panel">
        <div class="panel-heading">
          <h2>{{ text('实时 QPS', 'Live QPS') }}</h2>
          <strong class="qps-value">{{ qps ? count(qps.Current) : '—' }}</strong
          ><span class="panel-meta">{{ text('全局 · 次/秒', 'Global · req/s') }}</span>
        </div>
        <div class="qps-chart">
          <traffic-chart :option="qpsOption" :label="text('最近 60 秒实时 QPS', 'QPS over the last 60 seconds')" />
        </div>
        <div v-if="qpsFailed" class="chart-note warning">
          {{ text('更新失败，等待重试', 'Update failed; retrying') }}
        </div>
      </article>

      <article class="traffic-panel source-regions-panel">
        <div class="panel-heading source-regions-heading">
          <h2>{{ text('来源地区', 'Source regions') }}</h2>
          <div class="segmented" :aria-label="text('来源地区统计类型', 'Source region traffic type')">
            <button
              type="button"
              :class="{ active: !blockedOnly }"
              :aria-pressed="!blockedOnly"
              @click="blockedOnly = false"
            >
              {{ text('访问', 'Requests') }}
            </button>
            <button
              type="button"
              :class="{ active: blockedOnly }"
              :aria-pressed="blockedOnly"
              @click="blockedOnly = true"
            >
              {{ text('仅拦截', 'Blocked') }}
            </button>
          </div>
        </div>
        <div class="region-ranking">
          <div class="ranking-caption">
            <span>{{ text('国家 / 地区', 'Country / region') }}</span>
            <span>{{ blockedOnly ? text('拦截次数', 'Blocked') : text('请求次数', 'Requests') }}</span>
          </div>
          <div v-for="row in regions.slice(0, 7)" :key="row.name" class="region-row">
            <div>
              <span :title="row.name">{{ row.name }}</span
              ><strong>{{ count(row.value) }}</strong>
            </div>
            <div class="rank-track"><i :style="{ width: percent(row.value, regions[0]?.value) }"></i></div>
          </div>
          <div v-if="!regions.length" class="panel-empty">
            {{
              regionUnavailable
                ? text('地区数据暂不可用', 'Region data unavailable')
                : text('暂无地区数据', 'No regional traffic')
            }}
          </div>
        </div>
        <p class="source-regions-note">
          {{
            text(
              '按所选时间的请求次数降序展示前 7 个来源地区，内网流量不计入统计。',
              'Top 7 source regions by request count in the selected period. Internal traffic is excluded.',
            )
          }}
        </p>
      </article>

      <div class="traffic-trends">
        <article v-for="chart in trendCharts" :key="chart.key" class="traffic-panel trend-panel">
          <div class="panel-heading">
            <h2>{{ chart.title }}</h2>
            <span class="panel-meta"
              >{{ text('峰值', 'Peak') }}
              <strong>{{ data.trend ? count(Math.max(0, ...chart.values)) : '—' }}</strong></span
            >
          </div>
          <div class="trend-chart">
            <traffic-chart :option="lineOption(chart.values, chart.color, chart.title)" :label="chart.title" />
          </div>
          <div v-if="!data.trend" class="chart-note">{{ text('等待统计数据', 'Waiting for statistics') }}</div>
        </article>
      </div>
    </div>

    <div class="traffic-breakdowns">
      <article class="traffic-panel distribution-panel">
        <div class="panel-heading">
          <h2>{{ text('请求分布', 'Request distribution') }}</h2>
          <button class="text-button" type="button" @click="openLogs('')">{{ text('查看日志', 'View logs') }} ↗</button>
        </div>
        <div class="distribution-body">
          <div class="donut-chart">
            <traffic-chart
              :option="donutOption(requestSlices)"
              :label="text('放行与拦截请求占比', 'Allowed and blocked requests')"
            />
            <div class="donut-center">
              <strong>{{ data.trend ? rate(totals.blocked, totals.requests) : '—' }}</strong
              ><span>{{ text('拦截占比', 'Block rate') }}</span>
            </div>
          </div>
          <div class="distribution-list">
            <div v-for="item in requestSlices" :key="item.name" class="distribution-row">
              <span><i :style="{ background: item.color }"></i>{{ item.name }}</span
              ><strong>{{ data.trend ? count(item.value) : '—' }}</strong
              ><small>{{ data.trend ? rate(item.value, totals.requests) : '—' }}</small>
            </div>
            <p>
              {{
                text(
                  '请求次数包含放行与拦截，按所选日期统计。',
                  'Requests include allowed and blocked traffic in the selected period.',
                )
              }}
            </p>
          </div>
        </div>
      </article>
      <article class="traffic-panel distribution-panel">
        <div class="panel-heading">
          <h2>{{ text('流量统计', 'Traffic volume') }}</h2>
          <router-link class="text-button" to="/dashboard/overview"
            >{{ text('站点明细', 'Site details') }} ↗</router-link
          >
        </div>
        <div class="distribution-body">
          <div class="donut-chart">
            <traffic-chart
              :option="donutOption(trafficSlices)"
              :label="text('入站与出站流量占比', 'Inbound and outbound traffic')"
            />
            <div class="donut-center">
              <strong>{{
                data.overview ? traffic((data.overview.total_in_mb || 0) + (data.overview.total_out_mb || 0)) : '—'
              }}</strong
              ><span>{{ text('总流量', 'Total traffic') }}</span>
            </div>
          </div>
          <div class="distribution-list">
            <div v-for="item in trafficSlices" :key="item.name" class="distribution-row">
              <span><i :style="{ background: item.color }"></i>{{ item.name }}</span
              ><strong>{{ data.overview ? traffic(item.value) : '—' }}</strong>
            </div>
            <p>
              {{ text('由各站点入站与出站字节数汇总。', 'Aggregated from inbound and outbound bytes across sites.') }}
            </p>
          </div>
        </div>
      </article>

      <article v-for="rank in ipRankings" :key="rank.key" class="traffic-panel ip-panel">
        <div class="panel-heading">
          <h2>{{ rank.title }}</h2>
          <button class="text-button" type="button" @click="openLogs(rank.action)">
            {{ text('查看更多', 'View more') }} ↗
          </button>
        </div>
        <div class="ip-list">
          <div v-for="(row, index) in rank.rows.slice(0, 7)" :key="row.ip" class="ip-row">
            <span class="rank-index">{{ String(index + 1).padStart(2, '0') }}</span>
            <div class="ip-description">
              <button class="ip-address" type="button" @click="lookupDialog?.open(row.ip)">{{ row.ip }}</button
              ><span class="ip-location">{{ row.ip_belong || text('归属地未知', 'Unknown location') }}</span>
            </div>
            <ip-tag-cell
              v-if="row.ip_tags?.length || rank.key === 'normal'"
              :ip="row.ip"
              :tags="row.ip_tags || []"
              :count="row.count"
              @view-log="openLogs('', row.ip)"
            />
            <span v-else class="blocked-chip">{{ text('已拦截', 'Blocked') }}</span>
            <strong>{{ count(row.count) }}</strong>
            <button
              class="log-button"
              type="button"
              :aria-label="text('查看 ', 'View logs for ') + row.ip + text(' 的日志', '')"
              @click="openLogs(rank.action, row.ip)"
            >
              ↗
            </button>
          </div>
          <div v-if="!rank.rows.length" class="panel-empty">
            {{
              data.ranks
                ? text('所选时间暂无记录', 'No records in this period')
                : text('排行数据暂不可用', 'Ranking data unavailable')
            }}
          </div>
        </div>
      </article>
    </div>
    <footer class="traffic-footer">
      <span>{{
        text(
          '站点 UV / IP 为各站点去重后相加，跨站点可能重复。',
          'UV / IP totals are deduplicated per site; visitors across sites may be counted more than once.',
        )
      }}</span
      ><span>{{ text('统计每 30 秒刷新 · QPS 每 5 秒刷新', 'Statistics refresh every 30s · QPS every 5s') }}</span>
    </footer>
    <ip-lookup ref="lookupDialog" hide-trigger />
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import dayjs from 'dayjs';
import store from '@/store';
import { RefreshIcon } from '@/ui/icons';
import TrafficChart from './TrafficChart.vue';
import IpTagCell from './IpTagCell.vue';
import { useTrafficData } from './useTrafficData';
import countryLocations from '../../screen/country-locations.json';

const router = useRouter();
const dark = computed(() => store.getters['setting/mode'] === 'dark');
const palette = computed(() =>
  dark.value
    ? {
        visits: '#85c7eb',
        allowed: '#89bdd7',
        blocked: '#ef9471',
        inbound: '#edc5a7',
        outbound: '#a49bc4',
        axis: '#918f8b',
        grid: '#363534',
        tooltip: '#343230',
        border: '#51463e',
        text: '#eee5de',
        empty: '#3b3835',
      }
    : {
        visits: '#3485b4',
        allowed: '#3889b5',
        blocked: '#c66c42',
        inbound: '#b27b4e',
        outbound: '#8875ac',
        axis: '#687482',
        grid: '#e4e8ee',
        tooltip: '#ffffff',
        border: '#dfe4eb',
        text: '#263342',
        empty: '#e4e8ee',
      },
);
const english = computed(() => store.state.language.currentLanguage === 'en_US');
const text = (zh: string, en: string) => (english.value ? en : zh);
const today = dayjs().format('YYYY-MM-DD');
const preset = ref('7d'),
  range = ref<[string, string]>([dayjs().subtract(6, 'day').format('YYYY-MM-DD'), today]);
const draftStart = ref(range.value[0]),
  draftEnd = ref(today),
  dateError = ref('');
const blockedOnly = ref(false);
const lookupDialog = ref<{ open: (ip: string) => void }>();
const { data, errors, loading, updatedAt, qps, qpsFailed, refresh, refreshQps } = useTrafficData(() => range.value);
function changePreset() {
  dateError.value = '';
  if (preset.value === 'custom') {
    draftStart.value = range.value[0];
    draftEnd.value = range.value[1];
    return;
  }
  const now = dayjs();
  range.value =
    preset.value === 'yesterday'
      ? [now.subtract(1, 'day').format('YYYY-MM-DD'), now.subtract(1, 'day').format('YYYY-MM-DD')]
      : [
          now.subtract(preset.value === '7d' ? 6 : preset.value === '30d' ? 29 : 0, 'day').format('YYYY-MM-DD'),
          now.format('YYYY-MM-DD'),
        ];
  void refresh(true);
}
function applyCustom() {
  if (
    !draftStart.value ||
    !draftEnd.value ||
    draftStart.value > draftEnd.value ||
    draftEnd.value > dayjs().format('YYYY-MM-DD') ||
    dayjs(draftEnd.value).diff(dayjs(draftStart.value), 'day') > 365
  ) {
    dateError.value = text('请选择有效日期范围，最长 366 天。', 'Choose a valid date range of up to 366 days.');
    return;
  }
  dateError.value = '';
  range.value = [draftStart.value, draftEnd.value];
  void refresh(true);
}
function refreshAll() {
  void refresh();
  void refreshQps();
}
function openLogs(action: string, ip = '') {
  void router.push({
    path: '/waf/wafvisitlog',
    query: { action, src_ip: ip, date_begin: range.value[0] + ' 00:00:00', date_end: range.value[1] + ' 23:59:59' },
  });
}
function count(value: number) {
  return Number.isFinite(value)
    ? value.toLocaleString(english.value ? 'en-US' : 'zh-CN', { maximumFractionDigits: 1 })
    : '—';
}
function rate(value: number, total: number) {
  return total > 0 ? ((value / total) * 100).toFixed(2) + '%' : '—';
}
function percent(value: number, max: number) {
  return `${max > 0 ? Math.max(0, Math.min(100, (value / max) * 100)) : 0}%`;
}
function traffic(mb: number) {
  if (!Number.isFinite(mb)) return '—';
  return mb >= 1024 ? (mb / 1024).toFixed(2) + ' GB' : mb >= 1 ? mb.toFixed(2) + ' MB' : (mb * 1024).toFixed(1) + ' KB';
}
const days = computed(() => {
  const size = dayjs(range.value[1]).diff(dayjs(range.value[0]), 'day') + 1;
  return Array.from({ length: Math.max(1, Math.min(366, size)) }, (_, i) =>
    dayjs(range.value[0]).add(i, 'day').format('YYYYMMDD'),
  );
});
const values = computed(() => {
  const allowed = days.value.map((day) => Number(data.trend?.NormalCountOfRange[day]) || 0);
  const blocked = days.value.map((day) => Number(data.trend?.AttackCountOfRange[day]) || 0);
  return { allowed, blocked, requests: allowed.map((n, i) => n + blocked[i]) };
});
const totals = computed(() => ({
  allowed: values.value.allowed.reduce((a, b) => a + b, 0),
  blocked: values.value.blocked.reduce((a, b) => a + b, 0),
  requests: values.value.requests.reduce((a, b) => a + b, 0),
}));
const siteTip = computed(() =>
  text(
    '各站点内按 IP 去重后求和，跨站点可能重复；UV 以 IP 为统计依据。',
    'Sum of unique IPs per site; cross-site visitors may be repeated. UV is IP-based.',
  ),
);
const mainMetrics = computed(() => [
  {
    label: text('请求次数（PV）', 'Requests (PV)'),
    value: data.trend ? count(totals.value.requests) : '—',
    action: '',
    tip: text(
      '放行与拦截请求总数，不等同于浏览器页面浏览量。',
      'All allowed and blocked HTTP requests; not browser page views.',
    ),
  },
  {
    label: text('放行次数', 'Allowed requests'),
    value: data.trend ? count(totals.value.allowed) : '—',
    action: '放行',
  },
  {
    label: text('站点访客（UV）', 'Site visitors (UV)'),
    value: data.overview ? count(data.overview.total_uv) : '—',
    tip: siteTip.value,
  },
  {
    label: text('站点 IP 总数', 'Site IP total'),
    value: data.overview ? count(data.overview.total_ip) : '—',
    tip: siteTip.value,
  },
  {
    label: text('拦截次数', 'Blocked requests'),
    value: data.trend ? count(totals.value.blocked) : '—',
    action: '阻止',
  },
  {
    label: text('拦截占比', 'Block rate'),
    value: data.trend ? rate(totals.value.blocked, totals.value.requests) : '—',
  },
]);
const averageTime = computed(() => {
  const sites = data.overview?.site_list || [],
    total = sites.reduce((n, s) => n + s.total_count, 0);
  return total ? (sites.reduce((n, s) => n + s.avg_time_ms * s.total_count, 0) / total).toFixed(1) + ' ms' : '—';
});
const secondaryMetrics = computed(() => [
  [
    {
      label: text('入站流量', 'Inbound traffic'),
      value: data.overview ? traffic(data.overview.total_in_mb) : '—',
      tip: '',
    },
    {
      label: text('出站流量', 'Outbound traffic'),
      value: data.overview ? traffic(data.overview.total_out_mb) : '—',
      tip: '',
    },
  ],
  [
    {
      label: text('活跃站点', 'Active sites'),
      value: data.overview ? count(data.overview.site_list.filter((s) => s.total_count > 0).length) : '—',
      tip: '',
    },
    {
      label: text('平均响应', 'Avg. response'),
      value: averageTime.value,
      tip: text('按站点请求次数加权平均', 'Weighted by request count per site'),
    },
  ],
  [
    {
      label: text('日均请求', 'Daily avg. requests'),
      value: data.trend ? count(Math.round(totals.value.requests / days.value.length)) : '—',
      tip: '',
    },
    {
      label: text('放行占比', 'Allow rate'),
      value: data.trend ? rate(totals.value.allowed, totals.value.requests) : '—',
      tip: '',
    },
  ],
]);
const regionUnavailable = computed(() => (blockedOnly.value ? data.blockedRegions === null : data.regions === null));
const regions = computed(() => {
  const source = (blockedOnly.value ? data.blockedRegions : data.regions) || [];
  const aliases: Record<string, string> = countryLocations.aliases;
  const countries: Record<string, { name: string }> = countryLocations.countries;
  const totals = new Map<string, number>();
  // Normalize country aliases, but keep sources even when no map geometry exists.
  for (const row of source) {
    const rawName = row.name.trim();
    if (!rawName || rawName === '0' || rawName === '内网' || row.value <= 0) continue;
    const name = countries[aliases[rawName.toLowerCase()]]?.name || rawName;
    totals.set(name, (totals.get(name) || 0) + row.value);
  }
  return [...totals].map(([name, value]) => ({ name, value })).sort((a, b) => b.value - a.value);
});
const trendCharts = computed(() => [
  {
    key: 'visits',
    title: text('访问情况', 'Request trend'),
    values: values.value.requests,
    color: palette.value.visits,
  },
  {
    key: 'blocked',
    title: text('拦截情况', 'Blocked trend'),
    values: values.value.blocked,
    color: palette.value.blocked,
  },
]);
const requestSlices = computed(() => [
  { name: text('放行请求', 'Allowed'), value: totals.value.allowed, color: palette.value.allowed },
  { name: text('拦截请求', 'Blocked'), value: totals.value.blocked, color: palette.value.blocked },
]);
const trafficSlices = computed(() => [
  { name: text('入站流量', 'Inbound'), value: data.overview?.total_in_mb || 0, color: palette.value.inbound },
  { name: text('出站流量', 'Outbound'), value: data.overview?.total_out_mb || 0, color: palette.value.outbound },
]);
const ipRankings = computed(() => [
  {
    key: 'attack',
    title: text('攻击来源 IP', 'Attack source IPs'),
    action: '阻止',
    rows: data.ranks?.AttackIPOfRange || [],
  },
  {
    key: 'normal',
    title: text('正常访问 IP', 'Allowed source IPs'),
    action: '放行',
    rows: data.ranks?.NormalIPOfRange || [],
  },
]);
const tooltip = computed(() => ({
  trigger: 'axis',
  renderMode: 'richText',
  confine: true,
  backgroundColor: palette.value.tooltip,
  borderColor: palette.value.border,
  textStyle: { color: palette.value.text, fontSize: 12 },
}));
function lineOption(series: number[], color: string, name: string) {
  return {
    animation: false,
    tooltip: tooltip.value,
    grid: { top: 14, bottom: 24, left: 38, right: 10 },
    xAxis: {
      type: 'category',
      data: days.value.map((d) => d.slice(4, 6) + '-' + d.slice(6)),
      boundaryGap: series.length === 1,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: palette.value.axis, fontSize: 10, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      min: 0,
      minInterval: 1,
      splitNumber: 3,
      axisLabel: {
        color: palette.value.axis,
        fontSize: 10,
        formatter: (n: number) => (n >= 1000 ? n / 1000 + 'k' : n),
      },
      splitLine: { lineStyle: { color: palette.value.grid } },
    },
    series: [
      {
        name,
        type: 'line',
        smooth: 0.25,
        showSymbol: series.length === 1,
        symbolSize: 6,
        data: data.trend ? series : [],
        lineStyle: { color, width: 2.5 },
        itemStyle: { color },
        areaStyle: { color, opacity: 0.035 },
      },
    ],
  };
}
const qpsOption = computed(() => ({
  animation: false,
  tooltip: tooltip.value,
  grid: { top: 8, bottom: 18, left: 8, right: 8 },
  xAxis: {
    type: 'category',
    data: (qps.value?.Points || []).map((p) => dayjs.unix(p.T).format('HH:mm:ss')),
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: { color: palette.value.axis, fontSize: 9, interval: 19, hideOverlap: true },
  },
  yAxis: { type: 'value', min: 0, minInterval: 1, show: false },
  series: [
    {
      name: 'QPS',
      type: 'bar',
      barMaxWidth: 7,
      data: (qps.value?.Points || []).map((p) => p.V),
      itemStyle: { color: palette.value.inbound, borderRadius: [2, 2, 0, 0] },
    },
  ],
}));
function donutOption(items: { name: string; value: number; color: string }[]) {
  const total = items.reduce((sum, item) => sum + item.value, 0);
  return {
    animation: false,
    tooltip: { ...tooltip.value, trigger: 'item' },
    series: [
      {
        type: 'pie',
        radius: ['72%', '88%'],
        center: ['50%', '50%'],
        label: { show: false },
        emphasis: { scaleSize: 3 },
        data:
          total > 0
            ? items.map((item) => ({ ...item, itemStyle: { color: item.color } }))
            : [
                {
                  value: 1,
                  name: text('暂无数据', 'No data'),
                  itemStyle: { color: palette.value.empty },
                  tooltip: { show: false },
                },
              ],
      },
    ],
  };
}
</script>
<style scoped src="./traffic-analysis.css"></style>
