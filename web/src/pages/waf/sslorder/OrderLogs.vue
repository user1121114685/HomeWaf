<template>
  <hw-dialog
    class="ssl-order-log-dialog"
    modal-class="ssl-order-log-overlay"
    append-to-body
    header="申请与续期任务详情"
    :visible="visible"
    @update:visible="emit('update:visible', $event)"
    :width="920"
  >
    <template #body>
      <div class="order-logs" data-testid="ssl-order-logs">
        <p v-if="error" role="alert" class="log-error">{{ error }}</p>
        <p v-if="loading && !detail" role="status">正在读取申请状态和日志…</p>
        <template v-if="detail">
          <dl class="log-summary">
            <div>
              <dt>申请域名</dt>
              <dd>{{ detail.apply_domain }}</dd>
            </div>
            <div>
              <dt>申请状态</dt>
              <dd>
                {{
                  detail.heartbeat_stale
                    ? '心跳中断（等待确认）'
                    : statusLabels[detail.apply_status] || detail.apply_status
                }}
              </dd>
            </div>
            <div>
              <dt>处理阶段</dt>
              <dd>{{ detail.progress || '等待任务处理' }}</dd>
            </div>
            <div>
              <dt>最近状态更新</dt>
              <dd>{{ detail.update_time || '—' }}</dd>
            </div>
          </dl>
          <p class="log-order-id">订单 ID：{{ detail.id }}</p>
          <section v-if="detail.notifications" class="log-config">
            <h3>本次任务通知</h3>
            <p v-if="!detail.notifications.operation_subscriptions">
              没有启用的操作通知渠道；到期提醒不会替代申请/续期失败通知。
            </p>
            <div v-for="delivery in detail.notifications.deliveries" :key="delivery.id">
              <p>
                {{ delivery.channel_name }} · {{ notifyStates[delivery.status] || delivery.status
                }}<span v-if="delivery.error">：{{ delivery.error }}</span>
              </p>
              <p v-if="delivery.status === 'retry_wait'">下次投递：{{ showTime(delivery.next_attempt_at) }}</p>
            </div>
            <p v-if="detail.notifications.events.some((event: any) => event.coverage === 'no_subscription')">
              通知事件已记录，但没有对应订阅。
            </p>
          </section>

          <section v-if="detail.renewal" class="log-config">
            <h3>当前恢复安排</h3>
            <p>
              {{ detail.renewal.enabled ? '自动续期已开启' : '自动续期已关闭' }} ·
              {{ renewalStates[detail.renewal.status] || detail.renewal.status }}
            </p>
            <p v-if="detail.renewal.last_error" class="log-error">{{ detail.renewal.last_error }}</p>
            <p>下次尝试：{{ showTime(detail.renewal.next_attempt_at) }}</p>
            <RouterLink :to="'/waf-host/wafsslconfiglist/' + detail.certificate_id"
              >查看证书、续期设置与通知结果</RouterLink
            >
          </section>
          <div v-if="detail.deployment_result">
            <h3>各使用位置的更新结果</h3>
            <ul>
              <li v-for="(item, index) in deploymentResults" :key="index">
                {{ item.name }}：{{
                  item.status === 'updated'
                    ? '配置已更新'
                    : item.status === 'detached'
                      ? '已解除绑定，未覆盖'
                      : '更新失败'
                }}
                {{ item.error }}
              </li>
            </ul>
          </div>

          <details class="log-config">
            <summary>申请配置</summary>
            <dl class="log-summary">
              <div>
                <dt>提交时的续期选项</dt>
                <dd>{{ detail.auto_renew === false ? '已关闭' : '已开启，签发成功后生效' }}</dd>
              </div>
              <div>
                <dt>证书机构</dt>
                <dd>{{ detail.apply_platform || '—' }}</dd>
              </div>
              <div>
                <dt>验证方式</dt>
                <dd>
                  {{
                    detail.apply_method === 'dns01' ? 'DNS 验证' : detail.apply_method === 'http01' ? 'HTTP 验证' : '—'
                  }}
                </dd>
              </div>
              <div v-if="detail.apply_method === 'dns01'">
                <dt>DNS 服务商</dt>
                <dd>{{ detail.apply_dns || '—' }}</dd>
              </div>
              <div>
                <dt>申请邮箱</dt>
                <dd>{{ detail.apply_email || '—' }}</dd>
              </div>
              <div v-if="detail.apply_method === 'dns01'">
                <dt>密钥分组</dt>
                <dd>{{ detail.private_group_name || '—' }}</dd>
              </div>
              <div>
                <dt>申请时间</dt>
                <dd>{{ detail.create_time || '—' }}</dd>
              </div>
              <div v-if="detail.result_valid_to">
                <dt>有效期至</dt>
                <dd>{{ detail.result_valid_to }}</dd>
              </div>
            </dl>
          </details>
          <p v-if="detail.heartbeat_stale" role="alert" class="log-error">
            处理进程已超过 90 秒没有更新心跳，可能已经中断。系统会在超过 10 分钟无心跳后标记失败，届时可以重试。
          </p>
          <pre v-if="detail.result_error" role="alert" class="log-error log-message">{{ detail.result_error }}</pre>
          <p v-if="detail.apply_status === 'fail'" class="log-help">
            修正日志中的问题后可直接重试，沿用本次配置并保留失败记录。申请过程中请保持服务运行，避免重启。
          </p>
          <p v-if="detail.legacy" class="log-help">
            此订单创建时尚未启用逐单日志。以下历史内容按域名和时间从运行日志中筛选，可能不完整。
          </p>
          <ol v-if="detail.entries.length" class="log-entries" role="log" aria-label="申请过程日志">
            <li v-for="(entry, index) in detail.entries" :key="entry.id || `legacy-${index}`">
              <div class="log-meta">
                <time>{{ entry.time }}</time
                ><span>{{ levelLabels[entry.level] || entry.level }}</span>
              </div>
              <pre class="log-message">{{ entry.message }}</pre>
            </li>
          </ol>
          <p v-else class="log-help">暂无可读取的过程日志。排队状态、失败原因和最近更新时间仍会显示在上方。</p>
          <p class="log-help">
            保留最近 {{ detail.limit }} 条日志，密钥和认证参数会脱敏。等待中、申请中的订单每 3 秒刷新。
          </p>
        </template>
      </div>
    </template>
    <template #footer>
      <div class="log-actions">
        <hw-button variant="outline" :loading="loading" @click="loadLogs()">刷新日志</hw-button>
        <hw-button
          v-if="detail?.apply_status === 'fail'"
          theme="primary"
          :loading="retrying"
          :disabled="retrying"
          @click="emit('retry', detail.id)"
          >重试申请</hw-button
        >
        <hw-button variant="outline" @click="emit('update:visible', false)">关闭</hw-button>
      </div>
    </template>
  </hw-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { renewalStates, showTime } from '@/apis/certificates';
import { sslOrderLogsApi } from '@/apis/sslorder';

interface LogDetail {
  notifications?: { operation_subscriptions: number; events: any[]; deliveries: any[] };
  certificate_id?: string;
  renewal?: Record<string, any>;
  deployment_result?: string;
  auto_renew?: boolean;
  apply_platform?: string;
  apply_method?: string;
  apply_dns?: string;
  apply_email?: string;
  private_group_name?: string;
  create_time?: string;
  result_valid_to?: string;
  id: string;
  apply_status: string;
  apply_domain: string;
  progress: string;
  update_time: string;
  result_error: string;
  heartbeat_stale: boolean;
  legacy: boolean;
  limit: number;
  entries: { id: number; time: string; level: string; message: string }[];
}

const props = defineProps({
  visible: { type: Boolean, default: false },
  orderId: { type: String, required: true },
  retrying: { type: Boolean, default: false },
});
const emit = defineEmits(['update:visible', 'retry']);
const detail = ref<LogDetail | null>(null);
const deploymentResults = computed(() => {
  try {
    return JSON.parse(detail.value?.deployment_result || '[]');
  } catch {
    return [];
  }
});
const error = ref('');
const loading = ref(false);
const notifyStates: Record<string, string> = {
  pending: '等待投递',
  sending: '正在投递',
  sent: '渠道已接收',
  retry_wait: '发送失败，等待重试',
  failed: '发送失败',
  filtered: '被通知规则过滤',
  disabled: '渠道或订阅未启用',
};
const statusLabels: Record<string, string> = {
  submitted: '已提交',
  applying: '申请中',
  success: '已成功',
  fail: '已失败',
  renewed: '已续期',
  expired: '已过期',
};
const levelLabels: Record<string, string> = { info: '信息', warn: '警告', error: '错误' };
let sequence = 0;
let timer: ReturnType<typeof setInterval> | undefined;

async function loadLogs(quiet = false) {
  if (!props.visible || !props.orderId || (quiet && loading.value)) return;
  const current = ++sequence;
  loading.value = true;
  try {
    const result = await sslOrderLogsApi(props.orderId);
    if (current !== sequence) return;
    if (result.code !== 0) throw new Error(result.msg || '日志读取失败，请重试');
    detail.value = { ...result.data, entries: result.data.entries || [] };
    error.value = '';
  } catch (cause) {
    if (current === sequence) error.value = cause instanceof Error ? cause.message : '日志读取失败，请检查网络后重试';
  } finally {
    if (current === sequence) loading.value = false;
  }
}

watch(
  () => [props.visible, props.orderId] as const,
  ([visible]) => {
    clearInterval(timer);
    sequence++;
    detail.value = null;
    loading.value = false;
    error.value = '';
    if (!visible) return;
    void loadLogs();
    timer = setInterval(() => {
      if (detail.value && ['submitted', 'applying'].includes(detail.value.apply_status)) void loadLogs(true);
    }, 3000);
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  clearInterval(timer);
  sequence++;
});
</script>

<style scoped>
:global(.ssl-order-log-dialog.el-dialog) {
  display: flex;
  flex-direction: column;
  max-height: calc(100dvh - 12vh);
  margin-bottom: 0;
  overflow: hidden;
}
:global(.ssl-order-log-dialog .el-dialog__header),
:global(.ssl-order-log-dialog .el-dialog__footer) {
  flex: 0 0 auto;
}
:global(.ssl-order-log-dialog .el-dialog__body) {
  flex: 1 1 auto;
  min-height: 0;
  max-height: none;
  overflow-y: auto;
  overscroll-behavior: contain;
}
:global(.ssl-order-log-overlay .el-overlay-dialog) {
  overflow: hidden;
}
.order-logs {
  min-width: 0;
}
.log-summary {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
  margin: 0;
}
.log-summary dt,
.log-help,
.log-meta,
.log-order-id {
  color: var(--el-text-color-secondary);
}
.log-summary dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}
.log-order-id {
  overflow-wrap: anywhere;
}
.log-config {
  margin-block: 12px 16px;
}
.log-config summary {
  cursor: pointer;
  color: var(--el-color-primary);
  padding-block: 8px;
}
.log-config[open] summary {
  margin-bottom: 12px;
}
.log-config summary:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.log-error {
  color: var(--el-color-danger);
}
.log-message {
  margin: 4px 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: inherit;
  line-height: 1.6;
}
.log-entries {
  list-style: none;
  padding: 0;
  margin: 16px 0;
  border-block: 1px solid var(--el-border-color-lighter);
}
.log-entries li {
  padding: 10px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.log-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
}
.log-help {
  font-size: 12px;
  line-height: 1.6;
}
.log-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
}
.log-actions :deep(.el-button) {
  margin-left: 0;
}
@media (max-width: 600px) {
  :global(.ssl-order-log-dialog.el-dialog) {
    max-height: calc(100dvh - 6vh);
  }
  .log-summary {
    grid-template-columns: 1fr;
  }
  .log-actions {
    justify-content: flex-start;
  }
}
</style>
