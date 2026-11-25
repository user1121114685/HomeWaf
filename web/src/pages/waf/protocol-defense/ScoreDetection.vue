<template>
  <section class="automatic-assessment" data-testid="score-detection" :aria-busy="busy || loading">
    <header class="assessment-heading">
      <div>
        <h2>检查服务防御能力</h2>
        <p>自动模拟不同来源和常见攻击，找出能挡住的请求与仍需处理的风险。</p>
      </div>
      <a v-if="config" :href="advancedHref" target="_blank" rel="noopener" @click="prepareAdvanced">自定义测试 ↗</a>
    </header>
    <ElAlert v-if="loadError" :title="loadError" type="error" :closable="false"
      ><ElButton text @click="load">重新读取服务</ElButton></ElAlert
    >
    <ElSkeleton v-if="loading" :rows="3" animated />
    <template v-else-if="config">
      <div class="service-selection">
        <label v-if="!draftConfig"
          >选择服务<ElSelect v-model="port" aria-label="检测服务" :disabled="busy"
            ><ElOption
              v-for="service in services"
              :key="service.port"
              :label="(service.name || '服务') + ' · ' + service.port"
              :value="service.port" /></ElSelect
        ></label>
        <div class="service-identity">
          <strong>{{ draftConfig ? '检测当前草稿' : '检测已保存配置' }} · {{ config.name || '服务' }}</strong
          ><code>{{ serviceTarget(config) }}</code
          ><span>配置版本 {{ config.revision }}{{ draftConfig ? ' · 无需先保存' : '' }}</span>
        </div>
        <ElButton v-if="!draftConfig" text :disabled="busy" @click="load">刷新配置</ElButton>
      </div>
      <div v-if="!result" class="coverage-overview">
        <div>
          <strong>逐城检查来源</strong
          ><span
            >中国每城、每种已启用协议各 5 个 IP，优先覆盖移动、联通、电信、广电；未启用协议保留每城 1
            个探测样本。数据库缺口会明确列出。</span
          >
        </div>
        <div><strong>攻击与正常访问</strong><span>自动生成多种请求形式，并检查正常使用是否受影响</span></div>
        <div><strong>突发访问与诱捕</strong><span>自动设置压力方案，并检查已启用的伪装画像</span></div>
      </div>
      <div class="run-controls">
        <ElButton type="primary" size="large" :loading="busy" :disabled="!!loadError" @click="run">{{
          busy ? '正在检测' : result ? '重新检测' : '开始检测'
        }}</ElButton>
        <ElButton v-if="busy" @click="cancel">取消检测</ElButton>
        <span v-if="busy" role="status">{{
          progress.total
            ? `已检查 ${progress.done} / ${progress.total} 个来源${progress.done === progress.total ? '，正在检查压力与画像状态' : ''}`
            : '正在从 IP 数据库生成各城市的检测方案…'
        }}</span>
        <span v-else class="quiet">自动完成全部测试准备；不会修改访问策略或向服务发送真实攻击。</span>
      </div>
      <ElAlert v-if="error" :title="error" type="error" :closable="false"
        ><ElButton text :disabled="busy" @click="run">重试检测</ElButton
        ><RouterLink to="/sys/IPLocation" target="_blank" rel="noopener">查看 IP 数据库</RouterLink></ElAlert
      >
      <template v-if="result">
        <ElAlert v-if="incompleteCities" :title="`${incompleteCities} 个库内城市存在样本缺口；当前分数仅反映已完成的检测。`" type="warning" :closable="false" />
        <ElAlert v-if="stale" title="服务配置已变化，下方为上次结果。请重新检测。" type="warning" :closable="false" />
        <div class="score-result" role="status">
          <div class="score-value">
            <strong>{{ report.score ?? '—' }}</strong
            ><span>/ 100</span>
          </div>
          <div>
            <h3>{{ verdictTitle }}</h3>
            <p>
              {{
                report.score == null
                  ? '查看下方原因与建议，补齐覆盖后再检测。'
                  : `业务直连防御 ${report.direct_score} × 90% + 画像就绪 ${report.camouflage_score} 分。仅观察不会算作拦截。`
              }}
            </p>
            <span class="quiet"
              >{{ result.plan?.cities?.length || 0 }} 个库内城市 · {{ result.plan?.ipv4 || 0 }} 个 IPv4 ·
              {{ result.plan?.ipv6 || 0 }} 个 IPv6 · {{ report.evaluations }} 次判定</span
            >
          </div>
        </div>
        <div class="finding-grid">
          <section>
            <span>来源访问限制</span
            ><strong>{{ boundary ? boundary.blocked + ' / ' + boundary.total : '本轮无拒绝来源' }}</strong>
            <p>{{ boundary ? '不允许的来源中，被直接拒绝的样本' : '尚未验证非允许来源的拒绝效果' }}</p>
          </section>
          <section>
            <span>正常访问</span><strong>{{ report.normal_passed }} / {{ report.normal_total }}</strong>
            <p>正常请求能继续访问或到达服务认证入口</p>
          </section>
          <section>
            <span>突发访问保护</span><strong>{{ report.pressure.limited }} / {{ report.pressure.evaluations }}</strong>
            <p>重复请求中触发限流或验证的次数</p>
          </section>
        </div>
        <section class="result-section">
          <div class="result-heading">
            <h3>建议优先处理</h3>
            <RouterLink
              v-if="!draftConfig"
              :to="{ name: 'WafPipeline', query: { edit: config.port, tab: 'ruleset' } }"
              target="_blank"
              rel="noopener"
              >查看服务规则 ↗</RouterLink
            >
          </div>
          <ol v-if="report.advice.length" class="recommendations">
            <li v-for="advice in report.advice" :key="advice">{{ advice }}</li>
          </ol>
          <p v-else>本轮已覆盖的样本没有发现明显缺口。分数反映本次测试范围，不能保证所有攻击都被阻止。</p>
        </section>
        <section class="result-section">
          <h3>各类攻击的防御结果</h3>
          <div v-for="category in report.categories" :key="category.category" class="category-result">
            <div>
              <strong>{{ categoryLabels[category.category] || category.category }}</strong
              ><span
                >{{ category.blocked }} 次已防御 · {{ category.challenged }} 次需验证 ·
                {{ category.observed + category.missed }} 次未防御</span
              >
            </div>
            <ElTag :type="category.observed + category.missed ? 'warning' : 'success'">{{
              category.observed + category.missed ? '需要关注' : '本轮通过'
            }}</ElTag>
          </div>
        </section>
        <section v-if="report.pressure.lanes?.length" class="result-section">
          <h3>压力检查</h3>
          <div v-for="lane in report.pressure.lanes" :key="lane.name" class="category-result">
            <div>
              <strong>{{ lane.name }}</strong
              ><span
                >{{ lane.evaluations }} 次连续请求；{{
                  lane.first_limited ? '第 ' + lane.first_limited + ' 次开始限制' : '本轮未触发限制'
                }}</span
              >
            </div>
            <ElTag :type="lane.limited ? 'success' : 'info'">限制 {{ lane.limited }} 次</ElTag>
          </div>
          <p class="quiet">在独立计数环境中检查限流规则，结果不代表公网带宽或源站承载上限。</p>
        </section>
        <section class="result-section">
          <div class="result-heading">
            <h3>伪装画像 · {{ report.camouflage_score }} / 10 分</h3>
            <RouterLink
              :to="{ name: 'WafProtocolDefense', query: { tab: 'profiles', create: '1' } }"
              target="_blank"
              rel="noopener"
              >添加伪装画像 ↗</RouterLink
            >
          </div>
          <p>每条画像单独选择协议、端口和版本。至少一个已保存、启用且监听运行的画像可获得 10 分。</p>
          <p v-if="!result.camouflage.length">建议至少配置一个画像，用来诱捕并封禁尝试探测的 IP。</p>
          <div v-for="profile in result.camouflage" :key="profile.id" class="category-result">
            <div>
              <strong>{{ profile.name }} · {{ profile.protocol_profile }} :{{ profile.port }}</strong
              ><span
                >版本 {{ profile.version || '默认' }} · {{ profile.predicted }} 个来源命中后预计封禁 ·
                {{ profile.local }} 个局域网豁免</span
              ><span v-if="profile.last_error">{{ profile.last_error }}</span>
            </div>
            <ElTag :type="profile.running ? 'success' : 'warning'">{{
              profile.running ? '监听运行中' : profile.enabled ? '监听未运行' : '未启用'
            }}</ElTag>
          </div>
          <p class="quiet">
            此项检查画像就绪状态；实际封禁需来源先访问画像端口，且系统防火墙执行成功。本轮不执行封禁。
          </p>
        </section>
        <section v-if="result.listeners?.length" class="result-section">
          <h3>实际协议监听</h3>
          <div v-for="listener in result.listeners" :key="listener.family" class="category-result">
            <div>
              <strong>{{ listener.family }} · 已保存配置{{ listener.expected ? '启用' : '关闭' }}</strong
              ><span>{{ listener.detail }}</span>
            </div>
            <ElTag :type="listener.connected && !listener.expected ? 'danger' : 'info'">{{
              listener.connected && !listener.expected
                ? '发现意外监听'
                : listener.connected
                  ? '本机可连接'
                  : '本机未连通'
            }}</ElTag>
          </div>
          <p class="quiet">只检查所选服务本机 TCP 端口；不能代替公网连通性检查。草稿的监听变更需保存后复测。</p>
        </section>
        <details v-if="result.plan?.cities?.length" class="result-details">
          <summary>城市覆盖 · {{ result.plan.cities.length }} 个城市 · {{ incompleteCities }} 个存在缺口</summary>
          <p class="quiet">
            范围为当前 IP 数据库收录的中国城市；未收录的城市尚未验证。每种启用协议每城 5 个不同
            IP，运营商尽量多样；未启用协议减至 1 个。不会用重复或虚构地址填补缺口。
          </p>
          <ElTable :data="result.plan.cities.slice((cityPage - 1) * 20, cityPage * 20)" aria-label="城市覆盖"
            ><ElTableColumn label="城市" min-width="160"
              ><template #default="{ row }">{{ row.province }} · {{ row.city }}</template></ElTableColumn
            ><ElTableColumn label="IPv4" min-width="200"
              ><template #default="{ row }"
                >{{ row.ipv4 }} / {{ row.ipv4_target }} · {{ row.ipv4_carriers?.join('、') || '无记录' }}</template
              ></ElTableColumn
            ><ElTableColumn label="IPv6" min-width="200"
              ><template #default="{ row }"
                >{{ row.ipv6 }} / {{ row.ipv6_target }} · {{ row.ipv6_carriers?.join('、') || '无记录' }}</template
              ></ElTableColumn
            ></ElTable
          ><ElPagination
            v-model:current-page="cityPage"
            :page-size="20"
            :total="result.plan.cities.length"
            layout="prev, pager, next, total"
          />
        </details>
        <details class="result-details">
          <summary>查看本次来源与测试范围</summary>
          <p class="quiet">地址从当前 IP 数据库的不同网段中生成，归属地经过查询确认。允许名单中的来源也会纳入检查。</p>
          <ElTable :data="sourceRows.slice((sourcePage - 1) * 20, sourcePage * 20)" aria-label="自动生成的测试来源"
            ><ElTableColumn prop="ip" label="模拟来源" min-width="230" /><ElTableColumn
              prop="family"
              label="协议"
              width="80"
            /><ElTableColumn label="地区 / 运营商" min-width="200"
              ><template #default="{ row }">{{
                [row.country, row.province, row.city, row.isp].filter(Boolean).join(' · ') || '本地或未知'
              }}</template></ElTableColumn
            ><ElTableColumn label="预期" min-width="110"
              ><template #default="{ row }">{{
                row.expect_access ? '正常可访问' : '应拒绝访问'
              }}</template></ElTableColumn
            ><ElTableColumn label="实际判定" min-width="210"
              ><template #default="{ row }"
                >{{ row.blocked || 0 }} 次防御 · {{ row.passed || 0 }} 次放行 · {{ row.observed || 0 }} 次观察 ·
                {{ row.challenged || 0 }} 次挑战 · {{ row.unavailable || 0 }} 次不完整</template
              ></ElTableColumn
            ></ElTable
          ><ElPagination
            v-model:current-page="sourcePage"
            :page-size="20"
            :total="sourceRows.length"
            layout="prev, pager, next, total"
          />
          <p v-for="limit in report.limits" :key="limit" class="quiet">{{ limit }}</p>
        </details>
      </template>
    </template>
    <div v-else-if="!loadError" class="first-use">
      <h3>先添加要保护的服务</h3>
      <p>添加后，这里会自动生成适合该服务的检测方案。</p>
      <RouterLink :to="{ name: 'WafPipeline' }">前往我的服务</RouterLink>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue';
import { useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElOption,
  ElPagination,
  ElSelect,
  ElSkeleton,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';
import { runAssessment, type AssessmentProgress } from './assessmentJob';
import { pipelineGet } from '@/apis/pipeline';
import type { PortConfig } from '../pipeline/model';
import { categoryLabels, serviceTarget } from './assessment';
import { provideAssessmentHandoff } from './assessmentWindow';
const props = defineProps({ draftConfig: Object as PropType<PortConfig> });
const router = useRouter(),
  services = ref<PortConfig[]>([]),
  port = ref<number>(),
  loading = ref(false),
  busy = ref(false),
  loadError = ref(''),
  error = ref(''),
  result = ref<any>(null),
  resultKey = ref('');
const config = computed(() => props.draftConfig || services.value.find((service) => service.port === port.value));
const key = computed(() => JSON.stringify(config.value)),
  stale = computed(() => resultKey.value !== key.value);
const report = computed(() => result.value?.assessment);
const boundary = computed(() => report.value?.categories.find((row: any) => row.category === 'source_boundary'));
const verdictTitle = computed(() =>
  report.value?.score == null
    ? '本轮覆盖还不完整'
    : report.value.score >= 90
      ? '本轮防御表现良好'
      : report.value.score >= 60
        ? '部分防御仍需加强'
        : '发现需要优先处理的缺口',
);
const progress = ref<AssessmentProgress>({ state: 'preparing', done: 0, total: 0 }),
  cityPage = ref(1),
  sourcePage = ref(1);
const incompleteCities = computed(
  () =>
    result.value?.plan?.cities?.filter((city: any) => city.ipv4 < city.ipv4_target || city.ipv6 < city.ipv6_target)
      .length || 0,
);
const sourceRows = computed(() => {
  const results = new Map((report.value?.source_results || []).map((row: any) => [row.ip, row]));
  return (result.value?.plan?.sources || []).map((row: any) => ({
    ...row,
    ...((results.get(row.ip) as object) || {}),
  }));
});
const handoffId = crypto.randomUUID(),
  advancedHref = computed(
    () =>
      router.resolve({
        name: 'WafAssessmentAdvanced',
        query: { port: config.value?.port, ...(props.draftConfig || result.value?.plan ? { handoff: handoffId } : {}) },
      }).href,
  );
let live = true,
  controller: AbortController | undefined,
  releaseHandoff = () => {};
function prepareAdvanced() {
  releaseHandoff();
  if (props.draftConfig || result.value?.plan)
    releaseHandoff = provideAssessmentHandoff(handoffId, { config: props.draftConfig, plan: result.value?.plan });
}
async function load() {
  loading.value = true;
  loadError.value = '';
  try {
    if (!props.draftConfig) {
      const data = await pipelineGet<PortConfig[]>('ports');
      if (live) {
        services.value = data;
        if (!data.some((service) => service.port === port.value)) port.value = data[0]?.port;
      }
    }
  } catch (cause: any) {
    if (live) loadError.value = cause.message || '服务配置暂时无法读取，请重试';
  } finally {
    if (live) loading.value = false;
  }
}
function cancel() {
  controller?.abort();
}
async function run() {
  if (busy.value || !config.value) return;
  busy.value = true;
  error.value = '';
  progress.value = { state: 'preparing', done: 0, total: 0 };
  const startedKey = key.value;
  controller = new AbortController();
  try {
    const data = await runAssessment(
      {
        port: config.value.port,
        expected_revision: config.value.revision,
        automatic: true,
        ...(props.draftConfig ? { config: props.draftConfig } : {}),
      },
      controller.signal,
      (value) => {
        if (live) progress.value = value;
      },
    );
    if (live) {
      result.value = data;
      resultKey.value = startedKey;
      cityPage.value = 1;
      sourcePage.value = 1;
    }
  } catch (cause: any) {
    if (live)
      error.value = controller.signal.aborted
        ? '检测已取消，可以随时重新开始。'
        : cause.message || '检测暂时未完成，请重试。';
  } finally {
    if (live) busy.value = false;
  }
}
watch(
  () => config.value?.port,
  () => {
    result.value = null;
    error.value = '';
  },
);
onMounted(load);
onBeforeUnmount(() => {
  live = false;
  cancel();
  releaseHandoff();
});
</script>
<style scoped>
.automatic-assessment {
  display: grid;
  gap: 24px;
  min-width: 0;
  color: var(--hw-ink);
}
h2,
h3,
p {
  margin: 0;
}
h2 {
  font-size: 20px;
}
h3 {
  font-size: 16px;
}
.assessment-heading,
.result-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}
.assessment-heading p {
  color: var(--hw-muted);
  margin-top: 8px;
  line-height: 1.7;
}
.assessment-heading > a {
  white-space: nowrap;
  padding: 6px 0;
}
.service-selection {
  display: flex;
  align-items: center;
  gap: 24px;
  border-bottom: 1px solid var(--hw-line);
  padding-bottom: 22px;
}
.service-selection label {
  display: grid;
  gap: 8px;
  width: 260px;
  max-width: 100%;
  font-size: 14px;
}
.service-identity {
  display: grid;
  gap: 5px;
  min-width: 0;
}
.service-identity span,
.quiet {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
}
code {
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
  font-size: 13px;
}
.coverage-overview,
.finding-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
}
.coverage-overview > div {
  display: grid;
  gap: 8px;
}
.coverage-overview span,
.finding-grid p {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
}
.run-controls {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.score-result {
  display: flex;
  align-items: center;
  gap: 28px;
  padding: 24px 0;
  border-top: 1px solid var(--hw-line);
  border-bottom: 1px solid var(--hw-line);
}
.score-value {
  display: flex;
  align-items: baseline;
  gap: 8px;
  white-space: nowrap;
}
.score-value strong {
  font: 700 52px var(--font-mono);
}
.score-value span {
  color: var(--hw-muted);
}
.score-result p {
  margin: 8px 0;
  color: var(--hw-muted);
  line-height: 1.7;
}
.finding-grid section {
  display: grid;
  gap: 8px;
}
.finding-grid section > span {
  font-size: 13px;
  color: var(--hw-muted);
}
.finding-grid strong {
  font-size: 23px;
  font-variant-numeric: tabular-nums;
}
.result-section {
  display: grid;
  gap: 14px;
  border-top: 1px solid var(--hw-line);
  padding-top: 22px;
}
.recommendations {
  margin: 0;
  padding-left: 23px;
  line-height: 1.8;
}
.recommendations li + li {
  margin-top: 8px;
}
.category-result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--hw-line);
}
.category-result > div {
  display: grid;
  gap: 5px;
  min-width: 0;
}
.category-result span {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.6;
}
.result-details summary {
  cursor: pointer;
  padding: 8px 0;
}
.result-details[open] {
  display: grid;
  gap: 14px;
}
.first-use {
  display: grid;
  gap: 14px;
  padding: 24px 0;
}
.first-use p {
  color: var(--hw-muted);
}
@media (max-width: 760px) {
  .assessment-heading,
  .service-selection {
    flex-direction: column;
    align-items: stretch;
  }
  .service-selection label {
    width: 100%;
  }
  .coverage-overview,
  .finding-grid {
    grid-template-columns: 1fr;
    gap: 18px;
  }
  .score-result {
    align-items: flex-start;
    gap: 18px;
  }
  .score-value strong {
    font-size: 40px;
  }
  .category-result {
    align-items: flex-start;
  }
  .run-controls > .el-button {
    min-height: 44px;
  }
}
</style>
