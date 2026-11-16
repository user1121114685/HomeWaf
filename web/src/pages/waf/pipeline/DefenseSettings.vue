<template>
  <section class="defense-settings" data-testid="defense-settings" :aria-busy="working">
    <h3>异常访问处置 · :{{ config.port }}</h3>
    <ElAlert v-if="error" :title="error" type="error" :closable="false"
      ><ElButton text @click="refresh">重试读取状态</ElButton></ElAlert
    >
    <p v-if="!status && !error" role="status">正在读取当前处置状态…</p>
    <section v-if="status" class="runtime-state" aria-live="polite">
      <template v-if="status.state">
        <strong>{{ modeNames[status.state.mode] }}已生效</strong>
        <span>{{
          status.state.reason === 'no_success_history'
            ? '没有有效成功来源，已自动关闭访问'
            : '安全拒绝与认证失败达到阈值'
        }}</span>
        <span>预计恢复：{{ timeText(status.state.expires_at) }} · {{ remaining }}</span>
        <div class="actions">
          <ElButton :disabled="dirty || busy || working" @click="act('release')">提前恢复</ElButton>
          <ElButton v-if="status.state.mode === 'close'" :disabled="dirty || busy || working" @click="act('close')"
            >保持关闭</ElButton
          >
        </div>
        <small v-if="dirty">先保存或取消草稿，再执行即时恢复操作。</small>
        <small v-if="status.state.mode === 'close'"
          >修改阈值、时长或模式不会改变本次恢复时间。“保持关闭”会取消定时恢复。</small
        >
        <details v-if="status.state.mode === 'restrict'">
          <summary>本轮允许的网段（{{ status.state.prefixes?.length || 0 }}）</summary>
          <ElInput
            :model-value="(status.state.prefixes || []).join('\n')"
            type="textarea"
            :rows="6"
            readonly
            aria-label="本轮允许的网段"
          />
        </details>
      </template>
      <template v-else
        ><strong>{{
          status.legacy
            ? '尚未配置异常访问处置'
            : !status.enabled
              ? '自动处置未启用'
              : status.mode === 'honeypot'
                ? status.honeypot_ready
                  ? '统一蜜罐导流已启用'
                  : '统一蜜罐尚未就绪'
                : '当前未触发自动限制'
        }}</strong
        ><span>本轮累计 {{ status.failures }} 次 · 近 30 天成功网段 {{ status.success_prefixes }} 个</span></template
      >
      <ElAlert v-if="status.error" :title="status.error" type="warning" :closable="false" />
    </section>
    <template v-if="!model">
      <p>选择一种处置方式，设置异常访问达到什么条件后采取行动。</p>
      <ElButton :disabled="busy" @click="configure">配置异常访问处置</ElButton>
    </template>
    <DefensePolicyFields v-else :config="config" :busy="busy" @invalid="emit('invalid', $event)" />
    <p class="hint">
      上面的模式和阈值随“保存端口配置”一起生效；提前恢复、保持关闭是即时操作。持续期间的新拒绝不延长结束时间。
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import type { PropType } from 'vue';
import { ElAlert, ElButton, ElInput, ElMessageBox } from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import type { PortConfig } from './model';
import DefensePolicyFields from './DefensePolicyFields.vue';
import { defenseModeNames as modeNames, newDefensePolicy } from './defensePolicy';
const props = defineProps({
  config: { type: Object as PropType<PortConfig>, required: true },
  busy: Boolean,
  dirty: Boolean,
});
const emit = defineEmits(['invalid', 'committed']);
const model = computed(() => props.config.policy.defense);
const status = ref<any>(null),
  error = ref(''),
  working = ref(false),
  clock = ref(Date.now());
const timeText = (value: number) => new Date(value * 1000).toLocaleString('zh-CN');
const remaining = computed(() => {
  const seconds = Math.max(0, (status.value?.state?.expires_at || 0) - Math.floor(clock.value / 1000));
  return seconds ? `剩余 ${Math.ceil(seconds / 60)} 分钟` : '等待恢复完成';
});
let mounted = true,
  generation = 0,
  timer: ReturnType<typeof setInterval> | undefined;
function configure() {
  props.config.policy.defense = newDefensePolicy();
}
async function refresh() {
  const seq = ++generation;
  try {
    const next = await pipelineGet('defense', { port: props.config.port });
    if (!mounted || seq !== generation) return;
    status.value = next;
    error.value = '';
  } catch (e: any) {
    if (mounted && seq === generation) error.value = e.message || '读取失败';
  }
}
async function act(action: string) {
  if (working.value || props.dirty) return;
  working.value = true;
  try {
    await ElMessageBox.confirm(
      action === 'close' ? '保持此服务关闭，并取消本次自动恢复？' : '解除当前限制并恢复本轮处置前的访问状态？',
      '确认服务访问操作',
      { type: 'warning' },
    );
    const result = await pipelinePost<PortConfig>('ports/access', {
      port: props.config.port,
      revision: status.value.revision,
      state_id: status.value.state?.id || '',
      action,
    });
    emit('committed', result);
  } catch (e: any) {
    if (e !== 'cancel' && e !== 'close') error.value = e.message || String(e);
  } finally {
    working.value = false;
  }
}
onMounted(() => {
  refresh();
  timer = setInterval(() => {
    clock.value = Date.now();
    if (!working.value) refresh();
  }, 5000);
});
onUnmounted(() => {
  mounted = false;
  generation++;
  clearInterval(timer);
});
</script>

<style scoped>
.defense-settings {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.defense-settings h3,
.defense-settings h4,
.defense-settings p {
  margin: 0;
}
.defense-settings p,
.hint {
  color: var(--hw-muted);
  line-height: 1.7;
}
.runtime-state {
  display: grid;
  gap: 12px;
  padding: 16px;
  background: var(--hw-panel-alt, var(--el-fill-color-light));
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
}
.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.defense-settings a {
  color: var(--el-color-primary);
  text-decoration: underline;
}
.defense-settings :deep(.el-button) {
  min-height: 44px;
}
</style>
