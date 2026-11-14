<template>
  <div class="order-table" :aria-busy="loading">
    <p v-if="loading && !rows.length" class="order-empty" role="status">正在读取证书申请…</p>
    <div v-else-if="!rows.length" class="order-empty">
      <p>{{ filtered ? '没有符合条件的申请' : '还没有证书申请' }}</p>
      <p class="order-muted">
        {{ filtered ? '调整域名或状态筛选后再查询。' : '点击“申请证书”开始，家庭网络推荐 DNS 验证。' }}
      </p>
    </div>
    <table v-else>
      <caption class="sr-only">
        证书申请记录
      </caption>
      <thead>
        <tr>
          <th scope="col">证书域名 / 关联服务</th>
          <th scope="col">申请状态</th>
          <th scope="col">有效期至</th>
          <th scope="col">自动续期</th>
          <th scope="col">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.id" class="ssl-order-row" :data-order-id="row.id">
          <td data-label="证书域名 / 关联服务" class="order-identity">
            <button type="button" class="domain-link" @click="emit('logs', row.id)">
              {{ row.apply_domain || '未填写域名' }}
            </button>
            <p class="order-muted">{{ hostNames[row.host_code] || row.host_code }}</p>
            <p class="order-muted">
              {{ platformName(row.apply_platform) }} · {{ row.apply_method === 'dns01' ? 'DNS 验证' : 'HTTP 验证' }}
            </p>
          </td>
          <td data-label="申请状态">
            <span class="order-status" :class="statusTone(row)">{{
              row.heartbeat_stale ? '心跳中断' : statusNames[row.apply_status] || row.apply_status
            }}</span>
            <p
              v-if="row.progress || row.result_error || row.heartbeat_stale"
              class="order-progress"
              :class="{ 'order-danger': row.apply_status === 'fail' || row.heartbeat_stale }"
              :title="row.result_error || row.progress"
            >
              {{
                row.heartbeat_stale
                  ? '状态未更新，请查看日志'
                  : row.apply_status === 'fail'
                    ? row.result_error || row.progress
                    : row.progress
              }}
            </p>
          </td>
          <td data-label="有效期至">
            <span>{{ expiryDate(row) }}</span>
            <p v-if="expiryNote(row)" class="order-muted">{{ expiryNote(row) }}</p>
          </td>
          <td data-label="自动续期">
            <span :class="row.auto_renew === false ? 'order-muted' : 'renew-enabled'">{{
              row.auto_renew === false ? '已关闭' : '已开启'
            }}</span>
            <p
              v-if="['submitted', 'applying', 'fail'].includes(row.apply_status) && row.auto_renew !== false"
              class="order-muted"
            >
              签发成功后生效
            </p>
          </td>
          <td data-label="操作" class="order-actions-cell">
            <div class="order-actions">
              <button type="button" class="t-button-link" @click="emit('logs', row.id)">日志</button>
              <button
                v-if="row.apply_status === 'fail'"
                type="button"
                class="t-button-link"
                :disabled="!!retryingOrderId"
                @click="emit('retry', row)"
              >
                {{ retryingOrderId === row.id ? '正在提交…' : '重试' }}
              </button>
              <button
                v-if="row.apply_status === 'success'"
                type="button"
                class="t-button-link"
                @click="emit('renew', row)"
              >
                手动续期
              </button>
              <hw-dropdown
                v-if="!['submitted', 'applying'].includes(row.apply_status)"
                trigger="click"
                :options="[{ content: '删除记录', value: 'delete' }]"
                @click="emit('delete', row)"
              >
                <hw-button variant="text" aria-label="更多操作">更多</hw-button>
              </hw-dropdown>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="pagination.total" class="order-pagination">
      <ElPagination
        :current-page="pagination.current"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        :pager-count="5"
        layout="total, prev, pager, next"
        @current-change="emit('page', { current: $event, pageSize: pagination.pageSize })"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue';
import { ElPagination } from 'element-plus';

defineProps({
  rows: { type: Array as PropType<Record<string, any>[]>, default: () => [] },
  hostNames: { type: Object as PropType<Record<string, string>>, default: () => ({}) },
  loading: { type: Boolean, default: false },
  filtered: { type: Boolean, default: false },
  retryingOrderId: { type: String, default: '' },
  pagination: { type: Object as PropType<{ current: number; pageSize: number; total: number }>, required: true },
});
const emit = defineEmits(['logs', 'retry', 'renew', 'delete', 'page']);
const statusNames: Record<string, string> = {
  submitted: '等待处理',
  applying: '申请中',
  success: '已签发',
  fail: '申请失败',
  renewed: '已续期',
  expired: '已过期',
};
const platformName = (value: string) => ({ letsencrypt: "Let's Encrypt", zerossl: 'ZeroSSL' })[value] || value;
const statusTone = (row: Record<string, any>) =>
  row.heartbeat_stale || row.apply_status === 'fail'
    ? 'status-error'
    : row.apply_status === 'success'
      ? 'status-success'
      : ['applying', 'submitted'].includes(row.apply_status)
        ? 'status-pending'
        : 'status-history';
function expiration(row: Record<string, any>) {
  if (!['success', 'renewed', 'expired'].includes(row.apply_status) || !row.result_valid_to) return null;
  const date = new Date(row.result_valid_to);
  return Number.isNaN(date.getTime()) || date.getFullYear() < 2000 ? null : date;
}
function expiryDate(row: Record<string, any>) {
  const date = expiration(row);
  return date ? date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }) : '—';
}
function expiryNote(row: Record<string, any>) {
  const date = expiration(row);
  if (!date) return '';
  const days = Math.ceil((date.getTime() - Date.now()) / 86400000);
  return days <= 0 ? '已到期' : `剩余 ${days} 天`;
}
</script>

<style scoped>
.order-table {
  min-width: 0;
}
table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  text-align: left;
  font-size: 14px;
}
th {
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  font-weight: 500;
}
th,
td {
  padding: 16px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  vertical-align: top;
  overflow-wrap: anywhere;
}
th:nth-child(1) {
  width: 30%;
}
th:nth-child(2) {
  width: 24%;
}
th:nth-child(3) {
  width: 15%;
}
th:nth-child(4) {
  width: 13%;
}
th:nth-child(5) {
  width: 18%;
}
tbody tr:hover {
  background: var(--el-fill-color-lighter);
}
.domain-link {
  padding: 0;
  border: 0;
  background: none;
  text-align: left;
  color: var(--el-text-color-primary);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.domain-link:hover {
  color: var(--el-color-primary);
  text-decoration: underline;
}
.order-muted {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
.order-progress {
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.order-danger {
  color: var(--el-color-danger);
}
.order-status {
  display: inline-block;
  border-radius: 4px;
  padding: 2px 8px;
  font-size: 12px;
  line-height: 1.8;
}
.status-success {
  color: var(--el-color-success);
  background: var(--el-color-success-light-9);
}
.status-error {
  color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
}
.status-pending {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.status-history {
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color);
}
.renew-enabled {
  color: var(--el-color-success);
}
.order-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  column-gap: 12px;
  row-gap: 4px;
}
.order-actions .t-button-link {
  min-height: 32px;
  padding: 0;
  margin: 0;
  white-space: nowrap;
}
.order-actions :deep(.el-button) {
  padding: 0 4px;
}
.order-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
.order-pagination :deep(.el-pagination) {
  flex-wrap: wrap;
  justify-content: flex-end;
  row-gap: 10px;
}
.order-empty {
  padding: 48px 16px;
  text-align: center;
  color: var(--el-text-color-primary);
}
button:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
  border-radius: 3px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
@media (max-width: 1050px) {
  table,
  tbody {
    display: block;
  }
  thead {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  tbody tr {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 16px 24px;
    padding: 20px 0;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }
  td {
    display: block;
    padding: 0;
    border: 0;
  }
  td::before {
    content: attr(data-label);
    display: block;
    margin-bottom: 6px;
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }
  .order-identity {
    grid-column: 1 / -1;
  }
  .order-actions-cell {
    grid-column: 1 / -1;
  }
  .order-actions .t-button-link,
  .order-actions :deep(.el-button) {
    min-height: 44px;
  }
}
@media (max-width: 420px) {
  tbody tr {
    column-gap: 16px;
  }
  .order-pagination {
    justify-content: flex-start;
  }
}
</style>
