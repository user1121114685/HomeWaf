<template>
  <main ref="screen" class="security-screen" :class="{ 'is-dark': dark }">
    <header class="screen-header">
      <div class="screen-brand">
        <div class="brand-mark" aria-hidden="true">
          <svg viewBox="0 0 32 36">
            <path d="M16 2 29 7v10c0 8-8 14-13 17C11 31 3 25 3 17V7Z" />
            <path d="m10 17 4 4 9-10" />
          </svg>
        </div>
        <div>
          <div class="brand-eyebrow">HOMEWAF <span>SECURITY MONITOR</span></div>
          <h1>Web 安全防护监控大屏</h1>
        </div>
      </div>
      <div class="header-actions">
        <div class="screen-clock">
          <span>{{ dateText }}</span
          ><strong>{{ timeText }}</strong>
        </div>
        <div class="header-divider"></div>
        <button
          class="icon-button"
          :title="dark ? '切换浅色主题' : '切换深色主题'"
          :aria-label="dark ? '切换浅色主题' : '切换深色主题'"
          @click="toggleTheme"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.8 13A9 9 0 0 1 11 3.2 9 9 0 1 0 20.8 13Z" /></svg>
        </button>
        <button
          class="icon-button"
          :title="paused ? '恢复自动刷新' : '暂停自动刷新'"
          :aria-label="paused ? '恢复自动刷新' : '暂停自动刷新'"
          @click="togglePause"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path v-if="paused" d="m8 5 11 7-11 7Z" />
            <path v-else d="M8 5v14M16 5v14" />
          </svg>
        </button>
        <button class="screen-button" :disabled="busy" @click="refresh">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20 7v5h-5M4 17v-5h5M6 6a8 8 0 0 1 13 3M18 18A8 8 0 0 1 5 15" /></svg
          ><span>刷新</span>
        </button>
        <button class="screen-button primary" @click="toggleFullscreen">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5" /></svg
          ><span>{{ fullscreen ? '退出全屏' : '全屏' }}</span>
        </button>
        <ScreenControls v-if="!sharedView" :target="snapshot?.target || null" @updated="refresh" />
        <span v-else class="share-view-badge">只读分享 · 已脱敏</span>
        <router-link v-if="!sharedView" class="console-link" to="/dashboard/base"
          >返回控制台 <span aria-hidden="true">↗</span></router-link
        >
      </div>
    </header>

    <div class="screen-context">
      <div class="connection" :class="{ warning: connectionWarning }" role="status">
        <i></i>{{ connectionLabel }}<span class="context-separator">/</span
        ><span :title="windowLabel">近 24 小时 · 全部站点 · 服务端时间窗口</span>
      </div>
    </div>
    <div v-if="fullscreenError" class="screen-notice" role="alert">{{ fullscreenError }}</div>
    <div v-if="sharedError" class="screen-notice" role="alert">{{ sharedError }}</div>
    <div v-if="snapshot?.partial" class="screen-notice" role="status">
      统计不完整：{{ [...new Set(snapshot.issues)].join('；') }}
    </div>

    <div class="screen-main">
      <aside class="left-column" aria-label="近24小时指标和攻击排名">
        <div class="metric-stack">
          <article v-for="metric in metrics" :key="metric.label" class="metric" :class="metric.color">
            <div class="metric-symbol" aria-hidden="true">
              <svg viewBox="0 0 32 32"><path :d="metric.path" /></svg>
            </div>
            <div class="metric-copy">
              <div class="metric-label">{{ metric.label }}</div>
              <strong>{{ formatCount(metric.value) }}</strong
              ><span>{{ metric.note }}</span>
            </div>
          </article>
          <div v-if="status('summary')" class="source-status" :class="{ warning: sources.summary.failed }">
            {{ status('summary') }}
          </div>
        </div>
        <section class="screen-panel ranking-panel">
          <div class="panel-heading">
            <h2>攻击 IP 排名</h2>
            <span>24 小时 TOP 5</span>
          </div>
          <div v-if="status('ips')" class="source-status" :class="{ warning: sources.ips.failed }">
            {{ status('ips') }}
          </div>
          <ol v-if="rankedIPs.length" class="ip-ranking">
            <li v-for="(item, index) in rankedIPs" :key="`${index}-${item.ip}`">
              <span class="rank-number" :class="{ 'top-three': index < 3 }">{{
                String(index + 1).padStart(2, '0')
              }}</span>
              <div class="rank-info">
                <div>
                  <span class="mono" :title="item.ip">{{ item.ip }}</span
                  ><strong>{{ formatCount(item.count) }}</strong>
                </div>
                <div class="rank-track">
                  <span :style="{ width: `${(item.count / Math.max(rankedIPs[0].count, 1)) * 100}%` }"></span>
                </div>
              </div>
            </li>
          </ol>
          <div v-else class="panel-empty">
            <span class="empty-radar" aria-hidden="true">◎</span
            >{{ sources.ips.updatedAt ? '近 24 小时暂无攻击 IP' : status('ips') }}
          </div>
          <div class="panel-foot">按拦截次数排序 <span>单位：次</span></div>
        </section>
      </aside>

      <section class="map-column" aria-labelledby="map-title">
        <div class="map-heading">
          <div>
            <div class="section-eyebrow">THREAT ORIGINS</div>
            <h2 id="map-title">全球攻击来源分布</h2>
          </div>
          <span class="map-key"><i></i>拦截来源</span>
        </div>
        <div v-if="status('regions')" class="source-status map-status" :class="{ warning: sources.regions.failed }">
          {{ status('regions') }}
        </div>
        <WorldMap
          :dark="dark"
          :regions="regions"
          :events="events"
          :target="snapshot?.target || null"
          :paused="paused"
          :server-time="snapshot?.window_end || 0"
          :status="status('regions')"
        />
        <div class="map-summary">
          <div>
            <span>24 小时来源国家／地区</span
            ><strong
              >{{ sources.regions.updatedAt ? regions.filter((r) => r.value > 0).length : '—'
              }}<small>个</small></strong
            >
          </div>
          <div>
            <span>24 小时请求拦截占比</span><strong>{{ blockRate(sources.summary.data) }}</strong>
          </div>
          <div class="unblocked-summary" title="同一统计窗口内，已记录访问次数减去拦截次数">
            <span>24 小时未拦截请求</span><strong>{{ formatCount(unblockedRequests) }}<small>次</small></strong>
          </div>
        </div>
      </section>

      <section class="screen-panel events-panel" aria-labelledby="events-title">
        <div class="panel-heading">
          <h2 id="events-title">最近 Web 攻击</h2>
          <span class="live-label"><i></i>5 秒刷新</span>
        </div>
        <div v-if="status('events')" class="source-status" :class="{ warning: sources.events.failed }">
          {{ status('events') }}
        </div>

        <ol v-if="events.length" class="event-list">
          <li v-for="event in events" :key="event.req_uuid" class="event-item">
            <div class="event-line">
              <span class="event-dot"></span><strong class="mono" :title="event.src_ip">{{ event.src_ip }}</strong
              ><span class="event-action">{{ event.action || '阻止' }}</span>
            </div>
            <div class="event-place" :title="`${eventLocation(event)} · ${event.host || '未知站点'}`">
              {{ eventLocation(event) }}<span>{{ event.host || '未知站点' }}</span>
            </div>
            <div class="event-detail">
              <time>{{ event.create_time || '时间未知' }}</time
              ><span :title="event.rule">{{ event.rule || '未标注规则' }}</span>
            </div>
          </li>
        </ol>
        <div v-else class="panel-empty">
          <span class="empty-radar" aria-hidden="true">◎</span
          >{{ sources.events.updatedAt ? '近 24 小时暂无拦截事件' : status('events')
          }}<small v-if="sources.events.updatedAt">检测到新的拦截事件后将在这里显示</small>
        </div>
        <router-link
          v-if="!sharedView"
          class="panel-foot events-link"
          :to="{ name: 'WafvisitLog', query: { view_type: 'event', action: '阻止' } }"
          >查看安全日志 <span aria-hidden="true">↗</span></router-link
        >
      </section>
    </div>

    <section class="screen-panel qps-panel" aria-label="实时 QPS">
      <div class="qps-overview">
        <div class="section-eyebrow">LIVE TRAFFIC</div>
        <h2>实时 QPS</h2>
        <div class="qps-current">
          <strong>{{ formatCount(sources.qps.data?.Current) }}</strong
          ><span>次 / 秒</span>
        </div>
        <span class="qps-peak"
          >窗口峰值 <b>{{ formatCount(sources.qps.data?.Max) }}</b></span
        >
      </div>
      <div class="qps-plot">
        <QpsChart
          :points="sources.qps.data?.Points || []"
          :current="formatCount(sources.qps.data?.Current)"
          :dark="dark"
        />
        <div v-if="!sources.qps.data?.Points?.length" class="qps-empty">
          {{ sources.qps.updatedAt ? '暂无 QPS 采样，等待后端采集' : status('qps') }}
        </div>
        <span v-if="sources.qps.data?.Points?.length && status('qps')" class="qps-status warning">{{
          status('qps')
        }}</span>
      </div>
    </section>
    <footer class="screen-footer">
      <span>HomeWaf <span class="footer-dot">·</span> UV 按 IP 跨站点去重 · 统计可能存在入库延迟</span
      ><span
        >汇总 {{ summaryUpdated }} <span class="footer-dot">·</span> QPS 每 5 秒更新
        <span class="footer-dot">·</span> 服务端 {{ timezone
        }}<template v-if="snapshot?.share_expires_at">
          · 分享到期 {{ new Date(snapshot.share_expires_at).toLocaleString() }}</template
        ></span
      >
    </footer>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import QpsChart from './QpsChart.vue';
import WorldMap from './WorldMap.vue';
import ScreenControls from './ScreenControls.vue';
import { blockRate, eventLocation, formatCount, sourceLabels } from './data';
import { useScreenData } from './useScreenData';
const props = defineProps({ sharedView: Boolean, sharedToken: { type: String, default: '' } });
const { sources, snapshot, sharedError, paused, busy, now, issues, status, refresh, togglePause } = useScreenData({
  shared: props.sharedView,
  token: props.sharedToken,
});
const screen = ref<HTMLElement>();
const dark = ref(false);
const fullscreen = ref(false);
const fullscreenError = ref('');
const timezone = computed(() => snapshot.value?.timezone || '服务端时间待同步');
const windowLabel = computed(() =>
  snapshot.value
    ? '统计区间（UTC）：' +
      new Date(snapshot.value.window_start).toISOString() +
      ' 至 ' +
      new Date(snapshot.value.window_end).toISOString()
    : '等待服务端返回统计窗口',
);
const dateText = computed(
  () =>
    `${now.value.getFullYear()}-${String(now.value.getMonth() + 1).padStart(2, '0')}-${String(now.value.getDate()).padStart(2, '0')}`,
);
const timeText = computed(() => now.value.toLocaleTimeString('en-GB', { hour12: false }));
const summaryUpdated = computed(() =>
  sources.summary.updatedAt
    ? new Date(sources.summary.updatedAt).toLocaleTimeString('en-GB', { hour12: false })
    : '尚未更新',
);
const connectionWarning = computed(() => paused.value || issues.value.length > 0 || snapshot.value?.partial);
const connectionLabel = computed(() =>
  paused.value
    ? '自动刷新已暂停'
    : issues.value.length
      ? `${issues.value.map((key) => sourceLabels[key]).join(' / ')}数据异常`
      : Object.values(sources).every((s) => s.updatedAt)
        ? snapshot.value?.partial
          ? '数据已连接 · 统计不完整'
          : '数据已连接'
        : '正在连接数据源',
);
const rankedIPs = computed(() =>
  [...(sources.ips.data?.AttackIPOfRange || [])].sort((a, b) => b.count - a.count).slice(0, 5),
);
const regions = computed(() => [...(sources.regions.data || [])].sort((a, b) => b.value - a.value));
const unblockedRequests = computed(() => {
  const summary = sources.summary.data;
  return summary ? summary.visits - summary.blocked : undefined;
});
const events = computed(() =>
  [...(sources.events.data?.list || [])].sort((a, b) => b.unix_add_time - a.unix_add_time).slice(0, 12),
);
const metrics = computed(() => [
  {
    label: '近 24 小时访问次数',
    value: sources.summary.data?.visits,
    note: '滚动窗口内已记录的 HTTP 请求',
    color: 'violet',
    path: 'M5 23V9h22v14ZM11 28h10M16 23v5M9 16h4l3-4 3 7 2-3h3',
  },
  {
    label: '近 24 小时拦截次数',
    value: sources.summary.data?.blocked,
    note: '攻击来源 IP：' + formatCount(sources.summary.data?.attack_ips),
    color: 'coral',
    path: 'm16 3 11 4v9c0 6-6 11-11 14C11 27 5 22 5 16V7ZM11 16l4 4 7-8',
  },
  {
    label: '24 小时独立访客 UV',
    value: sources.summary.data?.visitors,
    note: 'IP 跨站去重 · 非自然人数',
    color: 'blue',
    path: 'M12 15a5 5 0 1 0 0-10 5 5 0 0 0 0 10ZM3 28v-3a9 9 0 0 1 18 0v3M24 5a5 5 0 0 1 0 10m0 5a7 7 0 0 1 5 7',
  },
]);
function toggleTheme() {
  dark.value = !dark.value;
  try {
    localStorage.setItem('homewaf-screen-theme', dark.value ? 'dark' : 'light');
  } catch {
    /* Private browsing still permits theme switching. */
  }
}
function onFullscreen() {
  fullscreen.value = document.fullscreenElement === screen.value;
}
async function toggleFullscreen() {
  fullscreenError.value = '';
  try {
    if (document.fullscreenElement) await document.exitFullscreen();
    else if (screen.value?.requestFullscreen) await screen.value.requestFullscreen();
    else fullscreenError.value = '当前浏览器不支持全屏，请使用浏览器的全屏模式。';
  } catch {
    fullscreenError.value = '全屏请求未成功，请使用浏览器的全屏模式。';
  }
}
onMounted(() => {
  try {
    dark.value = localStorage.getItem('homewaf-screen-theme') === 'dark';
  } catch {
    /* Keep the default light theme. */
  }
  document.addEventListener('fullscreenchange', onFullscreen);
});
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFullscreen);
  if (document.fullscreenElement === screen.value) void document.exitFullscreen().catch(() => {});
});
</script>

<style scoped src="./screen.css"></style>
