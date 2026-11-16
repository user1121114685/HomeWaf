<!-- HomeWaf modification (2026-11-17): observed certificates, separate from renewal. -->
<template>
  <div class="certificate-page resource-panel">
    <div class="certificate-toolbar">
      <div>
        <p class="certificate-muted">
          从 HomeWaf 所在机器检查域名与端口实际提供的证书。监测不会开启自动续期或修改服务。
        </p>
      </div>
      <div class="certificate-actions">
        <ElButton type="primary" @click="open()">添加监测</ElButton><ElButton @click="openServices">从服务添加</ElButton
        ><ElButton :loading="checking === 'all'" @click="check()">检查全部</ElButton>
      </div>
    </div>
    <form
      class="certificate-filters"
      @submit.prevent="
        page = 1;
        load();
      "
    >
      <label>域名 / IP<ElInput v-model="search" clearable placeholder="筛选监测目标" /></label
      ><ElButton native-type="submit">查询</ElButton><ElButton @click="load">刷新结果</ElButton>
    </form>
    <p v-if="notice" role="status">{{ notice }}</p>
    <p v-if="error" role="alert" class="certificate-error">{{ error }} <ElButton text @click="load">重试</ElButton></p>
    <p v-if="loading && !rows.length" role="status">正在读取监测目标…</p>
    <div v-else-if="!error && !rows.length" class="certificate-empty">
      <h3>{{ search ? '没有符合条件的目标' : '还没有监测目标' }}</h3>
      <p>添加域名和实际 HTTPS 端口，或从服务选择。</p>
    </div>
    <table v-if="rows.length" class="certificate-table" :aria-busy="loading">
      <thead>
        <tr>
          <th>监测目标</th>
          <th>本次状态</th>
          <th>远端证书有效期</th>
          <th>最近检测</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.id">
          <td data-label="监测目标">
            <strong>{{ row.domain }}:{{ row.port }}</strong
            ><small
              ><RouterLink :to="{ path: certificateRoot, query: { domains: row.domain } }">查找相关证书</RouterLink
              >（域名相同不代表证书版本相同）</small
            >
          </td>
          <td data-label="本次状态">
            <span :class="['failed', 'expired'].includes(row.status) ? 'certificate-error' : ''">{{
              states[row.status] || '状态未知'
            }}</span
            ><small v-if="row.visit_log" class="certificate-error">{{ row.visit_log }}</small>
          </td>
          <td data-label="远端证书有效期">
            {{ showTime(row.valid_to)
            }}<small v-if="['failed', 'stale'].includes(row.status)">上次成功结果；请检查本次状态与检测时间</small
            ><small v-else>{{ row.expiration_info }}</small>
          </td>
          <td data-label="最近检测">
            {{ showTime(row.last_detect) }}<small>最近成功：{{ showTime(row.last_success) }}</small>
          </td>
          <td data-label="操作">
            <div class="certificate-actions">
              <ElButton
                text
                type="primary"
                :loading="checking === row.id || row.status === 'checking'"
                @click="check(row.id)"
                >重新检查</ElButton
              ><ElButton text @click="open(row)">编辑目标</ElButton
              ><ElButton text @click="remove(row)">移除监测</ElButton>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <div class="certificate-pagination">
      <ElPagination
        v-model:current-page="page"
        :total="total"
        :page-size="20"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>
    <p class="certificate-muted">
      到期提醒由
      <RouterLink to="/notify/subscription">消息通知中的证书到期订阅</RouterLink>管理；续期失败使用操作通知，两者独立。
    </p>
    <ElDialog
      v-model="visible"
      :title="draft.id ? '编辑监测目标' : '添加监测目标'"
      width="540px"
      :close-on-click-modal="false"
      :before-close="close"
    >
      <ElForm label-position="top" :disabled="saving" @submit.prevent="save"
        ><ElFormItem label="域名或 IP"
          ><ElInput v-model="draft.domain" placeholder="nas.example.com（不含协议或路径）" /></ElFormItem
        ><ElFormItem label="HTTPS 端口"><ElInputNumber v-model="draft.port" :min="1" :max="65535" /></ElFormItem>
        <p class="certificate-muted">目标变更后会清除旧目标的检测结果，等待重新检查。</p>
        <p v-if="saveError" role="alert" class="certificate-error">{{ saveError }}</p></ElForm
      >
      <template #footer
        ><ElButton :disabled="saving" @click="close(() => (visible = false))">取消</ElButton
        ><ElButton type="primary" :loading="saving" @click="save">保存目标</ElButton></template
      >
    </ElDialog>
    <ElDialog v-model="servicesVisible" title="从服务添加监测" width="680px" :close-on-click-modal="false">
      <p>选择已有 HTTPS 服务的域名与端口。只增加缺失目标，不修改服务。</p>
      <p v-if="servicesError" role="alert" class="certificate-error">
        {{ servicesError }} <ElButton @click="loadServices">重新读取服务</ElButton>
      </p>
      <ElCheckboxGroup v-model="selected"
        ><div v-for="target in targets" :key="target.key">
          <ElCheckbox :value="target.key">{{ target.label }} · {{ target.domain }}:{{ target.port }}</ElCheckbox>
        </div></ElCheckboxGroup
      >
      <p v-if="!servicesLoading && !servicesError && !targets.length">
        没有可添加的 HTTPS 域名服务，请手动添加监测目标。
      </p>
      <template #footer
        ><ElButton :disabled="adding" @click="servicesVisible = false">返回</ElButton
        ><ElButton type="primary" :disabled="!selected.length || !!servicesError" :loading="adding" @click="addSelected"
          >添加选中目标</ElButton
        ></template
      >
    </ElDialog>
  </div>
</template>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router';
import {
  ElButton,
  ElInput,
  ElInputNumber,
  ElForm,
  ElFormItem,
  ElDialog,
  ElCheckboxGroup,
  ElCheckbox,
  ElPagination,
  ElMessage,
  ElMessageBox,
} from 'element-plus';
import {
  wafSslExpireListApi,
  wafSslExpireAddApi,
  wafSslExpireEditApi,
  wafSslExpireDelApi,
  wafSslExpireNowCheckApi,
} from '@/apis/ssl_expire';
import { sslCertificateTargetsApi } from '@/apis/sslorder';
import { certificateRoot, showTime } from '@/apis/certificates';
import '../sslconfig/certificates.css';
const route = useRoute(),
  router = useRouter();
const rows = ref<any[]>([]),
  search = ref(String(route.query.domain || '')),
  page = ref(Number(route.query.page) || 1),
  total = ref(0),
  loading = ref(false),
  error = ref(''),
  notice = ref(''),
  checking = ref('');
const visible = ref(false),
  saving = ref(false),
  saveError = ref(''),
  draft = ref({ id: '', domain: '', port: 443 }),
  original = ref('');
const servicesVisible = ref(false),
  servicesLoading = ref(false),
  servicesError = ref(''),
  targets = ref<any[]>([]),
  selected = ref<string[]>([]),
  adding = ref(false);
const states: Record<string, string> = {
  unverified: '未检测',
  checking: '正在检测',
  valid: '正常',
  expiring: '临近到期',
  expired: '已过期',
  failed: '检测失败',
  stale: '上次结果，待重新检测',
};
let sequence = 0,
  until = 0,
  timer: ReturnType<typeof setInterval> | undefined;
async function load() {
  await router.replace({ query: { ...route.query, domain: search.value || undefined, page: String(page.value) } });
  const current = ++sequence;
  loading.value = true;
  try {
    const res = await wafSslExpireListApi({ domain: search.value, pageIndex: page.value, pageSize: 20 });
    if (current !== sequence) return;
    if (res.code !== 0) throw new Error(res.msg);
    rows.value = res.data.list || [];
    total.value = res.data.total;
    error.value = '';
  } catch {
    if (current === sequence) error.value = '读取监测目标失败，当前保留上次结果。';
  } finally {
    if (current === sequence) loading.value = false;
  }
}
function open(row?: any) {
  draft.value = row ? { id: row.id, domain: row.domain, port: row.port } : { id: '', domain: '', port: 443 };
  original.value = JSON.stringify(draft.value);
  saveError.value = '';
  visible.value = true;
}
async function canLeave() {
  if (saving.value || adding.value) return false;
  if (!visible.value || JSON.stringify(draft.value) === original.value) return true;
  try {
    await ElMessageBox.confirm('放弃尚未保存的监测目标修改？', '未保存的修改', {
      confirmButtonText: '放弃修改',
      cancelButtonText: '继续编辑',
      closeOnHashChange: false,
    });
    return true;
  } catch {
    return false;
  }
}
async function close(done: () => void) {
  if (await canLeave()) done();
}
async function save() {
  if (saving.value) return;
  saving.value = true;
  try {
    if (!draft.value.domain.trim()) throw new Error('请填写域名或 IP');
    const res = await (draft.value.id ? wafSslExpireEditApi(draft.value) : wafSslExpireAddApi(draft.value));
    if (res.code !== 0) throw new Error(res.msg);
    visible.value = false;
    await load();
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '保存失败，输入已保留';
  } finally {
    saving.value = false;
  }
}
async function check(id = '') {
  if (checking.value) return;
  checking.value = id || 'all';
  try {
    const res = await wafSslExpireNowCheckApi(id ? { id } : {});
    if (res.code !== 0) throw new Error(res.msg);
    notice.value = res.msg + '。页面将刷新检测结果。';
    until = Date.now() + 120000;
    await load();
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '检测提交失败');
  } finally {
    checking.value = '';
  }
}
async function remove(row: any) {
  try {
    await ElMessageBox.confirm(
      '移除 ' + row.domain + ':' + row.port + ' 的监测？这不会删除证书或停用服务。',
      '移除监测',
      { confirmButtonText: '移除监测', cancelButtonText: '取消' },
    );
    const res = await wafSslExpireDelApi({ id: row.id });
    if (res.code !== 0) throw new Error(res.msg);
    await load();
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '移除失败');
  }
}
async function loadServices() {
  servicesLoading.value = true;
  servicesError.value = '';
  try {
    const res = await sslCertificateTargetsApi();
    if (res.code !== 0) throw new Error(res.msg);
    const found = new Map();
    for (const service of res.data || []) {
      if (service.protocol !== 'https') continue;
      for (const domain of service.domains) {
        if (domain.includes('*')) continue;
        const key = domain + ':' + service.port;
        found.set(key, { key, domain, port: service.port, label: service.label });
      }
    }
    targets.value = [...found.values()];
  } catch {
    servicesError.value = '服务目标读取失败，不能当作没有服务。';
  } finally {
    servicesLoading.value = false;
  }
}
function openServices() {
  servicesVisible.value = true;
  selected.value = [];
  void loadServices();
}
async function addSelected() {
  if (adding.value) return;
  adding.value = true;
  const failures: string[] = [];
  for (const key of selected.value) {
    const target = targets.value.find((item) => item.key === key);
    try {
      const res = await wafSslExpireAddApi({ domain: target.domain, port: target.port });
      if (res.code !== 0 && !res.msg?.includes('存在') && !res.msg?.includes('已在')) throw new Error(res.msg);
    } catch {
      failures.push(key);
    }
  }
  selected.value = failures;
  adding.value = false;
  if (failures.length) servicesError.value = '部分目标添加失败，已保留待处理项，可再次提交。';
  else servicesVisible.value = false;
  await load();
}
function unload(e: BeforeUnloadEvent) {
  if (visible.value && JSON.stringify(draft.value) !== original.value) {
    e.preventDefault();
    e.returnValue = '';
  }
}
onBeforeRouteLeave(canLeave);
onMounted(() => {
  void load();
  window.addEventListener('beforeunload', unload);
  timer = setInterval(() => {
    if (
      !loading.value &&
      !document.hidden &&
      (Date.now() < until || rows.value.some((row) => row.status === 'checking'))
    )
      void load();
  }, 3000);
});
onBeforeUnmount(() => {
  sequence++;
  clearInterval(timer);
  window.removeEventListener('beforeunload', unload);
});
</script>
