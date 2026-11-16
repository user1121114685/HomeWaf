<template>
  <div class="service-cards" data-testid="service-cards">
    <article v-for="row in rows" :key="row.config.port" class="service-card">
      <div class="service-card-main">
        <header>
          <span class="service-card-icon"><Connection aria-hidden="true" /></span>
          <div class="service-identity">
            <h2>
              <button @click="emit('edit', row.config)">{{ row.config.name || '服务 ' + row.config.port }}</button>
            </h2>
            <span>{{ row.config.protocol.toUpperCase() }} · :{{ row.config.port }}</span>
          </div>
        </header>
        <div class="service-target">
          <span>现有服务地址</span><code>{{ row.config.upstream }}</code>
        </div>
        <div class="service-status">
          <ElTag :type="serviceStatus(row).tone as any">{{ serviceStatus(row).label }}</ElTag>
          <span
            >今日拦截 <b>{{ row.stats?.today_denied || 0 }}</b> 次</span
          >
        </div>
      </div>
      <div class="service-card-detail">
        <div class="service-policy-summary">
          <span>{{
            row.config.policy.gate.require_token
              ? '服务令牌认证'
              : row.config.policy.gate.enabled
                ? '访问前需登录'
                : '访客直接访问'
          }}</span>
          <span>{{ accessSourceSummary(row.config.policy) }}</span>
        </div>
        <p v-if="row.listener?.error" class="service-problem" role="alert">{{ row.listener.error }}</p>
        <p v-else-if="row.config.defense_state" class="service-problem">{{ defenseNames[row.config.defense_state.mode] }} · {{ new Date(row.config.defense_state.expires_at * 1000).toLocaleString('zh-CN') }} 恢复。<ElButton link @click="emit('honeypot', row.config)">查看与恢复</ElButton></p>
        <p v-else-if="row.locked" class="service-problem">服务已临时锁定，请查看访问记录并处理。</p>
        <p v-if="row.defense?.error" class="service-problem" role="alert">{{ row.defense.error }}</p>
        <p v-else-if="!row.config.listen_enabled" class="service-next">
          {{
            row.config.policy.gate.require_token
              ? '启用前，请在配置中生成服务令牌。'
              : '配置已保存，启用后开始接收访问。'
          }}
        </p>
      </div>
      <footer>
        <span class="service-verify">{{
          row.config.listen_enabled ? '通过新入口访问，在记录中确认实际请求。' : '下一步：确认策略并启用服务。'
        }}</span>
        <div class="service-card-actions">
          <ElButton @click="emit('edit', row.config)">配置服务</ElButton>
          <ElButton @click="emit('inspect', row)">运行诊断</ElButton>
          <ElButton @click="emit('honeypot', row.config)">异常访问处置</ElButton>
          <ElButton v-if="!row.config.listen_enabled" type="primary" @click="emit('enable', row)">启用服务</ElButton>
          <ElButton v-else type="primary" plain @click="emit('logs', row)">访问记录</ElButton>
          <ElDropdown trigger="click" @command="(command) => action(command, row)">
            <ElButton aria-label="更多服务操作"><MoreFilled class="more-icon" aria-hidden="true" /></ElButton>
            <template #dropdown
              ><ElDropdownMenu>
                <ElDropdownItem command="logs">查看访问记录</ElDropdownItem>
                <ElDropdownItem command="copy">复制服务配置</ElDropdownItem>
                <ElDropdownItem command="template">存为策略模板</ElDropdownItem>
                <ElDropdownItem v-if="row.config.listen_enabled" command="stop" divided>停止服务</ElDropdownItem>
                <ElDropdownItem v-else command="remove" divided>删除服务配置</ElDropdownItem>
              </ElDropdownMenu></template
            >
          </ElDropdown>
        </div>
      </footer>
    </article>
  </div>
</template>
<script setup lang="ts">
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu, ElTag } from 'element-plus';
import { Connection, MoreFilled } from '@element-plus/icons-vue';
import type { PropType } from 'vue';
import { serviceStatus } from './onboarding';
import { accessSourceSummary } from './accessPolicy';
defineProps({ rows: { type: Array as PropType<any[]>, required: true } });
const emit = defineEmits(['edit', 'honeypot', 'enable', 'logs', 'inspect', 'copy', 'template', 'stop', 'remove']);
const defenseNames: Record<string, string> = { quiet: '无提示拒绝已生效', restrict: '仅历史成功网段可访问', close: '访问已自动关闭' };
function action(command: string, row: any) {
  emit(command as any, ['copy', 'template'].includes(command) ? row.config : row);
}
</script>
<style scoped>
.service-cards {
  display: grid;
  gap: 16px;
}
.service-card {
  min-width: 0;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
}
.service-card-main {
  display: grid;
  grid-template-columns: minmax(200px, 1fr) minmax(0, 1.5fr) auto;
  align-items: center;
  gap: 24px;
  padding: 20px 24px 12px;
}
header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.service-identity {
  min-width: 0;
}
h2 {
  margin: 0 0 4px;
  font-size: 16px;
  line-height: 1.5;
}
h2 button {
  border: 0;
  padding: 0;
  background: none;
  color: var(--hw-ink);
  font: inherit;
  font-weight: 600;
  text-align: left;
  overflow-wrap: anywhere;
}
h2 button:hover {
  color: var(--hw-accent);
}
.service-identity > span,
.service-target > span,
.service-status > span:not(.el-tag) {
  font-size: 12px;
  color: var(--hw-muted);
}
.service-card-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  color: var(--hw-accent);
}
.service-card-icon svg {
  width: 20px;
  height: 20px;
}
.service-target {
  min-width: 0;
  display: grid;
  gap: 5px;
}
.service-target code {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.service-status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 7px;
}
.service-status b {
  color: var(--hw-ink);
  font-weight: 500;
}
.service-card-detail {
  padding: 0 24px 16px;
}
.service-policy-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  color: var(--hw-muted);
  font-size: 13px;
}
.service-next,
.service-problem {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--hw-muted);
  overflow-wrap: anywhere;
}
.service-problem {
  color: var(--hw-danger);
}
footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 24px;
  border-top: 1px solid var(--hw-line);
}
.service-verify {
  color: var(--hw-muted);
  font-size: 12px;
}
.service-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.more-icon {
  width: 16px;
  height: 16px;
}
@media (max-width: 1100px) {
  .service-card-main {
    grid-template-columns: minmax(0, 1fr) auto;
  }
  .service-target {
    grid-column: 1 / -1;
    grid-row: 2;
  }
}
@media (max-width: 520px) {
  .service-card-main {
    padding: 16px;
    gap: 16px;
  }
  .service-card-detail {
    padding: 0 16px 16px;
  }
  footer {
    padding: 12px 16px;
  }
  .service-verify {
    width: 100%;
  }
  .service-card-actions {
    width: 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 44px;
  }
  .service-card-actions > .el-button--primary { grid-column: 1 / -1; grid-row: 2; }
  .service-card-actions > .el-dropdown { grid-column: 3; grid-row: 1; }
  .service-card-actions :deep(.el-button) { padding-inline: 10px; }
}
</style>
