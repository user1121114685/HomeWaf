<template>
  <div class="service-policy-page" data-testid="service-policy-page">
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <ElCard shadow="never">
      <div class="service-policy-toolbar">
        <label
          ><span>服务</span
          ><ElSelect
            :model-value="selectedPort"
            :loading="loading"
            :disabled="busy"
            filterable
            placeholder="选择已接入的服务"
            aria-label="服务"
            @update:model-value="selectService"
          >
            <ElOption
              v-for="row in rows"
              :key="row.config.port"
              :value="row.config.port"
              :label="(row.config.name || '未命名服务') + ' · :' + row.config.port"
            /> </ElSelect
        ></label>
        <div class="service-policy-actions">
          <ElButton :loading="loading" :disabled="busy" @click="refresh">刷新</ElButton
          ><ElButton :disabled="!selected || busy" @click="openLogs">访问记录</ElButton>
        </div>
      </div>
      <div v-if="!loading && !rows.length" class="service-policy-empty">
        <h2>先接入一个服务</h2>
        <p>新建群晖、相册或其他服务后，可在这里配置它的防护。</p>
        <ElButton type="primary" @click="router.push('/waf-pipeline/ports')">接入服务</ElButton>
      </div>
      <template v-else-if="selected">
        <div class="service-policy-heading">
          <h2>{{ selected.config.name || '未命名服务' }} · {{ entry.title }}</h2>
          <ElTag :type="selected.listener?.running ? 'success' : 'info'">{{
            selected.listener?.running ? '服务运行中' : '服务未监听'
          }}</ElTag>
        </div>
        <p class="service-policy-hint">{{ entry.hint }}</p>
        <template v-if="entry.tab === 'cc' && draft">
          <Fields :model="draft.policy.ruleset" :definitions="ccFields" />
          <div class="service-policy-save">
            <ElButton type="primary" :loading="busy" :disabled="!dirty" @click="saveCC">保存 CC 防护</ElButton
            ><ElButton :disabled="!dirty || busy" @click="resetDraft">撤销修改</ElButton
            ><span>只更新 {{ draft.name || '该服务' }} 的 CC 策略。</span>
          </div>
        </template>
        <template v-else>
          <p class="service-policy-hint">这里与“服务接入”使用同一份配置，保存后会应用到这个服务。</p>
          <ElButton type="primary" @click="editorVisible = true">配置{{ entry.title }}</ElButton>
        </template>
      </template>
    </ElCard>
    <ElDialog
      v-model="editorVisible"
      :title="entry.title"
      width="min(1400px,97vw)"
      :close-on-click-modal="false"
      :close-on-press-escape="!busy"
      :show-close="!busy"
      destroy-on-close
    >
      <PortEditor
        v-if="editorVisible && selected"
        :key="selected.config.port + '-' + entry.tab"
        :config="selected.config"
        :templates="templates"
        :initial-tab="entry.tab"
        :initial-panels="entry.panels || []"
        @saved="editorSaved"
        @close="editorVisible = false"
        @busy="busy = $event"
      />
    </ElDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router';
import { ElAlert, ElButton, ElCard, ElDialog, ElMessage, ElMessageBox, ElOption, ElSelect, ElTag } from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import { clone, fields, type PortConfig, type PolicyTemplate } from './model';
import Fields from './Fields.vue';
import PortEditor from './PortEditor.vue';
import { servicePolicyEntries } from './policyEntries';
const route = useRoute(),
  router = useRouter();
const entry = computed(() => servicePolicyEntries[String(route.name)]);
const rows = ref<any[]>([]),
  templates = ref<PolicyTemplate[]>([]),
  selectedPort = ref<number>();
const loading = ref(false),
  busy = ref(false),
  error = ref(''),
  editorVisible = ref(false),
  draft = ref<PortConfig>();
const selected = computed(() => rows.value.find((row) => row.config.port === selectedPort.value));
const ccFields = fields.ruleset.filter((field) => field.path.startsWith('cc.'));
const dirty = computed(
  () =>
    !!draft.value &&
    JSON.stringify(draft.value.policy.ruleset.cc) !== JSON.stringify(selected.value?.config.policy.ruleset.cc),
);
function resetDraft() {
  draft.value = selected.value ? clone(selected.value.config) : undefined;
}
async function canDiscard() {
  if (!dirty.value) return true;
  try {
    await ElMessageBox.confirm('CC 配置尚未保存，离开会丢弃这些修改。', '未保存的修改', {
      confirmButtonText: '丢弃修改',
      cancelButtonText: '继续编辑',
      type: 'warning',
    });
    return true;
  } catch {
    return false;
  }
}
async function selectService(port: number) {
  if (await canDiscard()) {
    selectedPort.value = port;
    resetDraft();
  }
}
async function refresh() {
  if (!(await canDiscard())) return;
  loading.value = true;
  error.value = '';
  try {
    const [overview, availableTemplates] = await Promise.all([
      pipelineGet('overview'),
      pipelineGet<PolicyTemplate[]>('templates'),
    ]);
    rows.value = overview.ports || [];
    templates.value = availableTemplates;
    const requested = String(route.query.host_code || '');
    if (!selected.value)
      selectedPort.value =
        rows.value.find((row) => row.config.host_code === requested)?.config.port || rows.value[0]?.config.port;
    resetDraft();
  } catch (e: any) {
    error.value = e.message || '服务配置读取失败，请重试。';
  } finally {
    loading.value = false;
  }
}
async function saveCC() {
  if (!draft.value || !dirty.value) return;
  busy.value = true;
  error.value = '';
  try {
    const config = clone(selected.value.config);
    config.policy.ruleset.cc = clone(draft.value.policy.ruleset.cc);
    const saved = await pipelinePost<PortConfig>('ports/save', { config });
    selected.value.config = saved;
    resetDraft();
    ElMessage.success(saved.listen_enabled ? 'CC 防护已保存并应用到当前服务' : 'CC 防护已保存，服务启用后生效');
  } catch (e: any) {
    error.value = e.message || '保存失败，修改已保留，请重试。';
  } finally {
    busy.value = false;
  }
}
async function editorSaved(config: PortConfig) {
  editorVisible.value = false;
  if (config?.port) selectedPort.value = config.port;
  await refresh();
}
function openLogs() {
  if (selected.value)
    router.push({
      name: 'WafvisitLog',
      query: { host_code: selected.value.config.host_code || 'pipeline-' + selectedPort.value },
    });
}
onBeforeRouteLeave(async () => !busy.value && (await canDiscard()));
onBeforeRouteUpdate(async () => {
  if (busy.value || !(await canDiscard())) return false;
  resetDraft();
  editorVisible.value = false;
  return true;
});
onMounted(refresh);
</script>
<style scoped>
.service-policy-page {
  display: grid;
  gap: 16px;
}
.service-policy-toolbar,
.service-policy-actions,
.service-policy-heading,
.service-policy-save {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.service-policy-toolbar {
  justify-content: space-between;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--hw-line);
}
.service-policy-toolbar label {
  display: grid;
  gap: 8px;
  width: min(100%, 360px);
}
.service-policy-toolbar label > span {
  font-weight: 500;
  font-size: 13px;
}
.service-policy-actions {
  align-self: flex-end;
}
.service-policy-heading {
  margin-top: 24px;
}
.service-policy-heading h2 {
  margin: 0;
  font-size: 18px;
}
.service-policy-hint {
  color: var(--hw-muted);
  font-size: 14px;
  line-height: 1.7;
  margin: 12px 0 24px;
}
.service-policy-save {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--hw-line);
}
.service-policy-save span {
  color: var(--hw-muted);
  font-size: 13px;
}
.service-policy-empty {
  padding: 32px 0;
  color: var(--hw-muted);
}
.service-policy-page :deep(.el-button + .el-button) {
  margin-left: 0;
}
@media (max-width: 520px) {
  .service-policy-toolbar label {
    width: 100%;
  }
  .service-policy-save .el-button {
    flex: 1;
  }
}
</style>
