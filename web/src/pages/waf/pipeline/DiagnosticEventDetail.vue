<template>
  <div class="event-detail" data-testid="diagnostic-event-detail">
    <dl>
      <div>
        <dt>最后处理步骤</dt>
        <dd>{{ event.stage ?? '未记录' }}</dd>
      </div>
      <div>
        <dt>处理耗时</dt>
        <dd>{{ event.elapsed_us === undefined ? '未记录' : `${event.elapsed_us} μs` }}</dd>
      </div>
      <div>
        <dt>请求标识</dt>
        <dd>{{ event.req_id || '未记录' }}</dd>
      </div>
      <div>
        <dt>命中规则</dt>
        <dd>{{ event.rule_id || detail?.record?.rule_id || '未记录' }}</dd>
      </div>
      <div>
        <dt>事件分类</dt>
        <dd>{{ event.category || detail?.record?.reason || '未记录' }}</dd>
      </div>
    </dl>
    <template v-if="hasTrace">
      <Trace class="desktop-trace" :result="detail" />
      <ol class="mobile-trace">
        <li v-for="(step, index) in detail.trace" :key="index">
          <strong>步骤 {{ step.stage }} · {{ step.check }}</strong
          ><span
            >{{ actionLabels[step.verdict.action] || step.verdict.action
            }}{{ step.verdict.observe ? '（观察）' : '' }}</span
          >
          <p>{{ accessReason(step.verdict.reason || '') }}</p>
          <p v-if="step.verdict.rule_id">规则 {{ step.verdict.rule_id }}</p>
          <p v-if="step.error" class="trace-error">{{ step.error }}</p>
          <small>耗时 {{ step.elapsed_us ?? '未记录' }} μs</small>
        </li>
      </ol>
    </template>
    <p v-else-if="event.kind === 'state'">{{ diagnosticReason(event) }}。这是处罚状态事件，不包含请求检查链。</p>
    <p v-else role="status">此记录没有可用的检查链。可凭请求标识前往访问记录继续排查。</p>
    <details v-if="detail">
      <summary>查看原始事件数据</summary>
      <pre>{{ JSON.stringify(detail, null, 2) }}</pre>
    </details>
    <p v-else class="detail-note">详细数据缺失或格式无法解析，基础事件信息仍可查看。</p>
  </div>
</template>
<script setup lang="ts">
import { computed, type PropType } from 'vue';
import Trace from './Trace.vue';
import { actionLabels, diagnosticDetail, diagnosticReason, type DiagnosticEvent } from './diagnostics';
import { accessReason } from './accessPolicy';
const props = defineProps({ event: { type: Object as PropType<DiagnosticEvent>, required: true } });
const detail = computed(() => diagnosticDetail(props.event));
const hasTrace = computed(
  () =>
    detail.value?.verdict &&
    Array.isArray(detail.value.trace) &&
    detail.value.trace.length > 0 &&
    detail.value.trace.every((step: any) => step && typeof step.verdict === 'object' && step.verdict),
);
</script>
<style scoped>
.mobile-trace {
  display: none;
}
.trace-error {
  color: var(--hw-danger);
}
.event-detail {
  padding: 16px 24px;
  display: grid;
  gap: 16px;
  color: var(--hw-ink);
}
dl {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 32px;
  margin: 0;
}
dt,
.detail-note {
  color: var(--hw-muted);
  font-size: 13px;
}
dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}
p {
  margin: 0;
  line-height: 1.7;
}
summary {
  cursor: pointer;
  padding: 12px 0;
  color: var(--hw-accent);
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: 12px/1.7 var(--font-mono);
}
@media (max-width: 1000px) {
  .desktop-trace {
    display: none;
  }
  .mobile-trace {
    display: grid;
    gap: 16px;
    padding: 0;
    list-style: none;
    margin: 0;
  }
  .mobile-trace li {
    display: grid;
    gap: 8px;
    border-left: 2px solid var(--hw-line);
    padding-left: 12px;
    overflow-wrap: anywhere;
    font-size: 13px;
  }
  .event-detail {
    padding: 12px;
  }
}
</style>
