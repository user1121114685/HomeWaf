<template>
  <section class="defense-settings" data-testid="defense-settings" :aria-busy="working">
    <h3>异常访问处置 · :{{ config.port }}</h3>
    <p>按当前端口跨 IP 累计安全拒绝和认证失败。同一请求只算一次；正常访问不清零，普通 404、500 和回源故障不计入。</p>
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
            ? '沿用旧端口处置，尚未迁移'
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
      <ElAlert v-if="capabilities?.runtime_error" :title="capabilities.runtime_error" type="error" :closable="false" />
    </section>
    <template v-if="!model">
      <ElAlert title="此服务仍使用旧处置设置，尚未迁移" type="info" :closable="false" />
      <p>迁移后改为四选一，旧蜜罐丢弃阈值和多来源自保不再同时触发；单来源封禁限于本端口。现有名单与检测规则保留。</p>
      <ElButton :disabled="busy" @click="configure">配置新处置模式</ElButton>
    </template>
    <template v-else>
      <label class="switch-row"
        ><span>启用异常访问处置</span><ElSwitch v-model="model.enabled" :disabled="busy" aria-label="启用异常访问处置"
      /></label>
      <label
        >处置模式
        <ElSelect v-model="model.mode" :disabled="busy" aria-label="异常访问处置模式">
          <ElOption v-for="(label, key) in modeNames" :key="key" :value="key" :label="label" />
        </ElSelect>
      </label>
      <p>{{ descriptions[model.mode] }}</p>
      <div v-if="model.mode !== 'honeypot'" class="defense-grid">
        <label
          >统计窗口（分钟）<ElInputNumber
            :model-value="model.window_seconds / 60"
            :min="1"
            :max="1440"
            :disabled="busy"
            aria-label="统计窗口（分钟）"
            @update:model-value="model.window_seconds = Number($event) * 60"
        /></label>
        <label
          >触发次数<ElInputNumber v-model="model.count" :min="1" :max="100000" :disabled="busy" aria-label="触发次数"
        /></label>
        <label
          >持续时间（小时）<ElInputNumber
            :model-value="model.duration_seconds / 3600"
            :min="1"
            :max="168"
            :disabled="busy"
            aria-label="持续时间（小时）"
            @update:model-value="model.duration_seconds = Number($event) * 3600"
        /></label>
      </div>
      <template v-if="model.mode === 'quiet'">
        <label
          >TCP 拒绝方式<ElSelect v-model="model.tcp_action" :disabled="busy" aria-label="TCP拒绝方式"
            ><ElOption value="reset" label="立即断开／复位连接" /><ElOption
              value="drop"
              label="静默丢包"
              :disabled="!capabilities?.drop_supported" /></ElSelect
        ></label>
        <p>正常新 IP 和正常登录入口仍可访问。需要检查请求内容的访问，收到请求后才判断并终止连接。</p>
        <ElAlert
          v-if="config.policy.network.trusted_proxies?.length"
          title="当前配置了前置代理，无法按真实访客执行 TCP 拒绝，请选择其他模式。"
          type="warning"
          :closable="false"
        />
        <p v-if="capabilities && !capabilities.drop_supported">{{ capabilities.drop_reason }}</p>
      </template>
      <template v-if="model.mode === 'restrict'">
        <p>
          参考触发时最近 30 天的正常业务成功来源：IPv4 固定 /16，IPv6 固定
          /64。范围内仍执行原有安全检查；限制期间不自动扩大范围。
        </p>
        <ElAlert
          title="没有有效历史时会关闭整个服务访问，并在设定时长后恢复。记录读取失败不会当成空历史。"
          type="warning"
          :closable="false"
        />
      </template>
      <p v-if="model.mode === 'close'">
        关闭现有访问开关，包括可信来源在内的业务用户均无法访问；管理控制台不受影响。沿用当前服务的连接排空设置。
      </p>
      <p v-if="model.mode === 'honeypot'">
        <a :href="honeyLink" target="_blank" rel="noopener">在新标签页配置统一蜜罐</a
        >，当前草稿会留在这里。保存统一设置后即可使用。
      </p>
      <div v-if="migrationNeeded" class="migration" data-testid="defense-migration">
        <h4>迁移确认</h4>
        <p>
          旧自保：{{ original.policy.hit_policy.lockdown.enabled ? '开启' : '关闭' }}；旧蜜罐：{{
            original.policy.honeypot.enabled ? '开启' : '关闭'
          }}；原封禁范围：{{ original.policy.hit_policy.ban_scope === 'global' ? '全局' : '本端口' }}。
        </p>
        <p>
          旧蜜罐内容可先在统一蜜罐页面选择并导入。保存后，旧端口锁定和蜜罐丢弃记录不再作用于此服务；IP
          封禁记录仍按原有效期处理。
        </p>
        <a :href="honeyLink" target="_blank" rel="noopener">查看旧蜜罐内容与统一设置</a>
        <ElCheckbox v-model="confirmed" :disabled="busy">我已核对差异，确认用新模式替代旧端口处置</ElCheckbox>
      </div>
    </template>
    <p class="hint">
      上面的模式和阈值随“保存端口配置”一起生效；提前恢复、保持关闭是即时操作。持续期间的新拒绝不延长结束时间。
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElInput,
  ElInputNumber,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElSwitch,
} from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import type { PortConfig } from './model';
const props = defineProps({
  config: { type: Object as PropType<PortConfig>, required: true },
  original: { type: Object as PropType<PortConfig>, required: true },
  busy: Boolean,
  dirty: Boolean,
});
const emit = defineEmits(['invalid', 'migration', 'committed']);
const model = computed(() => props.config.policy.defense);
const status = ref<any>(null),
  capabilities = ref<any>(null),
  error = ref(''),
  working = ref(false),
  confirmed = ref(false),
  clock = ref(Date.now());
const modeNames: Record<string, string> = {
  quiet: '无提示拒绝',
  restrict: '收紧 IP 访问',
  close: '自动关闭访问',
  honeypot: '蜜罐',
};
const descriptions: Record<string, string> = {
  quiet: '达到阈值后，未通过检查的访问不再收到提示页面。',
  restrict: '达到阈值后，仅近期成功来源所在的网段可进入后续检测。',
  close: '达到阈值后关闭服务访问，到期自动打开。',
  honeypot: '第一次安全拒绝即导入统一蜜罐，不等待触发阈值。',
};
const migrationNeeded = computed(() => !props.original.policy.defense && !!model.value);
const honeyLink = computed(() => useRouterLink(props.config.port));
const router = useRouter();
function useRouterLink(port: number) {
  return router.resolve({ path: '/waf-pipeline/honeypot', query: { port } }).href;
}
const timeText = (value: number) => new Date(value * 1000).toLocaleString('zh-CN');
const remaining = computed(() => {
  const seconds = Math.max(0, (status.value?.state?.expires_at || 0) - Math.floor(clock.value / 1000));
  return seconds ? `剩余 ${Math.ceil(seconds / 60)} 分钟` : '等待恢复完成';
});
let mounted = true,
  generation = 0,
  timer: ReturnType<typeof setInterval> | undefined;
function configure() {
  props.config.policy.defense = {
    enabled: true,
    mode: 'quiet',
    window_seconds: 1800,
    count: 10,
    duration_seconds: 10800,
    tcp_action: 'reset',
  };
}
watch([migrationNeeded, confirmed], () => emit('migration', !migrationNeeded.value || confirmed.value), {
  immediate: true,
});
watch(
  () => [
    model.value?.enabled,
    model.value?.mode,
    model.value?.tcp_action,
    migrationNeeded.value,
    confirmed.value,
    capabilities.value,
    props.config.policy.network.trusted_proxies?.length,
  ],
  () => {
    const unavailable =
      model.value?.enabled &&
      model.value.mode === 'quiet' &&
      (props.config.policy.network.trusted_proxies?.length ||
        (model.value.tcp_action === 'drop' && !capabilities.value?.drop_supported));
    emit('invalid', !!unavailable || (migrationNeeded.value && !confirmed.value));
  },
  { immediate: true },
);
async function refresh() {
  const seq = ++generation;
  try {
    const [next, caps] = await Promise.all([
      pipelineGet('defense', { port: props.config.port }),
      pipelineGet('defense/capabilities', { ip_version: props.config.policy.network.listen_ip_version || '' }),
    ]);
    if (!mounted || seq !== generation) return;
    status.value = next;
    capabilities.value = caps;
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
.defense-settings label:not(.el-checkbox) {
  display: grid;
  gap: 8px;
}
.defense-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.defense-grid :deep(.el-input-number) {
  width: 100%;
}
.switch-row {
  display: flex !important;
  align-items: center;
  justify-content: space-between;
}
.runtime-state,
.migration {
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
.migration :deep(.el-checkbox) {
  height: auto;
  white-space: normal;
}
.migration :deep(.el-checkbox__label) {
  white-space: normal;
  line-height: 1.6;
}
@media (max-width: 700px) {
  .defense-grid {
    grid-template-columns: 1fr;
  }
  .actions .el-button {
    flex: 1;
  }
}
</style>
