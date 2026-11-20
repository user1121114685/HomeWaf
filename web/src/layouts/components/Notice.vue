<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->
<template>
  <ElPopover
    v-model:visible="open"
    trigger="click"
    placement="bottom-end"
    width="min(400px, calc(100vw - 24px))"
    :teleported="true"
  >
    <template #reference>
      <hw-badge :count="badge">
        <hw-button
          class="notice-bell"
          :class="{ 'has-net-error': netErrors > 0 }"
          shape="square"
          variant="text"
          :aria-label="t('page.notice.notice_title')"
          :aria-expanded="open"
          :title="bellTitle"
        >
          <Bell />
        </hw-button>
      </hw-badge>
    </template>
    <section class="notice-panel" :aria-label="t('page.notice.notice_title')">
      <header class="notice-heading">
        <h2>{{ t('page.notice.notice_title') }}</h2>
        <hw-button
          variant="text"
          shape="square"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          :disabled="notesLoading || logsLoading"
          @click="refresh"
          ><Refresh
        /></hw-button>
      </header>
      <div class="notice-body">
        <section v-if="canSeeNotes">
          <div class="notice-heading">
            <h3>{{ text('重要待办', 'Important tasks') }}</h3>
            <RouterLink :to="{ name: 'UpgradeNotice' }">{{ t('menu.system.upgrade_notice_title') }}</RouterLink>
          </div>
          <p v-if="notesLoading" role="status">{{ text('正在读取待办…', 'Loading tasks…') }}</p>
          <p v-else-if="notesError" role="alert">
            {{ notesError }} <button class="t-button-link" @click="loadNotes">{{ t('topNav.retry') }}</button>
          </p>
          <ul v-else-if="notes.length">
            <li v-for="item in notes.slice(0, 3)" :key="item.notice_id">
              <RouterLink :to="noteLink(item)">{{ item.title }}</RouterLink>
            </li>
          </ul>
          <p v-else>{{ text('没有待处理的重要升级事项。', 'No high-priority upgrade tasks are pending.') }}</p>
        </section>
        <section v-if="canSeeLogs">
          <div class="notice-heading">
            <h3>{{ text('最近 24 小时的拦截', 'Blocks in the last 24 hours') }}</h3>
            <RouterLink :to="{ name: 'WafvisitLog', query: logFilters }">{{
              text('查看记录', 'View logs')
            }}</RouterLink>
          </div>
          <p v-if="logsLoading" role="status">{{ text('正在读取拦截记录…', 'Loading blocked requests…') }}</p>
          <p v-else-if="logsError" role="alert">
            {{ logsError }} <button class="t-button-link" @click="loadLogs">{{ t('topNav.retry') }}</button>
          </p>
          <template v-else>
            <p v-if="partial">
              {{
                text(
                  '部分日志分区未返回，记录可能不完整。',
                  'Some archives were unavailable. Results may be incomplete.',
                )
              }}
            </p>
            <ul v-if="blocks.length">
              <li v-for="row in blocks" :key="row.req_uuid">
                <RouterLink :to="logLink(row)">
                  <strong>{{ row.rule || text('已拦截请求', 'Blocked request') }}</strong>
                  <small>{{ [row.src_ip, row.create_time].filter(Boolean).join(' · ') }}</small>
                </RouterLink>
              </li>
            </ul>
            <p v-else-if="!partial">{{ text('这段时间没有拦截记录。', 'No blocks were recorded in this period.') }}</p>
          </template>
        </section>
        <section v-for="group in liveGroups" :key="group.key">
          <div class="notice-heading">
            <h3>{{ group.title }} ({{ group.items.length }})</h3>
            <hw-button variant="text" size="small" @click="store.commit('notification/markCategoryRead', group.key)">
              {{ text('全部设为已读', 'Mark all read') }}
            </hw-button>
          </div>
          <ul>
            <li v-for="item in group.items" :key="item.message_id">
              <strong
                >{{ item.message_type }}<span v-if="item.message_count > 1"> ×{{ item.message_count }}</span></strong
              >
              <p>{{ item.message_data }}</p>
              <div class="notice-heading">
                <small>{{ item.message_datetime }}</small>
                <button class="t-button-link" @click="store.commit('notification/markMessageRead', item.message_id)">
                  {{ t('page.notice.set_read') }}
                </button>
              </div>
            </li>
          </ul>
        </section>
      </div>
      <RouterLink v-if="routeNames.has('VpConfig')" class="notice-safety" :to="{ name: 'VpConfig' }">
        <Lock aria-hidden="true" />{{
          text('管理端口仅供内网访问。检查管理配置', 'Keep console access private. Review settings')
        }}
      </RouterLink>
    </section>
  </ElPopover>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import { ElPopover } from 'element-plus';
import { Bell, Lock, Refresh } from '@element-plus/icons-vue';
import store from '@/store';
import request from '@/utils/request';
import { logDateRange, logTimeBounds } from '@/pages/waf/attack/logPresentation';
import type { msgDataItem } from '@/store/modules/notification';

interface Task {
  notice_id: string;
  title: string;
  page?: string;
}
interface Block {
  req_uuid: string;
  rule: string;
  src_ip: string;
  create_time: string;
  shard_name?: string;
}
const route = useRoute();
const router = useRouter();
const { t, locale } = useI18n();
const text = (zh: string, en: string) => (locale.value === 'en_US' ? en : zh);
const open = ref(false);
const routeNames = computed(
  () =>
    new Set<string>(
      store.getters['permission/routers'].flatMap((parent) => parent.children?.map((child) => child.name) || []),
    ),
);
const canSeeNotes = computed(() => routeNames.value.has('UpgradeNotice'));
const canSeeLogs = computed(() => routeNames.value.has('WafvisitLog'));
const notes = ref<Task[]>([]);
const blocks = ref<Block[]>([]);
const notesLoading = ref(false);
const logsLoading = ref(false);
const notesError = ref('');
const logsError = ref('');
const partial = ref(false);
const logFilters = ref({ request_outcome: 'blocked', date_begin: '', date_end: '' });
let notesRequest = 0,
  logsRequest = 0;

const liveGroups = computed(() =>
  [
    {
      key: 'guard',
      title: text('实时防护告警', 'Live protection alerts'),
      items: store.getters['notification/guardMsg'] as msgDataItem[],
    },
    {
      key: 'system',
      title: text('系统消息', 'System messages'),
      items: store.getters['notification/systemMsg'] as msgDataItem[],
    },
  ].filter((group) => group.items.length),
);
const netErrors = computed(() => store.getters['notification/netUnreadCount']);
const badge = computed(() => notes.value.length + store.getters['notification/unreadMsg'].length);
const bellTitle = computed(() =>
  netErrors.value ? t('page.notice.net_error_tip', { count: netErrors.value }) : t('page.notice.notice_title'),
);

async function loadNotes() {
  if (!canSeeNotes.value) return;
  const seq = ++notesRequest;
  notesLoading.value = true;
  notesError.value = '';
  try {
    const res = await request({ url: 'upgradenotice/summary', method: 'get', params: { lang: locale.value } });
    if (seq !== notesRequest) return;
    if (res.code !== 0) throw new Error(res.msg);
    const data = res.data;
    // Go serializes an empty slice as null.
    if (
      !Number.isInteger(data?.high_pending_count) ||
      (data.high_pending_count > 0 && !Array.isArray(data.popup_items))
    )
      throw new Error();
    notes.value = data.popup_items || [];
  } catch (error) {
    if (seq !== notesRequest) return;
    notes.value = [];
    notesError.value =
      (error instanceof Error && error.message) || text('无法读取重要待办。', 'Could not load pending tasks.');
  } finally {
    if (seq === notesRequest) notesLoading.value = false;
  }
}
async function loadLogs() {
  if (!canSeeLogs.value) return;
  const seq = ++logsRequest;
  logsLoading.value = true;
  logsError.value = '';
  const range = logDateRange('24h');
  logFilters.value = { request_outcome: 'blocked', date_begin: range[0], date_end: range[1] };
  try {
    const res = await request({
      url: 'waflog/attack/list',
      method: 'post',
      background: true,
      data: {
        view_type: 'access',
        current_db_name: 'auto',
        request_outcome: 'blocked',
        ...logTimeBounds(range),
        sort_by: 'unix_add_time',
        sort_descending: 'desc',
        pageIndex: 1,
        pageSize: 3,
      },
    });
    if (seq !== logsRequest) return;
    if (res.code !== 0) throw new Error(res.msg);
    if (
      !Number.isInteger(res.data?.total) ||
      (!Array.isArray(res.data.list) && !(res.data.total === 0 && res.data.list === null))
    )
      throw new Error();
    blocks.value = res.data.list || [];
    partial.value = !!(res.data.partial || res.data.issues?.length);
  } catch (error) {
    if (seq !== logsRequest) return;
    blocks.value = [];
    logsError.value =
      (error instanceof Error && error.message) || text('无法读取拦截记录。', 'Could not load blocked requests.');
  } finally {
    if (seq === logsRequest) logsLoading.value = false;
  }
}
function refresh() {
  loadNotes();
  loadLogs();
}
function noteLink(item: Task) {
  if (
    item.page?.startsWith('/') &&
    !item.page.startsWith('//') &&
    routeNames.value.has(String(router.resolve(item.page).name))
  )
    return item.page;
  return { name: 'UpgradeNotice' };
}
function logLink(row: Block) {
  return routeNames.value.has('WafAttackLogDetail')
    ? { name: 'WafAttackLogDetail', query: { req_uuid: row.req_uuid + '#' + (row.shard_name || 'auto') } }
    : { name: 'WafvisitLog', query: { req_uuid: row.req_uuid } };
}
watch(open, (value) => {
  if (value) refresh();
});
watch(
  () => route.fullPath,
  () => {
    open.value = false;
  },
);
watch(
  [canSeeNotes, locale, () => store.state.user.token],
  () => {
    ++notesRequest;
    ++logsRequest;
    notes.value = [];
    blocks.value = [];
    if (store.state.user.token) {
      loadNotes();
      if (open.value) loadLogs();
    }
  },
  { immediate: true },
);
</script>

<style scoped>
.notice-panel {
  color: var(--hw-ink);
}
.notice-heading,
.notice-safety {
  display: flex;
  align-items: center;
  gap: 10px;
}
.notice-heading {
  justify-content: space-between;
}
.notice-heading h2,
.notice-heading h3,
.notice-body p {
  margin: 0;
}
.notice-heading h2 {
  font-size: 16px;
}
.notice-heading h3 {
  font-size: 14px;
}
.notice-body {
  max-height: min(62dvh, 480px);
  overflow: auto;
  overscroll-behavior: contain;
}
.notice-body section {
  padding: 12px 0;
  border-top: 1px solid var(--hw-line);
}
.notice-body p,
.notice-body small {
  color: var(--hw-muted);
  font-size: 13px;
}
.notice-body p,
.notice-body ul {
  margin: 6px 0 0;
}
.notice-body ul {
  list-style: none;
  padding: 0;
}
.notice-body li {
  padding: 5px 0;
  overflow-wrap: anywhere;
}
.notice-body li + li {
  border-top: 1px solid var(--hw-line);
}
.notice-panel a {
  color: var(--hw-accent);
}
.notice-body li a {
  display: block;
  padding: 4px 0;
  color: var(--hw-ink);
}
.notice-body li a:hover {
  color: var(--hw-accent);
}
.notice-body li strong,
.notice-body li small {
  display: block;
}
.notice-safety {
  padding-top: 10px;
  border-top: 1px solid var(--hw-line);
  font-size: 13px;
}
.notice-panel svg,
.notice-bell svg {
  width: 17px;
  height: 17px;
  flex: none;
}
.notice-bell.has-net-error {
  color: var(--hw-danger);
}
</style>
