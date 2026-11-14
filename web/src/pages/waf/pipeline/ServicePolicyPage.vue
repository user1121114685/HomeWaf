<template>
  <div class="service-policy-page" data-testid="service-policy-page">
    <PageActions
      ><ElButton :loading="loading" :disabled="busy" @click="refresh">刷新</ElButton
      ><ElButton :disabled="!selected || busy" @click="openLogs">访问记录</ElButton
      ><ElButton :disabled="!selected || busy" @click="openService">完整服务配置</ElButton></PageActions
    >
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <section class="service-scope-bar">
      <label
        ><span>当前服务</span
        ><ElSelect
          :model-value="selectedPort"
          :loading="loading"
          :disabled="busy"
          filterable
          placeholder="选择已接入的服务"
          aria-label="服务"
          @update:model-value="selectService"
          ><ElOption
            v-for="row in rows"
            :key="row.config.port"
            :value="row.config.port"
            :label="(row.config.name || '未命名服务') + ' · :' + row.config.port" /></ElSelect
      ></label>
      <div v-if="selected" class="service-scope-state">
        <ElTag :type="selected.listener?.running ? 'success' : 'info'">{{
          selected.listener?.running ? '服务运行中' : '服务未监听'
        }}</ElTag
        ><span
          >{{ selected.config.protocol.toUpperCase() }} · 端口 {{ selected.config.port }} · 配置版本
          {{ selected.config.revision }}</span
        >
      </div>
    </section>
    <section v-if="!loading && !rows.length && !error" class="service-policy-empty">
      <h2>先接入一个服务</h2>
      <p>新建群晖、相册或其他服务后，可在这里配置它的防护。</p>
      <ElButton type="primary" @click="router.push('/waf-pipeline/ports')">接入服务</ElButton>
    </section>
    <section v-else-if="selected" class="service-task-panel">
      <header class="service-task-heading">
        <h2>{{ entry.title }}</h2>
        <p>{{ entry.hint }}</p>
      </header>
      <ServiceTaskEditor
        :key="selected.config.port + '-' + route.name + '-' + editorVersion"
        ref="editor"
        :config="selected.config"
        :task="String(route.name)"
        @saved="saved"
        @busy="busy = $event"
      />
    </section>
    <p v-else-if="loading" class="service-policy-loading" role="status">正在读取服务配置…</p>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router';
import { ElAlert, ElButton, ElMessageBox, ElOption, ElSelect, ElTag } from 'element-plus';
import { pipelineGet } from '@/apis/pipeline';
import type { PortConfig } from './model';
import ServiceTaskEditor from './ServiceTaskEditor.vue';
import { servicePolicyEntries } from './policyEntries';
const route = useRoute(),
  router = useRouter();
const entry = computed(() => servicePolicyEntries[String(route.name)]);
const rows = ref<any[]>([]),
  selectedPort = ref<number>();
const loading = ref(false),
  busy = ref(false),
  error = ref(''),
  editorVersion = ref(0),
  editor = ref<InstanceType<typeof ServiceTaskEditor>>();
const selected = computed(() => rows.value.find((row) => row.config.port === selectedPort.value));
let selecting = false;
const identity = (row: any) => row.config.host_code || 'pipeline-' + row.config.port;
async function canDiscard() {
  if (busy.value) return false;
  if (!editor.value?.dirty) return true;
  try {
    await ElMessageBox.confirm(
      '当前配置尚未保存，离开会丢弃这些修改。已即时保存的账号或会话操作不会撤销。',
      '未保存的修改',
      { confirmButtonText: '丢弃修改', cancelButtonText: '继续编辑', type: 'warning' },
    );
    return true;
  } catch {
    return false;
  }
}
async function selectService(port: number) {
  if (port === selectedPort.value || !(await canDiscard())) return;
  const row = rows.value.find((row) => row.config.port === port);
  if (!row) return;
  selecting = true;
  try {
    await router.replace({ query: { ...route.query, host_code: identity(row) } });
    selectedPort.value = port;
    error.value = '';
    editorVersion.value++;
  } finally {
    selecting = false;
  }
}
function resolveSelection() {
  const requested = String(route.query.host_code || '');
  const row = requested ? rows.value.find((row) => identity(row) === requested) : selected.value || rows.value[0];
  selectedPort.value = row?.config.port;
  if (requested && !row) error.value = '请求的服务不存在或不可访问，请重新选择服务。';
}
async function refresh() {
  if (loading.value || !(await canDiscard())) return;
  loading.value = true;
  error.value = '';
  try {
    const overview = await pipelineGet('overview');
    rows.value = overview.ports || [];
    resolveSelection();
    editorVersion.value++;
  } catch (cause: any) {
    error.value = cause.message || '服务配置读取失败，请重试。';
  } finally {
    loading.value = false;
  }
}
function saved(config: PortConfig) {
  if (selected.value) selected.value.config = config;
  busy.value = false;
  editorVersion.value++;
}
function openLogs() {
  if (selected.value) router.push({ name: 'WafvisitLog', query: { host_code: identity(selected.value) } });
}
function openService() {
  if (selected.value) router.push({ name: 'WafPipeline', query: { edit: selectedPort.value } });
}
onBeforeRouteLeave(canDiscard);
onBeforeRouteUpdate(() => selecting || canDiscard());
watch(
  () => route.query.host_code,
  () => {
    if (!selecting) {
      resolveSelection();
      editorVersion.value++;
    }
  },
);
onMounted(refresh);
</script>
<style scoped>
.service-policy-page {
  display: grid;
  gap: 20px;
}
.service-scope-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 20px;
  padding: 18px 20px;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: 8px;
}
.service-scope-bar label {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.service-scope-bar label > span {
  font-size: 13px;
  color: var(--hw-muted);
  flex-shrink: 0;
}
.service-scope-bar .el-select {
  width: 280px;
}
.service-scope-state {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  font-size: 12px;
  color: var(--hw-muted);
}
.service-task-panel {
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: 8px;
  padding: 24px;
  min-width: 0;
}
.service-task-heading {
  padding-bottom: 20px;
  margin-bottom: 24px;
  border-bottom: 1px solid var(--hw-line);
}
.service-task-heading h2 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
}
.service-task-heading p {
  color: var(--hw-muted);
  margin: 8px 0 0;
  line-height: 1.7;
  font-size: 13px;
}
.service-policy-empty,
.service-policy-loading {
  padding: 40px 24px;
  text-align: center;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: 8px;
}
.service-policy-empty h2 {
  font-size: 20px;
}
.service-policy-empty p {
  color: var(--hw-muted);
  margin: 12px 0 24px;
}
@media (max-width: 620px) {
  .service-scope-bar label {
    width: 100%;
    flex-direction: column;
    align-items: stretch;
  }
  .service-scope-bar .el-select {
    width: 100%;
  }
  .service-scope-state {
    margin-left: 0;
  }
  .service-task-panel {
    padding: 16px;
  }
}
</style>
