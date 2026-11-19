<!-- HomeWaf modification (2026-11-17): certificate lifecycle entry point. -->
<template>
  <div class="certificate-page resource-panel">
    <div class="certificate-toolbar">
      <div></div>
      <div class="certificate-actions">
        <ElButton type="primary" @click="router.push(certificateRoot + '/request')">申请证书</ElButton
        ><ElButton @click="editor?.open()">导入证书</ElButton
        ><RouterLink :to="certificateRoot + '/settings'">证书设置</RouterLink>
      </div>
    </div>
    <form class="certificate-filters" @submit.prevent="search">
      <label class="certificate-search"
        >证书域名<ElInput v-model="filters.domains" clearable placeholder="搜索域名或 IP"
      /></label>
      <label
        >有效性<ElSelect v-model="filters.state" clearable placeholder="全部"
          ><ElOption v-for="(label, value) in validityStates" :key="value" :value="value" :label="label" /></ElSelect
      ></label>
      <label
        >更新方式<ElSelect v-model="filters.method" clearable placeholder="全部"
          ><ElOption v-for="(label, value) in updateMethods" :key="value" :value="value" :label="label" /></ElSelect
      ></label>
      <label
        >使用服务<ElSelect v-model="filters.service" clearable filterable placeholder="全部服务"
          ><ElOption
            v-for="target in serviceTargets"
            :key="target.value"
            :value="target.value"
            :label="target.label" /></ElSelect
      ></label>
      <ElCheckbox v-model="filters.attention">需要处理</ElCheckbox><ElButton native-type="submit">查询</ElButton
      ><ElButton text @click="reset">重置</ElButton><ElButton text :loading="loading" @click="load">刷新</ElButton>
    </form>
    <p v-if="error" class="certificate-error" role="alert">{{ error }} <ElButton text @click="load">重试</ElButton></p>
    <ElAlert v-if="savedWarning" :title="savedWarning" type="warning" show-icon @close="savedWarning = ''" />
    <p v-if="loading && !rows.length" role="status">正在读取证书…</p>
    <div v-else-if="!error && !rows.length" class="certificate-empty">
      <h3>{{ filtered ? '没有符合条件的证书' : '还没有证书' }}</h3>
      <p>{{ filtered ? '调整筛选条件后再试。' : '可以申请新证书，或导入已有证书和私钥。' }}</p>
      <ElButton v-if="filtered" @click="reset">清除筛选</ElButton>
    </div>
    <table v-if="rows.length" class="certificate-table" :aria-busy="loading">
      <caption class="sr-only">
        当前证书与续期状态
      </caption>
      <thead>
        <tr>
          <th>证书</th>
          <th>有效性</th>
          <th>使用范围</th>
          <th>更新方式</th>
          <th>续期与故障</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.id" :data-certificate-id="row.id">
          <td data-label="证书">
            <RouterLink :to="detailLink(row)"
              ><strong>{{ row.domains || row.subject || '未指定域名' }}</strong></RouterLink
            ><small>{{ row.issuer || '颁发者未知' }}</small>
          </td>
          <td data-label="有效性">
            <span :class="['expired', 'invalid'].includes(row.validity) ? 'certificate-error' : ''">{{
              validityStates[row.validity]
            }}</span
            ><small>{{ showTime(row.valid_to) }}</small>
          </td>
          <td data-label="使用范围">
            <template v-if="row.bindings?.length"
              ><div v-for="binding in row.bindings" :key="binding.kind + binding.id + binding.source">
                {{ binding.name
                }}<small>{{
                  binding.kind === 'manager'
                    ? '管理控制台绑定'
                    : binding.protocol.toUpperCase() + ' :' + binding.port + ' · 已绑定'
                }}</small>
              </div></template
            ><span v-else class="certificate-muted">未绑定服务</span>
          </td>
          <td data-label="更新方式">
            {{ updateMethods[row.update_method] }}<small v-if="row.export_status">{{ row.export_status }}</small>
          </td>
          <td data-label="续期与故障">
            <span :class="row.renewal?.last_error ? 'certificate-error' : ''">{{ renewalLabel(row) }}</span
            ><small
              v-if="
                row.renewal && ['queued', 'running', 'deploying', 'retry_wait', 'blocked'].includes(row.renewal.status)
              "
              >{{ row.renewal.enabled ? '自动续期已开启' : '自动续期已关闭' }}</small
            ><small v-if="row.renewal?.last_error">{{ row.renewal.last_error }}</small
            ><small v-if="showTime(row.renewal?.next_attempt_at) !== '—'"
              >下次尝试：{{ showTime(row.renewal.next_attempt_at) }}</small
            ><RouterLink v-if="row.renewal" :to="detailLink(row, true)">续期设置</RouterLink>
          </td>
          <td data-label="操作">
            <div class="certificate-actions">
              <ElButton
                v-if="row.renewal && row.renewal.status !== 'manual'"
                text
                type="primary"
                @click="openRenewal(row)"
                >{{
                  ['queued', 'running'].includes(row.renewal.status)
                    ? '查看进度'
                    : row.renewal.last_error
                      ? '处理失败'
                      : '立即续期'
                }}</ElButton
              ><ElButton text @click="edit(row)">编辑证书</ElButton
              ><ElDropdown @command="remove(row)"
                ><ElButton text aria-label="证书更多操作">更多</ElButton
                ><template #dropdown
                  ><ElDropdownMenu><ElDropdownItem command="delete">删除证书</ElDropdownItem></ElDropdownMenu></template
                ></ElDropdown
              >
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="rows.length" class="certificate-pagination">
      <ElPagination
        v-model:current-page="page"
        :page-size="20"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="searchPage"
      />
    </div>
    <CertificateEditor ref="editor" @saved="saved" />
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElDropdown,
  ElDropdownMenu,
  ElDropdownItem,
  ElInput,
  ElSelect,
  ElOption,
  ElPagination,
  ElMessage,
  ElMessageBox,
} from 'element-plus';
import CertificateEditor from './CertificateEditor.vue';
import {
  certificateListApi,
  certificateRoot,
  validityStates,
  updateMethods,
  showTime,
  renewalLabel,
  type CertificateRecord,
} from '@/apis/certificates';
import { sslConfigDetailApi, sslConfigDelApi } from '@/apis/sslconfig';
import { sslCertificateTargetsApi } from '@/apis/sslorder';
import './certificates.css';
const route = useRoute(),
  router = useRouter();
const filters = ref({
  domains: String(route.query.domains || ''),
  state: String(route.query.state || ''),
  method: String(route.query.method || ''),
  service: String(route.query.service || ''),
  attention: route.query.attention === '1',
});
const page = ref(Math.max(1, Number(route.query.page) || 1)),
  total = ref(0),
  rows = ref<CertificateRecord[]>([]),
  error = ref(''),
  loading = ref(false),
  savedWarning = ref('');
const serviceTargets = ref<any[]>([]);
const editor = ref<InstanceType<typeof CertificateEditor>>();
const filtered = computed(() => Object.values(filters.value).some(Boolean));
let sequence = 0;
async function load() {
  const current = ++sequence;
  loading.value = true;
  try {
    const res = await certificateListApi({ ...filters.value, pageIndex: page.value, pageSize: 20 });
    if (current !== sequence) return;
    if (res.code !== 0) throw new Error(res.msg);
    rows.value = res.data.list || [];
    total.value = res.data.total;
    error.value = '';
  } catch (e) {
    if (current === sequence) error.value = (e instanceof Error ? e.message : '') || '证书读取失败，当前保留上次结果。';
  } finally {
    if (current === sequence) loading.value = false;
  }
}
async function searchPage() {
  const previousPath = route.fullPath;
  await router.replace({
    query: { ...filters.value, attention: filters.value.attention ? '1' : undefined, page: String(page.value) },
  });
  if (route.fullPath === previousPath) await load();
}
watch(
  () => route.fullPath,
  () => {
    if (route.path !== certificateRoot) return;
    filters.value = {
      domains: String(route.query.domains || ''),
      state: String(route.query.state || ''),
      method: String(route.query.method || ''),
      service: String(route.query.service || ''),
      attention: route.query.attention === '1',
    };
    page.value = Math.max(1, Number(route.query.page) || 1);
    void load();
  },
);
function search() {
  page.value = 1;
  void searchPage();
}
function reset() {
  filters.value = { domains: '', state: '', method: '', service: '', attention: false };
  search();
}
function detailLink(row: CertificateRecord, renewal = false) {
  return {
    path: certificateRoot + '/' + row.id,
    query: { returnTo: route.fullPath, ...(renewal ? { edit: '1' } : {}) },
  };
}
function openRenewal(row: CertificateRecord) {
  void router.push({ ...detailLink(row), query: { returnTo: route.fullPath, action: 'renew' } });
}
async function edit(row: CertificateRecord) {
  try {
    const res = await sslConfigDetailApi({ id: row.id });
    if (res.code !== 0) throw new Error(res.msg);
    editor.value?.open(res.data);
  } catch {
    ElMessage.error('读取证书编辑内容失败，请重试或检查权限');
  }
}
async function remove(row: CertificateRecord) {
  try {
    await ElMessageBox.confirm(
      '删除证书 ' + row.domains + '？正在使用或存在活动任务的证书会被服务器拒绝删除。',
      '删除证书',
      { type: 'warning', confirmButtonText: '删除证书', cancelButtonText: '取消' },
    );
    const res = await sslConfigDelApi({ id: row.id });
    if (res.code !== 0) throw new Error(res.msg);
    ElMessage.success('证书已删除');
    await load();
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '删除失败');
  }
}
function saved(result: { message: string; exportFailed: boolean }) {
  savedWarning.value = result.exportFailed ? result.message : '';
  ElMessage[result.exportFailed ? 'warning' : 'success'](result.message);
  void load();
}
onMounted(async () => {
  void load();
  try {
    const res = await sslCertificateTargetsApi();
    if (res.code !== 0) throw new Error(res.msg);
    serviceTargets.value = res.data || [];
  } catch {
    ElMessage.warning('服务筛选项读取失败，可刷新页面重试；证书列表仍可查看');
  }
});
</script>
