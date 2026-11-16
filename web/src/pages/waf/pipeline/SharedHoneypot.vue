<template>
  <div class="shared-honeypot" data-testid="shared-honeypot" :aria-busy="busy || loading">
    <PageActions
      ><ElButton :disabled="busy" :loading="loading" @click="reload">重新加载</ElButton
      ><ElButton :disabled="busy" @click="back">{{ originPort ? '返回原服务' : '返回我的服务' }}</ElButton></PageActions
    >
    <ElAlert v-if="error" :title="error" type="error" :closable="false"
      ><ElButton text :disabled="busy" @click="reload">重试</ElButton></ElAlert
    >
    <ElSkeleton v-if="loading && !draft" :rows="5" animated />
    <template v-if="draft">
      <div class="setup-intro">
        <p class="intro-copy">
          为被安全策略拒绝的访问准备共用的仿真登录页，并记录其行为。正常访问不导流，保存后还需为服务选择蜜罐模式。
        </p>
        <ol class="setup-progress" aria-label="蜜罐使用步骤">
          <li>
            <strong>准备页面</strong
            ><span>{{ draft.configured ? '内容已保存' : '选择模板并保存' }}{{ dirty ? ' · 有未保存修改' : '' }}</span>
          </li>
          <li>
            <strong>在服务中启用</strong
            ><span>{{ services.length ? services.length + ' 个服务已选择蜜罐模式' : '尚无服务选择蜜罐模式' }}</span>
          </li>
          <li><strong>查看实际记录</strong><span>本地预览不代表已经触发</span></li>
        </ol>
      </div>
      <details v-if="services.length">
        <summary>查看已选择蜜罐模式的服务</summary>
        <ul>
          <li v-for="item in services" :key="item.port">
            <RouterLink :to="{ path: '/waf-pipeline/ports', query: { edit: item.port, tab: 'defense' } }"
              >{{ item.name || '服务' }} · :{{ item.port }}</RouterLink
            >
          </li>
        </ul>
      </details>
      <section class="settings-panel" :inert="busy || loading">
        <HoneypotSettings :model="draft.config" shared @invalid="invalid = $event" />
      </section>
      <details v-if="legacy.length" class="legacy-import">
        <summary>复用已有蜜罐内容（可选）</summary>
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
          >只导入内容、陷阱路径、运行限制和归档。旧自动处罚阈值不会导入；服务保存所选处置方式后，才会使用统一蜜罐。</small
        >
      </details>
      <div class="save-row">
        <div>
          <strong role="status">{{
            dirty ? '有未保存的修改' : draft.configured ? '配置已保存' : '尚未保存 · 可直接使用默认设置'
          }}</strong
          ><small>{{
            services.length
              ? '保存后将更新已选择蜜罐模式的 ' + services.length + ' 个服务。'
              : '保存后，请继续在服务中选择蜜罐模式。'
          }}</small
          ><small v-if="invalid" class="form-error">请先修正上方标出的填写问题。</small>
        </div>
        <ElButton type="primary" :loading="busy" :disabled="invalid || loading" @click="save"
          >保存统一蜜罐设置</ElButton
        >
      </div>
      <ElAlert v-if="saveError" :title="saveError" type="error" :closable="false"
        ><span>草稿已保留。可重试保存；若提示版本冲突，先保留自己的修改，再重新加载核对。</span></ElAlert
      >
      <section class="next-step">
        <h2>2. 在服务中选择蜜罐模式</h2>
        <p v-if="originPort">
          你从端口 :{{ originPort }}
          的服务配置进入。若是在新标签页打开，请回到原标签页点击“重新检查蜜罐”，原服务草稿仍在那里。
        </p>
        <p>
          进入服务的“异常访问处置”，启用处置并选择“蜜罐”，再保存端口配置。服务需启用监听；这里的保存只准备共享页面。
        </p>
        <div class="service-target">
          <label
            >要设置的服务<ElSelect v-model="targetPort" aria-label="要设置蜜罐的服务" placeholder="选择服务"
              ><ElOption
                v-for="item in allServices"
                :key="item.port"
                :value="item.port"
                :label="`${item.name || '服务'} · :${item.port}`" /></ElSelect
          ></label>
          <ElButton :disabled="busy || loading || !draft.configured || !targetPort" @click="openService"
            >前往服务设置</ElButton
          >
        </div>
        <p v-if="!draft.configured">先保存上面的蜜罐内容，才能为服务选择此模式。</p>
        <p v-else-if="!allServices.length">当前没有可选服务，可返回“我的服务”接入服务。</p>
      </section>
      <section class="logs-panel">
        <h2>3. 查看蜜罐最近访问</h2>
        <details class="verification-help">
          <summary>如何确认蜜罐实际生效？</summary>
          <ol>
            <li>确认目标服务已保存蜜罐模式并启用监听。</li>
            <li>如需主动测试，可添加一个所有关联服务都未使用的临时陷阱路径，保存后从业务地址访问该路径。</li>
            <li>应看到仿真登录页；再选择该服务并刷新下面的记录。测试后可删除临时路径并保存。</li>
          </ol>
          <p>普通正常访问不会自动进入蜜罐。这里只展示最近 20 条；没有记录不等于配置失败。</p>
        </details>
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
        <ElAlert v-if="logsError" :title="logsError" type="error" :closable="false"
          ><ElButton text @click="loadLogs">重试读取记录</ElButton></ElAlert
        >
        <ElSkeleton v-else-if="logsLoading" :rows="3" animated />
        <ElTable
          v-else
          :data="logs"
          empty-text="当前范围内暂无蜜罐记录；请确认服务设置，或稍后刷新。"
          :aria-busy="logsLoading"
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
const originPort = Number(route.query.port) || 0;
const targetPort = ref<number | undefined>(originPort || undefined);
const draft = ref<any>(null),
  baseline = ref(''),
  legacy = ref<any[]>([]),
  services = ref<any[]>([]),
  sources = ref<any[]>([]),
  logs = ref<any[]>([]);
const sourcePort = ref<number>(),
  logPort = ref(originPort),
  loading = ref(false),
  busy = ref(false),
  invalid = ref(false),
  error = ref(''),
  saveError = ref(''),
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
    saveError.value = '';
  } catch (e: any) {
    if (live) error.value = e.message || '蜜罐设置读取失败';
  } finally {
    if (live && generation === loadGeneration) loading.value = false;
  }
}
async function save() {
  if (busy.value || loading.value || invalid.value || !draft.value) return;
  busy.value = true;
  saveError.value = '';
  try {
    await ElMessageBox.confirm(
      services.value.length
        ? `保存会更新 ${services.value.length} 个已选择蜜罐模式的服务。继续保存？`
        : '保存共享蜜罐页面？保存后还需要到服务中选择蜜罐模式，不会自动启用任何服务。',
      '保存统一蜜罐',
      { type: 'warning' },
    );
    const value = await pipelinePost('honeypot/save', draft.value);
    draft.value = clone(value);
    baseline.value = JSON.stringify(draft.value);
    ElMessage.success('统一蜜罐设置已保存');
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') saveError.value = e.message || '保存失败，草稿已保留';
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
  router.push({ path: '/waf-pipeline/ports', query: originPort ? { edit: originPort, tab: 'defense' } : {} });
}
function openService() {
  if (targetPort.value) router.push({ path: '/waf-pipeline/ports', query: { edit: targetPort.value, tab: 'defense' } });
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
.next-step,
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
.service-target,
.log-controls {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}
.save-row {
  padding: 16px 24px;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
}
.save-row strong {
  font-size: 14px;
}
.save-row small {
  display: block;
  margin-top: 4px;
  font-size: 13px;
}
.save-row .form-error {
  color: var(--el-color-danger);
}
.intro-copy {
  margin-top: 0;
}
.setup-progress {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
  padding: 0 0 0 24px;
  margin: 20px 0;
}
.setup-progress li {
  padding-left: 4px;
}
.setup-progress strong,
.setup-progress span {
  display: block;
}
.setup-progress strong {
  font-size: 14px;
}
.setup-progress span {
  margin-top: 6px;
  font-size: 13px;
  color: var(--hw-muted);
}
.service-target {
  justify-content: flex-start;
  align-items: flex-end;
}
.shared-honeypot summary {
  cursor: pointer;
  min-height: 44px;
  align-content: center;
  line-height: 1.7;
}
.shared-honeypot summary:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
.verification-help {
  margin-top: 12px;
}
.verification-help ol {
  color: var(--hw-muted);
  line-height: 1.8;
  padding-left: 24px;
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
  .next-step,
  .logs-panel {
    padding: 16px;
  }
  .save-row > * {
    width: 100%;
  }
  .log-controls label {
    flex: 1;
  }
  .setup-progress {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    padding-left: 0;
  }
  .save-row {
    padding: 12px 16px;
    gap: 8px;
  }
  .service-target > * {
    width: 100%;
  }
}
</style>
