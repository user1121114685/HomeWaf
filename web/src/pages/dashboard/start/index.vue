<template>
  <main id="workspace-main" class="workbench" tabindex="-1" data-testid="workbench">
    <header class="workbench-heading">
      <div>
        <div class="workbench-eyebrow">HomeWaf 控制台</div>
        <h1>网站与家庭服务防护</h1>
        <p>从接入一个服务开始，在这里确认状态和下一步。</p>
      </div>
      <ElButton :loading="loading" @click="load"><Refresh class="button-icon" aria-hidden="true" />刷新状态</ElButton>
    </header>
    <ElAlert v-if="error" type="error" :title="error" :closable="false" show-icon class="workbench-alert" />
    <template v-if="!loaded && loading"><div class="workbench-loading" role="status">正在读取服务状态…</div></template>
    <template v-else-if="loaded">
      <section class="welcome-panel">
        <div class="welcome-copy">
          <span class="section-kicker">{{ ports.length ? '接入进度' : '第一次使用' }}</span>
          <h2>
            {{
              ports.length ? (pending.length ? '还有服务等待启用' : '服务已接入，查看运行情况') : '先把你的服务接进来'
            }}
          </h2>
          <p>
            {{
              ports.length
                ? pending.length
                  ? `已保存 ${ports.length} 个服务，其中 ${pending.length} 个尚未启用。确认配置后，继续完成接入。`
                  : '监听状态由 HomeWaf 实际返回。请通过新入口访问一次，确认请求确实经过这里。'
                : '选择 NAS、设备或网站，填写现有地址。HomeWaf 帮你准备连接和防护配置。'
            }}
          </p>
          <ElButton
            type="primary"
            size="large"
            @click="go(pending.length ? { enable: pending[0].config.port } : { action: 'create' })"
            >{{ pending.length ? '继续启用服务' : '接入服务' }}<ArrowRight class="button-icon after" aria-hidden="true"
          /></ElButton>
          <span class="welcome-note">{{
            pending.length ? '配置保存后，需要单独启用。' : '原有服务保持运行，准备好后再启用新入口。'
          }}</span>
        </div>
        <ol class="start-checklist" aria-label="服务接入流程">
          <li :class="{ done: ports.length }">
            <span class="checklist-number">{{ ports.length ? '✓' : '01' }}</span>
            <div>
              <h3>填写服务地址</h3>
              <p>选择场景，自动带入推荐设置。</p>
            </div>
            <span class="checklist-state">{{ ports.length ? '已配置' : '从这里开始' }}</span>
          </li>
          <li :class="{ done: running.length }">
            <span class="checklist-number">{{ running.length ? '✓' : '02' }}</span>
            <div>
              <h3>确认并启用</h3>
              <p>检查连接和访问策略，再启用监听。</p>
            </div>
            <span class="checklist-state">{{ running.length ? '已监听' : '待启用' }}</span>
          </li>
          <li>
            <span class="checklist-number">03</span>
            <div>
              <h3>验证实际访问</h3>
              <p>使用新入口访问，查看服务请求记录。</p>
            </div>
            <RouterLink v-if="ports.length" :to="{ name: 'WafPipeline', query: { inspect: ports[0].config.port } }"
              >查看记录<ArrowRight aria-hidden="true"
            /></RouterLink>
          </li>
        </ol>
      </section>
      <div class="workspace-metrics" aria-label="当前服务状态">
        <div>
          <span>已配置服务</span><strong>{{ ports.length }}<small>个</small></strong>
        </div>
        <div>
          <span>正在监听</span><strong>{{ running.length }}<small>个</small></strong>
        </div>
        <div>
          <span>等待启用</span><strong>{{ pending.length }}<small>个</small></strong>
        </div>
        <div>
          <span>今日拦截请求</span><strong>{{ denied.toLocaleString() }}<small>次</small></strong>
        </div>
      </div>
      <section class="workspace-services">
        <div class="workspace-section-heading">
          <div>
            <h2>我的服务</h2>
            <span>按服务管理连接、访问认证和防护</span>
          </div>
          <RouterLink to="/waf-pipeline/ports">管理服务<ArrowRight aria-hidden="true" /></RouterLink>
        </div>
        <div v-if="!ports.length" class="workspace-empty">
          <Connection aria-hidden="true" />
          <div>
            <h3>还没有接入服务</h3>
            <p>准备好现有服务地址，就可以开始。已有旧版站点可在服务接入页迁入。</p>
          </div>
          <ElButton @click="go({ action: 'create' })">添加第一个服务</ElButton>
        </div>
        <div v-else class="workspace-service-list">
          <RouterLink
            v-for="row in ports.slice(0, 4)"
            :key="row.config.port"
            :to="{ name: 'WafPipeline', query: { inspect: row.config.port } }"
            class="workspace-service"
            ><div class="service-symbol"><Connection aria-hidden="true" /></div>
            <div class="service-identity">
              <h3>{{ row.config.name || `服务 ${row.config.port}` }}</h3>
              <p>{{ row.config.upstream }}</p>
            </div>
            <span class="service-port">{{ row.config.protocol.toUpperCase() }} :{{ row.config.port }}</span
            ><ElTag :type="serviceStatus(row).tone as any" effect="light">{{ serviceStatus(row).label }}</ElTag
            ><ArrowRight class="service-arrow" aria-hidden="true"
          /></RouterLink>
        </div>
      </section>
    </template>
    <section class="workspace-resources">
      <div class="workspace-section-heading"><h2>常见任务</h2></div>
      <div class="resource-links">
        <RouterLink to="/waf/wafattacklog"
          ><Warning aria-hidden="true" />
          <div>
            <h3>查看拦截原因</h3>
            <p>从安全事件找到命中的规则。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
        <RouterLink to="/sys/RunDiagnostic"
          ><FirstAidKit aria-hidden="true" />
          <div>
            <h3>排查访问问题</h3>
            <p>检查实例运行状态与连接问题。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
        <RouterLink to="/help"
          ><Reading aria-hidden="true" />
          <div>
            <h3>阅读使用指南</h3>
            <p>了解接入方式和配置说明。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElAlert, ElButton, ElTag } from 'element-plus';
import { ArrowRight, Connection, FirstAidKit, Reading, Refresh, Warning } from '@element-plus/icons-vue';
import { pipelineGet } from '@/apis/pipeline';
import { serviceStatus } from '@/pages/waf/pipeline/onboarding';
const router = useRouter();
const ports = ref<any[]>([]),
  loading = ref(false),
  loaded = ref(false),
  error = ref('');
const pending = computed(() => ports.value.filter((row) => !row.config.listen_enabled));
const running = computed(() => ports.value.filter((row) => row.listener?.running));
const denied = computed(() => ports.value.reduce((sum, row) => sum + Number(row.stats?.today_denied || 0), 0));
function go(query: Record<string, string | number>) {
  router.push({ name: 'WafPipeline', query });
}
async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  try {
    const data = await pipelineGet('overview');
    ports.value = data.ports || [];
    loaded.value = true;
  } catch (cause: any) {
    error.value = `服务状态读取失败：${cause?.message || '请重试'}${loaded.value ? '。当前仍显示上次读取的结果。' : ''}`;
  } finally {
    loading.value = false;
  }
}
onMounted(load);
</script>

<style scoped>
.workbench {
  max-width: 1248px;
  margin: 0 auto;
  color: var(--el-text-color-primary);
  outline: none;
}
.workbench-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin: 8px 0 32px;
}
.workbench-eyebrow {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 12px;
}
.workbench h1 {
  font-size: 30px;
  letter-spacing: -0.8px;
  line-height: 1.4;
  font-weight: 500;
  margin: 0 0 8px;
}
.workbench-heading p {
  margin: 0;
  color: var(--el-text-color-regular);
  line-height: 1.7;
  font-size: 14px;
}
.button-icon {
  width: 16px;
  height: 16px;
  margin-right: 8px;
}
.button-icon.after {
  margin: 0 0 0 12px;
}
.workbench-alert {
  margin-bottom: 24px;
}
.workbench-loading {
  padding: 64px;
  text-align: center;
  color: var(--el-text-color-regular);
}
.welcome-panel {
  display: grid;
  grid-template-columns: 1fr 1fr;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  overflow: hidden;
}
.welcome-copy {
  padding: 36px;
}
.section-kicker {
  display: inline-flex;
  color: var(--el-color-primary);
  font-size: 13px;
  margin-bottom: 20px;
}
.welcome-copy h2 {
  font-size: 26px;
  font-weight: 500;
  letter-spacing: -0.6px;
  line-height: 1.5;
  margin: 0 0 12px;
}
.welcome-copy > p {
  font-size: 14px;
  line-height: 1.85;
  color: var(--el-text-color-regular);
  margin: 0 0 28px;
  max-width: 400px;
}
.welcome-note {
  display: block;
  margin-top: 12px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}
.start-checklist {
  margin: 0;
  list-style: none;
  padding: 24px 32px;
  background: var(--el-fill-color-extra-light);
  border-left: 1px solid var(--el-border-color-lighter);
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.start-checklist li {
  position: relative;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px 0;
}
.start-checklist li + li {
  border-top: 1px solid var(--el-border-color-lighter);
}
.checklist-number {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 1px solid var(--el-border-color);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
  color: var(--el-text-color-regular);
  background: var(--el-bg-color);
}
.done .checklist-number {
  border-color: var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}
.start-checklist h3 {
  font-size: 15px;
  font-weight: 500;
  margin: 0 0 6px;
}
.start-checklist p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-regular);
  line-height: 1.7;
}
.checklist-state,
.start-checklist a {
  margin-left: auto;
  font-size: 13px;
  white-space: nowrap;
  color: var(--el-text-color-regular);
}
.start-checklist a {
  color: var(--el-color-primary);
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 44px;
  text-decoration: none;
}
.start-checklist a svg {
  width: 14px;
}
.workspace-metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  padding: 28px 0;
  margin-bottom: 12px;
}
.workspace-metrics > div {
  padding: 0 28px;
  border-right: 1px solid var(--el-border-color-light);
}
.workspace-metrics > div:first-child {
  padding-left: 0;
}
.workspace-metrics > div:last-child {
  border: 0;
}
.workspace-metrics span {
  display: block;
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin-bottom: 12px;
}
.workspace-metrics strong {
  font-size: 28px;
  font-weight: 500;
  letter-spacing: -0.7px;
  font-variant-numeric: tabular-nums;
}
.workspace-metrics small {
  font-size: 13px;
  font-weight: 400;
  color: var(--el-text-color-regular);
  margin-left: 8px;
}
.workspace-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 20px 0 16px;
}
.workspace-section-heading h2 {
  font-size: 18px;
  font-weight: 500;
  margin: 0 0 6px;
}
.workspace-section-heading span {
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.workspace-section-heading > a {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--el-color-primary);
  text-decoration: none;
  font-size: 13px;
  min-height: 44px;
}
.workspace-section-heading > a svg {
  width: 15px;
}
.workspace-empty {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 32px;
  background: var(--el-bg-color);
  border: 1px dashed var(--el-border-color);
  border-radius: 12px;
}
.workspace-empty > svg {
  width: 36px;
  height: 36px;
  color: var(--el-text-color-regular);
  flex-shrink: 0;
}
.workspace-empty h3 {
  font-size: 16px;
  font-weight: 500;
  margin: 0 0 8px;
}
.workspace-empty p {
  font-size: 13px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
  margin: 0;
}
.workspace-empty .el-button {
  margin-left: auto;
  flex-shrink: 0;
}
.workspace-service-list {
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
  overflow: hidden;
}
.workspace-service {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 20px 24px;
  color: inherit;
  text-decoration: none;
  transition: background 150ms;
}
.workspace-service + .workspace-service {
  border-top: 1px solid var(--el-border-color-lighter);
}
.workspace-service:hover {
  background: var(--el-fill-color-extra-light);
}
.service-symbol {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  flex-shrink: 0;
}
.service-symbol svg {
  width: 22px;
  height: 22px;
}
.service-identity {
  min-width: 0;
  flex: 1;
}
.service-identity h3 {
  font-size: 14px;
  font-weight: 500;
  margin: 0 0 6px;
  overflow-wrap: anywhere;
}
.service-identity p {
  font-size: 13px;
  margin: 0;
  color: var(--el-text-color-regular);
  overflow-wrap: anywhere;
}
.service-port {
  font-size: 13px;
  color: var(--el-text-color-regular);
  font-family: monospace;
}
.service-arrow {
  width: 16px;
  flex-shrink: 0;
}
.workspace-resources {
  margin-top: 32px;
}
.resource-links {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}
.resource-links > a {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
  color: inherit;
  text-decoration: none;
  transition:
    transform 150ms,
    border-color 150ms;
}
.resource-links > a > svg {
  width: 22px;
  height: 22px;
  color: var(--el-text-color-regular);
  flex-shrink: 0;
}
.resource-links > a > svg:last-child {
  width: 15px;
  margin-left: auto;
}
.resource-links h3 {
  font-size: 14px;
  font-weight: 500;
  margin: 0 0 8px;
}
.resource-links p {
  font-size: 13px;
  color: var(--el-text-color-regular);
  margin: 0;
  line-height: 1.7;
}
.workbench :deep(.el-button) {
  min-height: 44px;
  border-radius: 6px;
}
.workbench a:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
@media (hover: hover) {
  .resource-links > a:hover {
    transform: translateY(-2px);
    border-color: var(--el-color-primary-light-5);
  }
}
@media (max-width: 1150px) {
  .welcome-panel {
    grid-template-columns: 1fr;
  }
  .start-checklist {
    border-left: 0;
    border-top: 1px solid var(--el-border-color-lighter);
  }
  .resource-links {
    grid-template-columns: 1fr;
  }
  .welcome-copy > p {
    max-width: none;
  }
  .welcome-copy {
    padding: 28px;
  }
}
@media (max-width: 600px) {
  .workbench-heading {
    position: relative;
    display: block;
    margin-top: 0;
    padding-top: 0;
  }
  .workbench-heading > div {
    width: 100%;
  }
  .workbench-eyebrow {
    min-height: 44px;
    display: flex;
    align-items: center;
  }
  .workbench-heading > .el-button {
    position: absolute;
    top: 0;
    right: 0;
  }
  .workbench h1 {
    font-size: 23px;
  }
  .workbench-eyebrow {
    font-size: 13px;
  }
  .workbench-heading > .el-button {
    padding: 10px;
  }
  .workbench-heading > .el-button .button-icon {
    display: none;
  }
  .workbench-heading p {
    font-size: 13px;
  }
  .welcome-copy {
    padding: 24px;
  }
  .welcome-copy h2 {
    font-size: 23px;
  }
  .welcome-copy > .el-button {
    width: 100%;
  }
  .start-checklist {
    padding: 12px 20px;
  }
  .start-checklist li {
    gap: 12px;
    flex-wrap: wrap;
  }
  .checklist-state {
    display: none;
  }
  .workspace-metrics {
    grid-template-columns: 1fr 1fr;
    gap: 24px;
    padding: 24px 0;
  }
  .workspace-metrics > div {
    padding: 0 16px;
  }
  .workspace-metrics > div:nth-child(odd) {
    padding-left: 0;
  }
  .workspace-metrics > div:nth-child(even) {
    border-right: 0;
  }
  .workspace-metrics strong {
    font-size: 26px;
  }
  .workspace-empty {
    padding: 24px;
    gap: 16px;
    flex-wrap: wrap;
  }
  .workspace-empty > div {
    flex: 1;
    min-width: 150px;
  }
  .workspace-empty .el-button {
    margin: 0;
    width: 100%;
  }
  .workspace-section-heading {
    align-items: flex-start;
  }
  .workspace-service {
    padding: 16px;
    gap: 12px;
    flex-wrap: wrap;
  }
  .service-port {
    display: none;
  }
  .service-arrow {
    display: none;
  }
  .service-symbol {
    width: 32px;
    height: 32px;
  }
  .workspace-section-heading span {
    display: block;
    line-height: 1.7;
  }
  .workspace-section-heading > a {
    white-space: nowrap;
  }
  .resource-links {
    gap: 12px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .workspace-service,
  .resource-links > a {
    transition: none;
  }
  .resource-links > a:hover {
    transform: none;
  }
}
</style>
