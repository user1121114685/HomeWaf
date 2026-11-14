<template>
  <div class="service-cards" data-testid="service-cards">
    <article v-for="row in rows" :key="row.config.port" class="service-card">
      <header>
        <span class="service-card-icon"><Connection aria-hidden="true" /></span>
        <div>
          <h2>{{ row.config.name || `服务 ${row.config.port}` }}</h2>
          <span>{{ row.config.protocol.toUpperCase() }} · 端口 {{ row.config.port }}</span>
        </div>
        <ElTag :type="serviceStatus(row).tone as any">{{ serviceStatus(row).label }}</ElTag>
      </header>
      <div class="service-target">
        <span>现有服务地址</span><code>{{ row.config.upstream }}</code>
      </div>
      <div class="service-facts">
        <span>{{
          row.config.policy.gate.require_token
            ? '服务令牌认证'
            : row.config.policy.gate.enabled
              ? '访问前需登录'
              : '访客直接访问'
        }}</span
        ><span
          >今日拦截 <b>{{ row.stats?.today_denied || 0 }}</b> 次</span
        >
      </div>
      <p class="service-next">{{ accessSourceSummary(row.config.policy) }}</p>
      <p v-if="row.listener?.error" class="service-problem" role="alert">{{ row.listener.error }}</p>
      <p v-else-if="!row.config.listen_enabled" class="service-next">
        {{
          row.config.policy.gate.require_token
            ? '下一步：进入配置生成服务令牌，再启用。'
            : '配置已保存，启用后开始接收访问。'
        }}
      </p>
      <p v-else-if="row.locked" class="service-problem">服务已临时锁定，请在访问记录中查看并处理。</p>
      <p v-else class="service-next">通过 HomeWaf 新入口访问，可在记录中确认请求。</p>
      <footer>
        <ElButton @click="$emit('edit', row.config)">配置服务</ElButton
        ><ElButton @click="$emit('honeypot', row.config)">蜜罐设置</ElButton
        ><ElButton v-if="!row.config.listen_enabled" type="primary" plain @click="$emit('enable', row)"
          >启用服务</ElButton
        ><ElButton v-else @click="$emit('inspect', row)">查看记录</ElButton>
        <ElDropdown trigger="click" @command="(command) => action(command, row)"
          ><ElButton aria-label="更多服务操作"><MoreFilled class="more-icon" aria-hidden="true" /></ElButton
          ><template #dropdown
            ><ElDropdownMenu
              ><ElDropdownItem command="inspect">查看状态与记录</ElDropdownItem
              ><ElDropdownItem command="copy">复制服务配置</ElDropdownItem
              ><ElDropdownItem command="template">存为策略模板</ElDropdownItem
              ><ElDropdownItem v-if="row.config.listen_enabled" command="stop" divided>停止服务</ElDropdownItem
              ><ElDropdownItem v-else command="remove" divided>删除服务配置</ElDropdownItem></ElDropdownMenu
            ></template
          ></ElDropdown
        >
      </footer>
    </article>
  </div>
</template>
<script setup lang="ts">
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu, ElTag } from 'element-plus';
import { Connection, MoreFilled } from '@element-plus/icons-vue';
import { serviceStatus } from './onboarding';
import { accessSourceSummary } from './accessPolicy';
defineProps({ rows: { type: Array, required: true } });
const emit = defineEmits(['edit', 'honeypot', 'enable', 'inspect', 'copy', 'template', 'stop', 'remove']);
function action(command: string, row: any) {
  emit(command as any, ['copy', 'template'].includes(command) ? row.config : row);
}
</script>
<style scoped>
.service-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}
.service-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
  padding: 24px;
  min-width: 0;
}
.service-card header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.service-card header > div {
  flex: 1;
  min-width: 0;
}
.service-card h2 {
  font-size: 16px;
  font-weight: 500;
  margin: 0 0 6px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.service-card header > div > span {
  font-size: 13px;
  color: var(--el-text-color-regular);
  font-family: monospace;
}
.service-card-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  border-radius: 8px;
  flex-shrink: 0;
}
.service-card-icon svg {
  width: 22px;
  height: 22px;
}
.service-target {
  padding: 24px 0 20px;
}
.service-target > span {
  display: block;
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 8px;
}
.service-target code {
  font-size: 14px;
  overflow-wrap: anywhere;
  line-height: 1.7;
}
.service-facts {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: var(--el-text-color-regular);
  flex-wrap: wrap;
}
.service-facts b {
  color: var(--el-text-color-primary);
  font-weight: 500;
}
.service-next,
.service-problem {
  font-size: 13px;
  line-height: 1.7;
  margin: 16px 0 0;
  color: var(--el-text-color-regular);
}
.service-problem {
  color: var(--el-color-danger);
  overflow-wrap: anywhere;
}
.service-card footer {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 20px;
  margin-top: 20px;
}
.service-card footer .el-button {
  min-height: 44px;
  margin: 0;
}
.service-card footer .el-dropdown {
  margin-left: auto;
}
.more-icon {
  width: 18px;
  height: 18px;
}
@media (max-width: 1060px) {
  .service-cards {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 600px) {
  .service-card {
    padding: 20px;
  }
  .service-card header {
    flex-wrap: wrap;
  }
  .service-card header .el-tag {
    margin-left: auto;
  }
  .service-card footer .el-button {
    padding-left: 12px;
    padding-right: 12px;
  }
}
</style>
