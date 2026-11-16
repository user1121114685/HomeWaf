<!-- HomeWaf modification (2026-11-17): certificate execution history. -->
<template>
  <div class="certificate-page resource-panel">
    <div class="certificate-toolbar">
      <div>
        <RouterLink :to="back">返回证书管理</RouterLink>
      </div>
      <RouterLink v-if="srcHostCode" :to="{ path: certificateRoot + '/request', query: { host_code: srcHostCode } }"
        >为此服务申请证书</RouterLink
      >
    </div>
    <form class="certificate-filters" @submit.prevent="search">
      <label>申请域名<ElInput v-model="filters.apply_domain" clearable placeholder="搜索域名" /></label>
      <label
        >状态<ElSelect v-model="filters.apply_status" clearable placeholder="全部状态"
          ><ElOption v-for="(label, value) in statuses" :key="value" :value="value" :label="label" /></ElSelect
      ></label>
      <label
        >触发方式<ElSelect v-model="filters.trigger" clearable placeholder="全部"
          ><ElOption value="initial" label="首次申请" /><ElOption value="automatic" label="自动续期" /><ElOption
            value="manual"
            label="手动续期 / 恢复" /></ElSelect
      ></label>
      <ElButton native-type="submit">查询</ElButton><ElButton text @click="reset">重置</ElButton
      ><ElButton text :loading="loading" @click="load">刷新</ElButton>
    </form>
    <p v-if="error" role="alert" class="certificate-error">{{ error }}</p>
    <p v-if="route.query.certificate_id" class="certificate-muted">
      当前仅显示所选证书的任务。<RouterLink :to="String(certificateRoot + '/' + route.query.certificate_id)"
        >查看此证书</RouterLink
      >
    </p>
    <OrderTable
      v-if="!error || rows.length"
      :rows="rows"
      :host-names="hostNames"
      :loading="loading"
      :filtered="!!(filters.apply_domain || filters.apply_status || filters.trigger || route.query.certificate_id)"
      :retrying-order-id="retrying"
      :pagination="{ current: page, pageSize: 20, total }"
      @logs="openLogs"
      @retry="retry"
      @certificate="viewCertificate"
      @delete="remove"
      @page="changePage"
    />
    <RouterLink v-if="!loading && !error && !rows.length" :to="certificateRoot">到证书管理申请或导入证书</RouterLink>
    <OrderLogs
      v-model:visible="logsVisible"
      :order-id="orderId"
      :retrying="!!retrying"
      @retry="retry({ id: $event, apply_status: 'fail' })"
    />
  </div>
</template>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElButton, ElInput, ElSelect, ElOption, ElMessage, ElMessageBox } from 'element-plus';
import { v4 as uuid } from 'uuid';
import { sslOrderListApi, sslOrderRetryApi, sslOrderDelApi, sslCertificateTargetsApi } from '@/apis/sslorder';
import { certificateRoot, safeCertificateReturn } from '@/apis/certificates';
import OrderTable from './OrderTable.vue';
import OrderLogs from './OrderLogs.vue';
import '../sslconfig/certificates.css';
const props = defineProps({ srcHostCode: { type: String, default: '' } });
const route = useRoute(),
  router = useRouter();
const back = safeCertificateReturn(route.query.returnTo);
const statuses: Record<string, string> = {
  submitted: '等待处理',
  applying: '处理中',
  success: '已完成',
  fail: '失败',
  renewed: '历史签发',
  expired: '历史过期',
};
const filters = ref({
  apply_domain: String(route.query.apply_domain || ''),
  apply_status: String(route.query.apply_status || ''),
  trigger: String(route.query.trigger || ''),
});
const rows = ref<any[]>([]),
  hostNames = ref<Record<string, string>>({}),
  page = ref(Number(route.query.page) || 1),
  total = ref(0),
  loading = ref(false),
  error = ref(''),
  retrying = ref('');
const logsVisible = ref(false),
  orderId = ref('');
const retryIds: Record<string, string> = {};
let sequence = 0,
  timer: ReturnType<typeof setInterval> | undefined;
async function load(quiet = false) {
  if (quiet && loading.value) return;
  const version = ++sequence;
  loading.value = true;
  try {
    const res = await sslOrderListApi({
      ...filters.value,
      certificate_id: route.query.certificate_id || '',
      host_code: props.srcHostCode,
      pageIndex: page.value,
      pageSize: 20,
    });
    if (version !== sequence) return;
    if (res.code !== 0) throw new Error(res.msg);
    rows.value = res.data.list || [];
    total.value = res.data.total;
    error.value = '';
  } catch (e) {
    if (version === sequence) error.value = e instanceof Error ? e.message : '状态读取失败，保留上次结果';
  } finally {
    if (version === sequence) loading.value = false;
  }
}
async function search() {
  page.value = 1;
  await persist();
  await load();
}
async function persist() {
  await router.replace({ query: { ...route.query, ...filters.value, page: String(page.value) } });
}
function reset() {
  filters.value = { apply_domain: '', apply_status: '', trigger: '' };
  void search();
}
async function changePage(value: { current: number }) {
  page.value = value.current;
  await persist();
  await load();
}
function openLogs(id: string) {
  orderId.value = id;
  logsVisible.value = true;
}
function viewCertificate(row: any) {
  void router.push(
    row.certificate_id
      ? { path: certificateRoot + '/' + row.certificate_id, query: { returnTo: route.fullPath } }
      : { path: certificateRoot, query: { domains: row.apply_domain } },
  );
}
async function retry(row: any) {
  if (retrying.value) return;
  retrying.value = row.id;
  retryIds[row.id] ||= uuid();
  try {
    const res = await sslOrderRetryApi({ id: row.id, request_id: retryIds[row.id] });
    if (res.code !== 0) throw new Error(res.msg);
    delete retryIds[row.id];
    openLogs(res.data.id);
    await load();
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '提交结果未确认，可再次重试');
  } finally {
    retrying.value = '';
  }
}
async function remove(row: any) {
  try {
    await ElMessageBox.confirm(
      '删除这条历史记录？仍用于恢复、来源核对或正在执行的记录不能删除。证书及其续期设置保持独立。',
      '删除记录',
      { confirmButtonText: '删除记录', cancelButtonText: '取消' },
    );
    const res = await sslOrderDelApi({ id: row.id });
    if (res.code !== 0) throw new Error(res.msg);
    await load();
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '删除失败');
  }
}
onMounted(async () => {
  await load();
  try {
    const res = await sslCertificateTargetsApi();
    if (res.code === 0)
      hostNames.value = Object.fromEntries((res.data || []).map((item: any) => [item.value, item.label]));
  } catch {
    /* IDs remain visible when labels cannot be fetched. */
  }
  if (route.query.order) openLogs(String(route.query.order));
  timer = setInterval(() => {
    if (!document.hidden && rows.value.some((row) => ['submitted', 'applying'].includes(row.apply_status)))
      void load(true);
  }, 3000);
});
watch(
  [() => props.srcHostCode, () => route.query.certificate_id],
  () => {
    page.value = 1;
    rows.value = [];
    logsVisible.value = false;
    void load();
  },
);
onBeforeUnmount(() => {
  sequence++;
  clearInterval(timer);
});
</script>
