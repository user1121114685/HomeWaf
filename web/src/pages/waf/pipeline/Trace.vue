<template>
  <div v-if="result" class="decision-trace" data-testid="decision-trace">
    <ElAlert :title="`最终动作：${result.verdict?.action || '未知'} · ${accessReason(result.verdict?.reason || '')}`" :type="result.verdict?.action === 'allow' || result.verdict?.action === 'bypass' ? 'success' : 'info'" :closable="false" />
    <p class="trace-note">请求标识 {{ result.req_id }} <span v-if="result.body_truncated"> · 正文视图达到采样上限，原始转发不被截断</span></p>
    <ElTable :data="result.trace || []" size="small" stripe>
      <ElTableColumn prop="stage" label="步骤" width="64" /><ElTableColumn prop="check" label="检查" min-width="125" />
      <ElTableColumn label="动作" width="105"><template #default="{ row }">{{ row.verdict.action }}{{ row.verdict.observe ? '（观察）' : '' }}</template></ElTableColumn>
      <ElTableColumn label="原因 / 规则" min-width="220"><template #default="{ row }">{{ accessReason(row.verdict.reason) }} <small v-if="row.verdict.rule_id"> · {{ row.verdict.rule_id }}</small><strong v-if="row.error"> · {{ row.error }}</strong></template></ElTableColumn>
      <ElTableColumn prop="elapsed_us" label="耗时 μs" width="95" />
    </ElTable>
  </div>
</template>
<script setup lang="ts">
import { ElAlert, ElTable, ElTableColumn } from 'element-plus';
import { accessReason } from './accessPolicy';
defineProps({result:Object});
</script>
<style scoped>.trace-note{font-size:12px;color:var(--el-text-color-secondary);word-break:break-all}.decision-trace{display:grid;gap:12px}.decision-trace strong{color:var(--el-color-warning)}</style>
