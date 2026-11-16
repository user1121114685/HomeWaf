<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->
<template>
  <div class="api-reference-page" data-testid="api-reference">
    <PageActions
      ><ElButton :loading="loading" @click="load">刷新接口文档</ElButton
      ><RouterLink class="el-button" to="/oplatform/key">管理调用凭据</RouterLink></PageActions
    >
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <section class="api-context">
      <div>
        <span class="context-label">当前管理入口</span><code>{{ origin }}</code>
      </div>
      <div>
        <span class="context-label">开放 API</span
        ><ElTag :type="enabled === 1 ? 'success' : 'info'">{{
          enabled === 1 ? '已启用' : enabled === 0 ? '未启用' : '状态未知'
        }}</ElTag>
      </div>
      <p>接口清单来自当前实例。部分接口仅接受管理会话，API Key 的可用范围仍由服务端校验。</p>
    </section>
    <section class="api-reference-panel">
      <div class="api-filters">
        <label
          ><span>查找接口</span
          ><ElInput v-model="query" clearable placeholder="路径、说明或请求方法" aria-label="查找接口"
        /></label>
        <label
          ><span>功能模块</span
          ><ElSelect v-model="module" clearable placeholder="全部模块" aria-label="功能模块"
            ><ElOption v-for="name in moduleNames" :key="name" :value="name" :label="name" /></ElSelect
        ></label>
        <span class="api-result-count">{{ filtered.length }} 个接口</span>
      </div>
      <p v-if="loading && !loaded" role="status" class="api-empty">正在读取当前实例的接口文档…</p>
      <p v-else-if="!filtered.length" class="api-empty">
        {{
          error
            ? '文档读取失败，请刷新重试。'
            : query || module
              ? '没有匹配的接口，请调整筛选条件。'
              : '当前实例没有返回可展示的接口。'
        }}
      </p>
      <div v-else class="api-reference-layout">
        <nav class="api-index" aria-label="接口列表">
          <button
            v-for="item in pageRows"
            :key="key(item)"
            type="button"
            :aria-current="selected && key(item) === key(selected) ? 'true' : undefined"
            @click="selectedKey = key(item)"
          >
            <span class="api-method">{{ item.method }}</span
            ><span
              ><code>{{ item.path }}</code
              ><small>{{ item.description || item.module }}</small></span
            >
          </button>
          <ElPagination
            v-if="filtered.length > pageSize"
            v-model:current-page="page"
            :page-size="pageSize"
            :total="filtered.length"
            :pager-count="5"
            layout="prev, pager, next"
            @current-change="selectedKey = ''"
          />
        </nav>
        <article v-if="selected" class="api-detail" data-testid="api-detail">
          <div class="api-detail-heading">
            <span class="api-method">{{ selected.method }}</span
            ><code>{{ selected.path }}</code
            ><ElButton text @click="copy(selected.path)">复制路径</ElButton>
          </div>
          <h2>{{ selected.description || '接口说明' }}</h2>
          <p v-if="!selected.description" class="api-muted">此接口尚未提供说明，请结合当前版本的接口定义使用。</p>
          <section class="api-example">
            <div>
              <h3>请求参数示例</h3>
              <ElButton v-if="selected.param_example" text @click="copy(selected.param_example)">复制参数</ElButton>
            </div>
            <pre v-if="selected.param_example">{{ format(selected.param_example) }}</pre>
            <p v-else class="api-muted">未提供参数示例。</p>
          </section>
          <section class="api-example">
            <div>
              <h3>响应示例</h3>
              <ElButton v-if="selected.response_example" text @click="copy(selected.response_example)"
                >复制响应</ElButton
              >
            </div>
            <pre v-if="selected.response_example">{{ format(selected.response_example) }}</pre>
            <p v-else class="api-muted">未提供响应示例。</p>
          </section>
          <section v-if="selected.curl_example" class="api-example">
            <div>
              <h3>cURL 示例</h3>
              <ElButton text @click="copy(curlExample)">复制命令</ElButton>
            </div>
            <pre>{{ curlExample }}</pre>
            <p class="api-muted">示例中的 Key 是占位值。执行前确认目标、参数及操作影响；此页面不会执行该命令。</p>
          </section>
        </article>
      </div>
    </section>
    <details class="api-help">
      <summary>认证、调用限制与排错</summary>
      <p>
        API Key 请求头：<code>{{ authHeader }}</code
        >。在调用凭据页配置来源、有效期和速率限制；不要将真实密钥放入共享文档。
      </p>
      <p>
        出现认证或权限错误时，先核对凭据状态及接口允许的认证方式，再查看调用记录。文档中的参数示例不代表所有部署都允许该操作。
      </p>
      <RouterLink to="/oplatform/log">查看 API 调用记录 →</RouterLink>
    </details>
  </div>
</template>
<script setup lang="ts">
import { computed, getCurrentInstance, onMounted, ref, watch } from 'vue';
import { ElAlert, ElButton, ElInput, ElMessage, ElOption, ElPagination, ElSelect, ElTag } from 'element-plus';
import { oplatform_doc_api, oplatform_doc_status_api } from '@/apis/oplatform';
interface ApiRoute {
  method: string;
  path: string;
  module: string;
  description?: string;
  param_example?: string;
  response_example?: string;
  curl_example?: string;
}
const routes = ref<ApiRoute[]>([]),
  query = ref(''),
  module = ref(''),
  selectedKey = ref(''),
  page = ref(1),
  pageSize = 40;
const loading = ref(false),
  loaded = ref(false),
  error = ref(''),
  enabled = ref<number>(),
  authHeader = ref('X-API-Key');
const origin = window.location.origin,
  component = getCurrentInstance();
const key = (item: ApiRoute) => item.method + ' ' + item.path;
const moduleNames = computed(() => [...new Set(routes.value.map((item) => item.module))].sort());
const filtered = computed(() => {
  const words = query.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
  return routes.value.filter(
    (item) =>
      (!module.value || item.module === module.value) &&
      words.every((word) =>
        (item.method + ' ' + item.path + ' ' + (item.description || '')).toLowerCase().includes(word),
      ),
  );
});
const pageRows = computed(() => filtered.value.slice((page.value - 1) * pageSize, page.value * pageSize));
const selected = computed(() => pageRows.value.find((item) => key(item) === selectedKey.value) || pageRows.value[0]);
const curlExample = computed(() =>
  (selected.value?.curl_example || '').replaceAll('http://your-samwaf-host:26666', origin),
);
function format(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
async function copy(value: string) {
  try {
    await component.proxy.$copyText(value);
    ElMessage.success('已复制');
  } catch {
    ElMessage.warning('复制失败，请选中文本手动复制。');
  }
}
async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  const results = await Promise.allSettled([oplatform_doc_api(), oplatform_doc_status_api()]);
  try {
    const doc = results[0];
    if (doc.status === 'rejected') throw doc.reason;
    if (doc.value.code !== 0) throw new Error(doc.value.msg || '接口文档读取失败');
    const groups = doc.value.data?.modules;
    if (!groups || typeof groups !== 'object' || Array.isArray(groups)) throw new Error('接口文档格式不正确');
    const next: ApiRoute[] = [];
    for (const [name, items] of Object.entries(groups)) {
      if (!Array.isArray(items)) continue;
      for (const item of items)
        if (typeof item.path === 'string' && typeof item.method === 'string') next.push({ ...item, module: name });
    }
    routes.value = next.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
    authHeader.value = doc.value.data.auth_info?.header_name || 'X-API-Key';
    loaded.value = true;
    if (page.value > Math.max(1, Math.ceil(filtered.value.length / pageSize))) page.value = 1;
  } catch (cause: any) {
    error.value = (cause.message || '接口文档读取失败') + (loaded.value ? '。当前仍显示上次读取的结果。' : '');
  }
  const status = results[1];
  enabled.value =
    status.status === 'fulfilled' && status.value.code === 0 ? Number(status.value.data?.enabled) : undefined;
  if (status.status === 'rejected' || (status.status === 'fulfilled' && status.value.code !== 0))
    error.value += (error.value ? '；' : '') + '开放状态读取失败，请刷新重试。';
  loading.value = false;
}
watch([query, module], () => {
  page.value = 1;
  selectedKey.value = '';
});
onMounted(load);
</script>
<style scoped>
.api-reference-page {
  display: grid;
  gap: 20px;
}
.api-context {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px 28px;
  padding: 18px 20px;
  border: 1px solid var(--hw-line);
  border-radius: 8px;
  background: var(--hw-panel);
}
.api-context > div {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
}
.context-label,
.api-muted,
.api-result-count {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
}
.api-context code {
  overflow-wrap: anywhere;
}
.api-context p {
  width: 100%;
  margin: 0;
  font-size: 13px;
  color: var(--hw-muted);
}
.api-reference-panel {
  border: 1px solid var(--hw-line);
  border-radius: 8px;
  background: var(--hw-panel);
  min-width: 0;
}
.api-filters {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 16px;
  padding: 20px;
  border-bottom: 1px solid var(--hw-line);
}
.api-filters label {
  display: grid;
  gap: 8px;
  min-width: 0;
  width: min(100%, 320px);
}
.api-filters label:nth-child(2) {
  width: 220px;
}
.api-filters label > span {
  font-size: 13px;
  font-weight: 500;
}
.api-result-count {
  padding-bottom: 9px;
  margin-left: auto;
}
.api-reference-layout {
  display: grid;
  grid-template-columns: minmax(240px, 0.85fr) minmax(0, 2fr);
  min-height: 440px;
}
.api-index {
  border-right: 1px solid var(--hw-line);
  max-height: 680px;
  overflow-y: auto;
  padding: 10px;
  min-width: 0;
}
.api-index button {
  display: flex;
  width: 100%;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 10px;
  border: 0;
  border-radius: 5px;
  color: var(--hw-ink);
  background: transparent;
  text-align: left;
  font: inherit;
}
.api-index button:hover,
.api-index button[aria-current] {
  background: var(--hw-accent-soft);
}
.api-index button > span:last-child {
  min-width: 0;
  display: grid;
  gap: 5px;
}
.api-index code {
  font-size: 12px;
  overflow-wrap: anywhere;
}
.api-index small {
  color: var(--hw-muted);
  line-height: 1.5;
}
.api-method {
  font: 600 11px/1.6 var(--font-mono);
  padding: 3px 6px;
  background: var(--hw-subtle);
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  flex-shrink: 0;
}
.api-detail {
  min-width: 0;
  padding: 24px;
}
.api-detail-heading {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.api-detail-heading > code {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 14px;
}
.api-detail h2 {
  font-size: 17px;
  margin: 18px 0 24px;
}
.api-example {
  margin: 24px 0 0;
}
.api-example > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.api-example h3 {
  font-size: 14px;
  margin: 0;
}
.api-example pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 300px;
  overflow: auto;
  padding: 16px;
  background: var(--hw-subtle);
  border: 1px solid var(--hw-line);
  border-radius: 6px;
  font: 12px/1.75 var(--font-mono);
}
.api-help {
  border-top: 1px solid var(--hw-line);
  padding: 16px 0;
}
.api-help summary {
  cursor: pointer;
  font-weight: 500;
}
.api-help p {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.8;
}
.api-empty {
  text-align: center;
  color: var(--hw-muted);
  padding: 48px 24px;
}
@media (max-width: 1000px) {
  .api-reference-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .api-index {
    max-height: 260px;
    border-right: 0;
    border-bottom: 1px solid var(--hw-line);
  }
}
@media (max-width: 600px) {
  .api-filters label,
  .api-filters label:nth-child(2) {
    width: 100%;
  }
  .api-detail {
    padding: 16px;
  }
  .api-result-count {
    margin-left: 0;
  }
}
</style>
