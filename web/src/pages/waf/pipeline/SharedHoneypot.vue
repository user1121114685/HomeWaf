<template>
  <div class="shared-honeypot" data-testid="shared-honeypot" :aria-busy="busy || loading">
    <PageActions
      ><ElButton :disabled="busy" :loading="loading" @click="reload">重新加载</ElButton
      ><ElButton @click="back">返回我的服务</ElButton></PageActions
    >
    <ElAlert v-if="error" :title="error" type="error" :closable="false"
      ><ElButton text :disabled="busy" @click="reload">重试</ElButton></ElAlert
    >
    <ElSkeleton v-if="loading && !draft" :rows="5" animated />
    <template v-if="draft">
      <p class="scope">实例统一配置 · 版本 {{ draft.revision }} · {{ services.length }} 个服务使用</p>
      <details v-if="services.length">
        <summary>查看使用统一蜜罐的服务</summary>
        <ul>
          <li v-for="item in services" :key="item.port">
            <RouterLink :to="{ path: '/waf-pipeline/ports', query: { edit: item.port, tab: 'defense' } }"
              >{{ item.name || '服务' }} · :{{ item.port }}</RouterLink
            >
          </li>
        </ul>
      </details>
      <ElAlert
        v-if="!draft.configured"
        title="首次使用：先保存统一蜜罐，再到服务中选择蜜罐模式。"
        type="info"
        :closable="false"
      />
      <section v-if="legacy.length" class="legacy-import">
        <h2>旧蜜罐配置迁移</h2>
        <p>
          以下服务仍保留自己的旧设置。可选择一份内容填入统一草稿，核对后保存；不会自动覆盖或合并其他服务的不同内容。
        </p>
        <label
          >选择旧服务<ElSelect v-model="sourcePort" :disabled="busy" aria-label="旧蜜罐配置来源"
            ><ElOption
              v-for="item in legacy"
              :key="item.port"
              :value="item.port"
              :label="`${item.name || '服务'} · :${item.port}`" /></ElSelect
        ></label>
        <template v-if="selectedLegacy">
          <p>
            提示 {{ selectedLegacy.config.lures?.length || 0 }} 项 · 陷阱路径
            {{ selectedLegacy.config.trap_paths?.length || 0 }} 项 · 并发上限 {{ selectedLegacy.config.max_concurrent }}
          </p>
          <details>
            <summary>查看所选服务的配置差异</summary>
            <pre>{{ JSON.stringify(selectedLegacy.config, null, 2) }}</pre>
          </details>
          <ElButton :disabled="busy" @click="importLegacy">将所选内容填入草稿</ElButton>
        </template>
        <small
          >只导入内容、陷阱路径、运行限制和归档。旧自动处罚阈值不会导入；原服务在确认迁移前继续按旧设置工作。</small
        >
      </section>
      <section class="settings-panel" :inert="busy">
        <HoneypotSettings :model="draft.config" shared @invalid="invalid = $event" />
      </section>
      <div class="save-row">
        <span>{{ dirty ? '有未保存的修改' : '配置已读取' }}</span
        ><ElButton type="primary" :loading="busy" :disabled="invalid || loading" @click="save"
          >保存统一蜜罐设置</ElButton
        >
      </div>
      <section class="logs-panel">
        <h2>蜜罐最近访问</h2>
        <div class="log-controls">
          <label
            >来源服务<ElSelect v-model="logPort" aria-label="蜜罐记录服务" @change="loadLogs"
              ><ElOption :value="0" label="全部服务" /><ElOption
                v-for="item in allServices"
                :key="item.port"
                :value="item.port"
                :label="`${item.name || '服务'} · :${item.port}`" /></ElSelect></label
          ><ElButton :loading="logsLoading" @click="loadLogs">刷新记录</ElButton>
        </div>
        <ElAlert v-if="logsError" :title="logsError" type="error" :closable="false" />
        <ElSkeleton v-else-if="logsLoading" :rows="3" animated />
        <ElTable v-else :data="logs" empty-text="当前范围内暂无蜜罐记录" :aria-busy="logsLoading"
          ><ElTableColumn label="时间" min-width="170"
            ><template #default="{ row }">{{ new Date(row.time_ms).toLocaleString('zh-CN') }}</template></ElTableColumn
          ><ElTableColumn prop="port_id" label="服务端口" min-width="100" /><ElTableColumn
            prop="client_ip"
            label="来源 IP"
            min-width="180"
          /><ElTableColumn prop="reason" label="触发原因" min-width="180" /><ElTableColumn type="expand"
            ><template #default="{ row }">
              <pre class="log-detail">{{ detail(row.detail_json) }}</pre>
            </template></ElTableColumn
          ></ElTable
        >
        <p>显示最新 20 条。记录延迟或队列过载可能影响展示；不执行提交内容，归档沿用脱敏与保留策略。</p>
      </section>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElSkeleton,
  ElTable,
  ElTableColumn,
} from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import HoneypotSettings from './HoneypotSettings.vue';
import { clone } from './model';
const route = useRoute(),
  router = useRouter();
const draft = ref<any>(null),
  baseline = ref(''),
  legacy = ref<any[]>([]),
  services = ref<any[]>([]),
  sources = ref<any[]>([]),
  logs = ref<any[]>([]);
const sourcePort = ref<number>(),
  logPort = ref(Number(route.query.port) || 0),
  loading = ref(false),
  busy = ref(false),
  invalid = ref(false),
  error = ref(''),
  logsError = ref(''),
  logsLoading = ref(false);
const dirty = computed(() => !!draft.value && JSON.stringify(draft.value) !== baseline.value);
const selectedLegacy = computed(() => legacy.value.find((item) => item.port === sourcePort.value));
const allServices = computed(() => [
  ...new Map([...sources.value, ...services.value, ...legacy.value].map((item) => [item.port, item])).values(),
]);
let live = true,
  loadGeneration = 0,
  logGeneration = 0;
async function mayDiscard() {
  if (!dirty.value) return true;
  try {
    await ElMessageBox.confirm('放弃尚未保存的统一蜜罐草稿？', '未保存的修改', { type: 'warning' });
    return true;
  } catch {
    return false;
  }
}
async function reload() {
  if (loading.value || busy.value || !(await mayDiscard())) return;
  const generation = ++loadGeneration;
  loading.value = true;
  try {
    const value: any = await pipelineGet('honeypot');
    if (!live || generation !== loadGeneration) return;
    draft.value = clone(value.shared);
    baseline.value = JSON.stringify(draft.value);
    legacy.value = value.legacy;
    sources.value = value.sources || [];
    services.value = value.services;
    error.value = '';
  } catch (e: any) {
    if (live) error.value = e.message || '蜜罐设置读取失败';
  } finally {
    if (live && generation === loadGeneration) loading.value = false;
  }
}
async function save() {
  if (busy.value || invalid.value) return;
  busy.value = true;
  error.value = '';
  try {
    await ElMessageBox.confirm(
      `保存会更新 ${services.value.length} 个已选择蜜罐模式的服务。继续保存？`,
      '保存统一蜜罐',
      { type: 'warning' },
    );
    const value = await pipelinePost('honeypot/save', draft.value);
    draft.value = clone(value);
    baseline.value = JSON.stringify(draft.value);
    ElMessage.success('统一蜜罐设置已保存');
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') error.value = e.message || '保存失败，草稿已保留';
  } finally {
    busy.value = false;
  }
}
async function importLegacy() {
  if (!selectedLegacy.value || busy.value) return;
  if (!(await mayDiscard())) return;
  const source = selectedLegacy.value.config;
  for (const key of ['lures', 'trap_paths', 'max_concurrent', 'max_delay_ms', 'archive_body', 'archive_max_bytes'])
    draft.value.config[key] = clone(
      source[key] ?? (['lures', 'trap_paths'].includes(key) ? [] : draft.value.config[key]),
    );
}
async function loadLogs() {
  logs.value = [];
  logsError.value = '';
  const generation = ++logGeneration;
  logsLoading.value = true;
  try {
    const value: any = await pipelineGet('honeypot/logs', { port: logPort.value });
    if (live && generation === logGeneration) {
      logs.value = value;
      logsError.value = '';
    }
  } catch (e: any) {
    if (live && generation === logGeneration) logsError.value = e.message || '蜜罐记录读取失败';
  } finally {
    if (live && generation === logGeneration) logsLoading.value = false;
  }
}
function detail(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return '记录格式不可用';
  }
}
function back() {
  router.push({ path: '/waf-pipeline/ports', query: logPort.value ? { edit: logPort.value, tab: 'defense' } : {} });
}
function unload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
    event.returnValue = '';
  }
}
onBeforeRouteLeave(async () => !busy.value && (await mayDiscard()));
onMounted(() => {
  reload();
  loadLogs();
  window.addEventListener('beforeunload', unload);
});
onBeforeUnmount(() => {
  live = false;
  loadGeneration++;
  logGeneration++;
  window.removeEventListener('beforeunload', unload);
});
</script>
<style scoped>
.shared-honeypot {
  display: grid;
  gap: 24px;
  min-width: 0;
}
.scope,
.shared-honeypot p,
.shared-honeypot small {
  color: var(--hw-muted);
  line-height: 1.7;
}
.shared-honeypot h2 {
  font-size: 18px;
  margin: 0;
}
.legacy-import,
.settings-panel,
.logs-panel {
  padding: 24px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  background: var(--hw-panel);
  min-width: 0;
}
.legacy-import {
  display: grid;
  gap: 16px;
}
.shared-honeypot label {
  display: grid;
  gap: 8px;
  min-width: 180px;
  max-width: 440px;
}
.save-row,
.log-controls {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}
.log-controls {
  justify-content: flex-start;
  margin: 16px 0;
}
.shared-honeypot pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 320px;
  overflow: auto;
  font-size: 13px;
  line-height: 1.6;
}
.log-detail {
  padding: 16px;
}
.shared-honeypot :deep(.el-button) {
  min-height: 44px;
}
@media (max-width: 700px) {
  .legacy-import,
  .settings-panel,
  .logs-panel {
    padding: 16px;
  }
  .save-row > * {
    width: 100%;
  }
  .log-controls label {
    flex: 1;
  }
}
</style>
