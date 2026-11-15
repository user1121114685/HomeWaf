<template>
  <ElDialog
    v-model="visible"
    title="OWASP 高级基线"
    width="min(760px,96vw)"
    :before-close="beforeClose"
    :close-on-click-modal="false"
  >
    <p>
      基线为所有使用当前 CRS
      的服务提供默认参数。用户调参层优先；修改基线不会覆盖已有用户调参，也不会更改引擎的观察/拦截模式。
    </p>
    <ElAlert v-if="error" ref="errorAlert" tabindex="-1" :title="error" type="error" :closable="false" />
    <p v-if="loading" role="status">正在读取基线和用户覆盖值…</p>
    <ElButton v-if="!loaded && !loading" @click="load">重新读取</ElButton>
    <ElForm v-if="loaded" label-position="top" class="baseline-form" @submit.prevent="save">
      <ElFormItem v-for="field in fields" :key="field.key" :label="field.label">
        <ElInputNumber
          v-model="draft[field.key]"
          :min="field.min"
          :max="field.max"
          :precision="0"
          :disabled="saving"
          :aria-label="'基线' + field.label"
        />
        <small>当前用户覆盖值：{{ effective[field.key] ?? '未返回' }}</small>
      </ElFormItem>
      <ElFormItem label="提前拦截"
        ><ElSwitch
          v-model="draft.early_blocking"
          :active-value="1"
          :inactive-value="0"
          :disabled="saving"
          aria-label="基线提前拦截"
        /><small>用户覆盖值：{{ effective.early_blocking === 1 ? '启用' : '关闭' }}</small></ElFormItem
      >
      <ElFormItem label="强制 URLENCODED 请求体处理"
        ><ElSwitch
          v-model="draft.enforce_bodyproc_urlencoded"
          :active-value="1"
          :inactive-value="0"
          :disabled="saving"
          aria-label="基线强制请求体处理"
        /><small>用户覆盖值：{{ effective.enforce_bodyproc_urlencoded === 1 ? '启用' : '关闭' }}</small></ElFormItem
      >
    </ElForm>
    <template #footer
      ><ElButton :disabled="saving" @click="beforeClose(() => (visible = false))">取消</ElButton
      ><ElButton type="primary" :loading="saving" :disabled="!loaded || loading || !dirty" @click="save"
        >保存高级基线</ElButton
      ></template
    >
  </ElDialog>
</template>
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import {
  ElAlert,
  ElButton,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInputNumber,
  ElMessage,
  ElMessageBox,
  ElSwitch,
} from 'element-plus';
import { owaspBaseConfigGetApi, owaspBaseConfigSetApi, owaspTuningGetApi } from '@/apis/owasp';
const visible = ref(false),
  loading = ref(false),
  loaded = ref(false),
  saving = ref(false),
  error = ref('');
const draft = ref<Record<string, number>>({}),
  effective = ref<Record<string, number>>({}),
  original = ref('');
const dirty = computed(() => loaded.value && JSON.stringify(draft.value) !== original.value);
const errorAlert = ref<any>();
watch(error, async (message) => {
  if (message) {
    await nextTick();
    errorAlert.value?.$el?.focus();
  }
});
const fields = [
  { key: 'blocking_paranoia_level', label: '拦截偏执级别', min: 1, max: 4 },
  { key: 'detection_paranoia_level', label: '检测偏执级别', min: 1, max: 4 },
  { key: 'inbound_anomaly_score_threshold', label: '请求异常分阈值', min: 1, max: 1000000 },
  { key: 'outbound_anomaly_score_threshold', label: '响应异常分阈值', min: 1, max: 1000000 },
];
async function load() {
  if (loading.value) return;
  loading.value = true;
  loaded.value = false;
  error.value = '';
  try {
    const [base, tuning] = await Promise.all([owaspBaseConfigGetApi(), owaspTuningGetApi()]);
    if (base.code !== 0 || !base.data) throw new Error(base.msg || '基线读取失败');
    if (tuning.code !== 0 || !tuning.data) throw new Error(tuning.msg || '用户覆盖值读取失败');
    draft.value = { ...base.data };
    effective.value = { ...tuning.data };
    original.value = JSON.stringify(draft.value);
    loaded.value = true;
  } catch (cause: any) {
    error.value = cause.message || '读取失败，请重试。';
  } finally {
    loading.value = false;
  }
}
async function beforeClose(done: () => void) {
  if (saving.value || loading.value) return;
  if (dirty.value) {
    try {
      await ElMessageBox.confirm('放弃尚未保存的基线修改？', '未保存的修改', { type: 'warning' });
    } catch {
      return;
    }
  }
  done();
}
async function save() {
  if (saving.value || !loaded.value) return;
  error.value = '';
  if (draft.value.detection_paranoia_level < draft.value.blocking_paranoia_level) {
    error.value = '检测偏执级别不能低于拦截偏执级别。';
    return;
  }
  saving.value = true;
  try {
    await ElMessageBox.confirm('写入共享 CRS 基线并热重载？已有用户调参仍优先生效。', '保存高级基线', {
      type: 'warning',
    });
    const res = await owaspBaseConfigSetApi(draft.value);
    if (res.code !== 0) throw new Error(res.msg || '保存失败');
    original.value = JSON.stringify(draft.value);
    await load();
    if (loaded.value) ElMessage.success('基线已保存并重读，用户覆盖层保持不变。');
  } catch (cause: any) {
    if (cause !== 'cancel' && cause !== 'close')
      error.value =
        (cause.message || '保存或热重载失败') +
        '。输入已保留。文件可能已写入而运行实例尚未更新；请修正后重试并重新核对。';
  } finally {
    saving.value = false;
  }
}
function open() {
  visible.value = true;
  void load();
}
defineExpose({ open });
</script>
<style scoped>
p {
  color: var(--hw-muted);
  line-height: 1.7;
}
.baseline-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 24px;
  margin-top: 20px;
}
small {
  display: block;
  width: 100%;
  color: var(--hw-muted);
  margin-top: 8px;
}
@media (max-width: 600px) {
  .baseline-form {
    grid-template-columns: 1fr;
  }
}
</style>
