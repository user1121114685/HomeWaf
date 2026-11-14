<template>
  <div class="workbench" data-testid="workbench">
    <div class="workbench-toolbar">
      <span class="workbench-updated">{{ lastUpdated ? `状态更新于 ${lastUpdated}` : '当前实例' }}</span>
      <div>
        <ElButton :loading="loading" @click="load"><Refresh class="button-icon" aria-hidden="true" />刷新状态</ElButton
        ><ElButton type="primary" @click="go({ action: 'create' })"
          ><Connection class="button-icon" aria-hidden="true" />接入服务</ElButton
        >
      </div>
    </div>
    <ElAlert v-if="error" type="error" :title="error" :closable="false" show-icon class="workbench-alert" />
    <div v-if="!loaded && loading" class="workbench-loading" role="status">正在读取服务状态…</div>
    <template v-else-if="loaded">
      <section class="workspace-metrics" aria-label="当前服务状态">
        <div>
          <span>已配置服务</span><strong>{{ ports.length }}<small>个</small></strong>
          <p>保存在此实例中的服务</p>
        </div>
        <div>
          <span>正在监听</span><strong>{{ running.length }}<small>个</small></strong>
          <p>已启动入口，等待访问</p>
        </div>
        <div>
          <span>等待启用</span><strong>{{ pending.length }}<small>个</small></strong>
          <p>配置已保存，尚未监听</p>
        </div>
        <div>
          <span>今日拦截</span><strong>{{ denied.toLocaleString() }}<small>次</small></strong>
          <p>已接入服务的拦截总数</p>
        </div>
      </section>
      <div class="workbench-grid">
        <section class="workspace-services">
          <div class="workspace-section-heading">
            <h2>我的服务</h2>
            <RouterLink to="/waf-pipeline/ports">管理服务<ArrowRight aria-hidden="true" /></RouterLink>
          </div>
          <div v-if="!ports.length" class="workspace-empty">
            <Connection aria-hidden="true" />
            <h3>接入第一个服务</h3>
            <p>准备 NAS、设备或网站的现有地址，检查配置后启用新入口。</p>
            <ElButton @click="go({ action: 'create' })">填写服务地址</ElButton>
          </div>
          <div v-else class="workspace-service-list">
            <RouterLink
              v-for="row in ports.slice(0, 6)"
              :key="row.config.port"
              :to="{ name: 'WafPipeline', query: { inspect: row.config.port } }"
              class="workspace-service"
            >
              <span class="service-symbol"><Connection aria-hidden="true" /></span>
              <div class="service-identity">
                <h3>{{ row.config.name || `服务 ${row.config.port}` }}</h3>
                <p>{{ row.config.upstream }}</p>
              </div>
              <span class="service-port">:{{ row.config.port }}</span
              ><ElTag :type="serviceStatus(row).tone as any" effect="light">{{ serviceStatus(row).label }}</ElTag
              ><ArrowRight class="service-arrow" aria-hidden="true" />
            </RouterLink>
          </div>
          <p class="workspace-panel-note">监听启动后，通过新入口访问一次，并在记录中确认请求。</p>
        </section>
        <section class="workspace-next">
          <div class="workspace-section-heading">
            <h2>{{ pending.length ? '继续完成接入' : '服务接入流程' }}</h2>
          </div>
          <ol class="start-checklist" aria-label="服务接入流程">
            <li :class="{ done: ports.length }">
              <span class="checklist-number">01</span>
              <div>
                <h3>填写服务地址</h3>
                <p>选择场景，设置新入口与现有地址。</p>
              </div>
              <span v-if="ports.length" class="checklist-state">已配置</span>
            </li>
            <li :class="{ done: running.length }">
              <span class="checklist-number">02</span>
              <div>
                <h3>确认并启用</h3>
                <p>核对访问策略，再启动监听。</p>
              </div>
              <span v-if="running.length" class="checklist-state">已监听</span>
            </li>
            <li>
              <span class="checklist-number">03</span>
              <div>
                <h3>验证实际访问</h3>
                <p>访问新入口，检查请求与防护记录。</p>
              </div>
            </li>
          </ol>
          <ElButton v-if="pending.length" @click="go({ enable: pending[0].config.port })"
            >继续启用服务<ArrowRight class="button-icon after"
          /></ElButton>
          <RouterLink
            v-else-if="ports.length"
            :to="{ name: 'WafPipeline', query: { inspect: ports[0].config.port } }"
            class="workspace-next-link"
            >查看服务记录<ArrowRight
          /></RouterLink>
          <RouterLink v-else to="/help" class="workspace-next-link">阅读接入说明<ArrowRight /></RouterLink>
        </section>
      </div>
    </template>
    <section class="workspace-resources">
      <div class="workspace-section-heading"><h2>常用操作</h2></div>
      <div class="resource-links">
        <RouterLink to="/waf/wafattacklog"
          ><Warning aria-hidden="true" />
          <div>
            <h3>查看拦截原因</h3>
            <p>定位事件，核对命中的防护规则。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
        <RouterLink to="/sys/RunDiagnostic"
          ><FirstAidKit aria-hidden="true" />
          <div>
            <h3>排查访问问题</h3>
            <p>检查运行状态与连接问题。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
        <RouterLink to="/dashboard/base"
          ><Reading aria-hidden="true" />
          <div>
            <h3>查看流量趋势</h3>
            <p>了解请求分布和访问来源。</p>
          </div>
          <ArrowRight aria-hidden="true"
        /></RouterLink>
      </div>
    </section>
  </div>
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
  error = ref(''),
  lastUpdated = ref('');
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
    lastUpdated.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
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
  color: var(--hw-ink);
}
.workbench-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}
.workbench-toolbar > div {
  display: flex;
  gap: 8px;
}
.workbench-updated {
  color: var(--hw-muted);
  font-size: 13px;
}
.button-icon {
  width: 16px;
  height: 16px;
  margin-right: 8px;
}
.button-icon.after {
  margin: 0 0 0 8px;
}
.workbench-alert {
  margin-bottom: 24px;
}
.workbench-loading {
  padding: 64px 24px;
  text-align: center;
  color: var(--hw-muted);
}
.workspace-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  margin-bottom: 24px;
}
.workspace-metrics > div {
  padding: 24px;
  min-width: 0;
}
.workspace-metrics > div + div {
  border-left: 1px solid var(--hw-line);
}
.workspace-metrics span {
  font-size: 13px;
  color: var(--hw-muted);
}
.workspace-metrics strong {
  display: block;
  margin: 12px 0 8px;
  font: 500 36px/1.1 var(--font-display);
  letter-spacing: -0.04em;
  font-variant-numeric: tabular-nums;
}
.workspace-metrics small {
  font: 13px var(--font-text);
  color: var(--hw-muted);
  margin-left: 8px;
}
.workspace-metrics p {
  color: var(--hw-muted);
  font-size: 13px;
}
.workbench-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(300px, 1fr);
  gap: 24px;
}
.workspace-services,
.workspace-next {
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  background: var(--hw-panel);
  min-width: 0;
}
.workspace-section-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
}
.workspace-section-heading h2 {
  font-size: 16px;
  font-weight: 500;
}
.workspace-section-heading a,
.workspace-next-link {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  color: var(--hw-muted);
  font-size: 13px;
  min-height: 32px;
}
.workspace-section-heading svg,
.workspace-next-link svg {
  width: 14px;
  height: 14px;
}
.workspace-service {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px;
  border-top: 1px solid var(--hw-line);
  color: var(--hw-ink);
  transition: background 150ms;
}
.workspace-service:hover {
  background: var(--hw-subtle);
}
.service-symbol {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  background: var(--hw-subtle);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  flex-shrink: 0;
}
.service-symbol svg {
  width: 20px;
  height: 20px;
  color: var(--hw-muted);
}
.service-identity {
  flex: 1;
  min-width: 0;
}
.service-identity h3 {
  font-size: 14px;
  font-weight: 500;
}
.service-identity p {
  font: 13px/1.6 var(--font-mono);
  color: var(--hw-muted);
  overflow-wrap: anywhere;
  margin-top: 4px;
}
.service-port {
  font: 13px var(--font-mono);
  color: var(--hw-muted);
}
.service-arrow {
  width: 16px;
  height: 16px;
  color: var(--hw-muted);
  flex-shrink: 0;
}
.workspace-panel-note {
  padding: 16px 24px;
  border-top: 1px solid var(--hw-line);
  font-size: 13px;
  color: var(--hw-muted);
}
.start-checklist {
  padding: 0 24px;
}
.start-checklist li {
  display: flex;
  gap: 12px;
  padding: 16px 0;
  align-items: flex-start;
}
.checklist-number {
  font: 13px var(--font-mono);
  border: 1px solid var(--hw-line);
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: var(--hw-muted);
  flex-shrink: 0;
}
.done .checklist-number {
  border-color: transparent;
  background: var(--el-color-success-light-9);
  color: var(--hw-success);
}
.start-checklist h3 {
  font-size: 14px;
  font-weight: 500;
}
.start-checklist p {
  font-size: 13px;
  color: var(--hw-muted);
  margin-top: 4px;
}
.checklist-state {
  font-size: 13px;
  color: var(--hw-success);
  margin-left: auto;
  white-space: nowrap;
}
.workspace-next > .el-button,
.workspace-next-link {
  margin: 16px 24px 24px;
}
.workspace-empty {
  padding: 40px 24px;
  text-align: center;
  border-top: 1px solid var(--hw-line);
}
.workspace-empty > svg {
  width: 32px;
  color: var(--hw-muted);
}
.workspace-empty h3 {
  margin: 16px 0 8px;
  font-weight: 500;
  font-size: 16px;
}
.workspace-empty p {
  color: var(--hw-muted);
  margin: 0 auto 20px;
  max-width: 35ch;
}
.workspace-resources {
  margin-top: 32px;
}
.workspace-resources > .workspace-section-heading {
  padding: 0 0 16px;
}
.resource-links {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.resource-links > a {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  color: var(--hw-ink);
  transition:
    transform 180ms var(--hw-ease),
    border-color 150ms;
}
.resource-links > a > svg {
  width: 20px;
  height: 20px;
  color: var(--hw-muted);
  flex-shrink: 0;
}
.resource-links > a > svg:last-child {
  margin-left: auto;
  width: 16px;
}
.resource-links h3 {
  font-size: 14px;
  font-weight: 500;
}
.resource-links p {
  color: var(--hw-muted);
  font-size: 13px;
  margin-top: 6px;
}
@media (hover: hover) and (pointer: fine) {
  .resource-links > a:hover {
    transform: translateY(-2px);
    border-color: var(--hw-accent);
  }
}
@media (max-width: 1200px) {
  .workbench-grid {
    grid-template-columns: 1fr;
  }
  .resource-links {
    grid-template-columns: 1fr;
  }
  .resource-links p {
    margin-top: 4px;
  }
}
@media (max-width: 760px) {
  .workspace-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .workspace-metrics > div {
    padding: 20px 16px;
  }
  .workspace-metrics > div:nth-child(3) {
    border-left: 0;
  }
  .workspace-metrics > div:nth-child(n + 3) {
    border-top: 1px solid var(--hw-line);
  }
  .workspace-metrics strong {
    font-size: 30px;
  }
  .workspace-metrics p {
    font-size: 13px;
  }
  .workspace-section-heading,
  .workspace-service {
    padding: 16px;
  }
  .service-port,
  .service-arrow {
    display: none;
  }
  .service-symbol {
    width: 32px;
    height: 32px;
  }
  .workspace-service {
    flex-wrap: wrap;
  }
  .workspace-service .el-tag {
    margin-left: 44px;
  }
  .workspace-service .service-identity {
    flex-basis: calc(100% - 48px);
  }
  .workbench-toolbar {
    flex-wrap: wrap;
  }
  .workbench-toolbar > div {
    width: 100%;
  }
  .workbench-toolbar .el-button {
    flex: 1;
  }
  .start-checklist {
    padding: 0 16px;
  }
  .workspace-panel-note {
    padding: 16px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .resource-links > a {
    transition: none;
  }
  .resource-links > a:hover {
    transform: none;
  }
}
</style>
