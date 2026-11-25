<template>
  <section class="score-detection" data-testid="score-detection" :aria-busy="busy">
    <header>
      <h2>服务防御评分</h2>
      <p>检查允许地区内的攻击、非允许来源的探测和正常访问，分数越高代表本轮样本下的防御越完整。</p>
    </header>
    <ElButton v-if="!draftConfig" class="refresh-config" :disabled="busy || loading" @click="load"
      >刷新服务配置</ElButton
    >
    <ElAlert v-if="loadError" :title="loadError" type="error" :closable="false"
      ><ElButton text @click="load">重新读取服务</ElButton></ElAlert
    >
    <ElSkeleton v-if="loading" :rows="3" animated />
    <template v-else>
      <div class="score-fields">
        <label v-if="!draftConfig"
          >我的服务
          <ElSelect v-model="port" aria-label="检测服务" :disabled="busy" placeholder="选择已添加的服务">
            <ElOption
              v-for="service in services"
              :key="service.port"
              :value="service.port"
              :label="`${service.name || '服务'} · ${service.port}`"
            />
          </ElSelect>
        </label>
        <label
          >检测方式<ElRadioGroup v-model="mode" :disabled="busy" aria-label="检测方式"
            ><ElRadioButton value="batch">批量评分</ElRadioButton
            ><ElRadioButton value="single">单条判定</ElRadioButton></ElRadioGroup
          ></label
        >
        <label
          >联动伪装画像<ElSelect
            v-model="profileId"
            :disabled="busy"
            clearable
            aria-label="联动伪装画像"
            placeholder="不模拟诱捕联动"
          >
            <ElOption
              v-for="profile in profiles"
              :key="profile.id"
              :value="profile.id"
              :label="`${profile.name} · ${profile.port}${profile.enabled ? '' : '（未启用）'}`"
            /> </ElSelect
        ></label>
      </div>
      <ElAlert v-if="profilesError" :title="profilesError" type="warning" :closable="false"
        ><ElButton text @click="loadProfiles">重读画像</ElButton></ElAlert
      >
      <p v-if="config" class="target-line">
        <strong
          >{{ draftConfig ? '当前未保存草稿' : '当前已保存配置' }} · {{ config.name }} · v{{ config.revision }}</strong
        ><code>{{ serviceTarget(config) }}</code>
      </p>
      <p v-else-if="!loadError" class="muted">尚未添加服务。请先在“我的服务”接入服务，再进行检测。</p>
      <p class="muted">
        域名来自所选服务；未填域名时使用本机监听 IP 与端口。此处在隔离环境计算判定，不发送网络攻击或修改线上封禁。
      </p>
      <fieldset :disabled="busy || !config">
        <legend>模拟来源与预期访问</legend>
        <div class="score-actions">
          <ElButton :loading="sourcesLoading" @click="sampleSources">从当前 IP 数据库取样</ElButton
          ><ElButton :disabled="sources.length >= 10000" @click="addSource">添加来源 IP</ElButton
          ><span class="muted">“允许”表示这个来源应能正常使用服务，请按你的实际需求确认。</span>
        </div>
        <ElAlert v-if="sourcesError" :title="sourcesError" type="warning" :closable="false" />
        <div
          v-for="(source, rowIndex) in sources.slice((sourcePage - 1) * 20, sourcePage * 20)"
          :key="(sourcePage - 1) * 20 + rowIndex"
          class="source-row"
        >
          <label
            >来源 IP {{ (sourcePage - 1) * 20 + rowIndex + 1
            }}<ElInput
              v-model="source.ip"
              :aria-label="`来源 IP ${(sourcePage - 1) * 20 + rowIndex + 1}`"
              @input="
                source.country = '';
                source.province = '';
              "
          /></label>
          <span class="muted">{{
            [source.country, source.province].filter(Boolean).join(' / ') || '检测时查询当前数据库'
          }}</span>
          <label
            >预期访问<ElSelect
              v-model="source.expect_access"
              :aria-label="`来源 ${(sourcePage - 1) * 20 + rowIndex + 1} 预期访问`"
              ><ElOption :value="true" label="允许正常访问" /><ElOption :value="false" label="拒绝访问" /></ElSelect
          ></label>
          <ElButton
            text
            type="danger"
            :aria-label="`移除来源 ${(sourcePage - 1) * 20 + rowIndex + 1}`"
            @click="sources.splice((sourcePage - 1) * 20 + rowIndex, 1)"
            >移除</ElButton
          >
        </div>
        <ElPagination
          v-model:current-page="sourcePage"
          :page-size="20"
          :total="sources.length"
          layout="prev, pager, next, total"
        />
        <p v-if="!sources.length" class="muted">添加至少一个预期允许的来源，以及一个预期拒绝的来源。</p>
      </fieldset>
      <fieldset v-if="mode === 'batch'" :disabled="busy || !config">
        <legend>测试样本与压力</legend>
        <ElCheckbox v-model="builtins">包含内置攻击、来源伪造和正常请求</ElCheckbox>
        <div class="score-fields">
          <label
            >导入请求日志 JSON（最多 4 MiB / 256 条）<input
              type="file"
              accept=".json,application/json"
              :disabled="busy"
              @change="importLog"
          /></label>
          <label
            >隔离 QPS / CC 重复请求<ElInputNumber
              v-model="pressure"
              :min="0"
              :max="2000"
              :step="100"
              aria-label="重复请求次数"
          /></label>
        </div>
        <div class="score-fields">
          <label
            >压力方式<ElSelect v-model="pressureMode" aria-label="压力方式"
              ><ElOption value="mixed" label="单来源与多来源" /><ElOption value="single" label="单来源" /><ElOption
                value="distributed"
                label="多来源" /></ElSelect></label
          ><label>压力路径<ElInput v-model="pressurePath" aria-label="压力路径" /></label>
        </div>
        <p class="muted">重复请求轮流使用上述来源，只测限流计数与规则计算耗时，不计入总分，也不代表公网或源站容量。</p>
        <template v-if="samples.length">
          <div class="score-actions">
            <strong>已导入 {{ samples.length }} 条</strong><ElButton text @click="samples = []">移除导入样本</ElButton
            ><ElSelect v-model="importKind" aria-label="导入样本标注" @change="markImported"
              ><ElOption value="unknown" label="未标注：只查看判定" /><ElOption
                value="attack"
                label="确认均为攻击：纳入评分" /><ElOption value="benign" label="确认均为正常：检查误拦"
            /></ElSelect>
          </div>
          <details>
            <summary>逐条确认样本（历史拦截不等于恶意）</summary>
            <div v-for="sample in samples" :key="sample.id" class="sample-row">
              <code>{{ sample.id }} · {{ sample.method }} {{ sample.path }}</code
              ><ElSelect v-model="sample.kind" :aria-label="`${sample.id} 标注`"
                ><ElOption value="unknown" label="未标注" /><ElOption value="attack" label="攻击" /><ElOption
                  value="benign"
                  label="正常"
              /></ElSelect>
            </div>
          </details>
        </template>
      </fieldset>
      <fieldset v-else :disabled="busy || !config">
        <legend>单条请求判定</legend>
        <div class="score-fields">
          <label
            >请求预期<ElSelect v-model="single.kind" aria-label="请求预期"
              ><ElOption value="unknown" label="未标注" /><ElOption value="attack" label="攻击：应防御" /><ElOption
                value="benign"
                label="正常：应可访问" /></ElSelect></label
          ><label>攻击分类<ElInput v-model="single.category" aria-label="攻击分类" /></label>
        </div>
        <div class="score-fields">
          <label
            >方法<ElSelect v-model="single.method" aria-label="请求方法"
              ><ElOption
                v-for="method in ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']"
                :key="method"
                :value="method"
                :label="method" /></ElSelect></label
          ><label>路径<ElInput v-model="single.path" aria-label="请求路径" /></label>
        </div>
        <div v-for="(header, index) in headers" :key="index" class="header-row">
          <ElInput v-model="header.name" :aria-label="`请求头名称 ${index + 1}`" placeholder="请求头名称" /><ElInput
            v-model="header.value"
            :aria-label="`请求头值 ${index + 1}`"
            placeholder="请求头值"
          /><ElButton text @click="headers.splice(index, 1)">移除</ElButton>
        </div>
        <ElButton text @click="headers.push({ name: '', value: '' })">添加请求头</ElButton>
        <label
          >样例正文<ElInput v-model="single.body" type="textarea" :rows="3" aria-label="样例正文" autocomplete="off"
        /></label>
      </fieldset>
      <details>
        <summary>完整测试参数（仅本次，不保存服务）</summary>
        <label
          >测试策略 JSON<ElInput
            v-model="configJSON"
            type="textarea"
            :rows="8"
            aria-label="测试策略 JSON"
            :disabled="busy" /></label
        ><ElButton :disabled="busy || !config" @click="configJSON = JSON.stringify(config, null, 2)"
          >载入当前策略以编辑</ElButton
        ><label
          >批量请求 JSON（含每条请求的分类、预期、请求头与正文）<ElInput
            v-model="sampleJSON"
            type="textarea"
            :rows="8"
            aria-label="批量请求 JSON"
            :disabled="busy" /></label
        ><ElButton :disabled="busy" @click="sampleJSON = JSON.stringify(samples, null, 2)">载入当前样本以编辑</ElButton>
        <p class="muted">留空使用上方设置。修改只作用于隔离测试；不会保存服务、变更监听或处罚来源。</p>
      </details>
      <p v-if="busy" role="status">
        {{ progress.total ? `已检查 ${progress.done} / ${progress.total} 个来源` : '正在准备检测…' }}
      </p>
      <ElAlert v-if="error" :title="error" type="error" :closable="false" />
      <div class="score-actions">
        <ElButton
          type="primary"
          :loading="busy"
          :disabled="!config || !sources.length || sourcesLoading"
          @click="run"
          >{{ mode === 'single' ? '检查当前判定' : '开始评分检测' }}</ElButton
        ><ElButton v-if="busy" @click="cancel">取消检测</ElButton
        ><span class="muted">调整配置或样本后需重新检测；取消会保留输入。</span>
      </div>
      <template v-if="result">
        <ElAlert
          v-if="stale"
          title="输入或草稿已变化，下方为上一次结果，请重新检测。"
          type="warning"
          :closable="false"
        />
        <div class="assessment-summary" role="status">
          <div>
            <strong>{{ result.assessment.score ?? '—' }}</strong
            ><span> / 100 防御分</span>
          </div>
          <p>
            正常来源可达 {{ result.assessment.normal_passed }} / {{ result.assessment.normal_total }} ·
            {{ result.assessment.evaluations }} 次判定 · {{ result.assessment.elapsed_ms.toFixed(1) }} ms
          </p>
          <code>{{ result.assessment.target }} · v{{ result.assessment.revision }}</code>
        </div>
        <ElTable :data="result.assessment.categories" aria-label="攻击分类评分"
          ><ElTableColumn prop="category" label="攻击类别" min-width="150" /><ElTableColumn
            prop="total"
            label="样例"
            width="75" /><ElTableColumn prop="blocked" label="已防御" width="90" /><ElTableColumn
            prop="challenged"
            label="需验证"
            width="90" /><ElTableColumn prop="observed" label="仅观察" width="90" /><ElTableColumn
            prop="missed"
            label="未防御/不完整"
            min-width="130"
        /></ElTable>
        <p v-if="result.assessment.pressure.evaluations">
          隔离压力：{{ result.assessment.pressure.evaluations }} 次 · 限制 {{ result.assessment.pressure.limited }} 次 ·
          首次限制 {{ result.assessment.pressure.first_limited || '未触发' }} ·
          {{ result.assessment.pressure.elapsed_ms.toFixed(1) }} ms
        </p>
        <section v-if="result.camouflage.length">
          <h3>伪装画像联动预测</h3>
          <p class="muted">假设先触及画像端口，再访问服务。预测封禁与业务直连评分分开显示。</p>
          <ElTable :data="result.camouflage"
            ><ElTableColumn prop="name" label="画像" min-width="150" /><ElTableColumn
              prop="protocol_profile"
              label="协议" /><ElTableColumn prop="port" label="端口" /><ElTableColumn
              prop="version"
              label="版本" /><ElTableColumn label="就绪状态"
              ><template #default="{ row }">{{
                row.running ? '监听运行中' : row.enabled ? '监听未运行' : '未启用'
              }}</template></ElTableColumn
            ><ElTableColumn prop="predicted" label="预计封禁来源"
          /></ElTable>
        </section>
        <section>
          <h3>调整建议</h3>
          <ul v-if="result.assessment.advice.length">
            <li v-for="advice in result.assessment.advice" :key="advice">{{ advice }}</li>
          </ul>
          <p v-else>本轮没有发现已覆盖样本的缺口。请扩大来源、业务路径与攻击类别后再验证。</p>
        </section>
        <details open>
          <summary>评分口径与验证范围</summary>
          <p v-for="limit in result.assessment.limits" :key="limit" class="muted">{{ limit }}</p>
        </details>
        <section>
          <h3>逐条判定</h3>
          <ElTable :data="visibleRows" row-key="rowKey"
            ><ElTableColumn prop="id" label="样例" min-width="130" /><ElTableColumn
              label="来源 / 归属地"
              min-width="190"
              ><template #default="{ row }"
                >{{ row.client_ip
                }}<small>{{
                  row.geo.ok ? [row.geo.country, row.geo.province].filter(Boolean).join(' / ') : '归属地不可判定'
                }}</small></template
              ></ElTableColumn
            ><ElTableColumn label="预期" width="90"
              ><template #default="{ row }">{{
                row.expected === 'access' ? '可访问' : row.expected === 'block' ? '应防御' : '未标注'
              }}</template></ElTableColumn
            ><ElTableColumn label="结果" min-width="130"
              ><template #default="{ row }">{{ outcomeLabels[row.outcome] || row.outcome }}</template></ElTableColumn
            ><ElTableColumn type="expand"
              ><template #default="{ row }"><Trace :result="row.result" /></template></ElTableColumn></ElTable
          ><ElPagination
            v-model:current-page="page"
            :page-size="20"
            :total="result.assessment.rows.length"
            layout="prev, pager, next, total"
          />
        </section>
      </template>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue';
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElInput,
  ElInputNumber,
  ElOption,
  ElPagination,
  ElRadioButton,
  ElRadioGroup,
  ElSelect,
  ElSkeleton,
  ElTable,
  ElTableColumn,
} from 'element-plus';
import { nextTick } from 'vue';
import { runAssessment, type AssessmentProgress } from './assessmentJob';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import { camouflageGet } from '@/apis/camouflage';
import Trace from '../pipeline/Trace.vue';
import type { PortConfig } from '../pipeline/model';
import { importAssessmentLog, outcomeLabels, serviceTarget, type Sample } from './assessment';
const props = defineProps({
  draftConfig: Object as PropType<PortConfig>,
  initialPort: Number,
  initialPlan: Object as PropType<any>,
});
const services = ref<PortConfig[]>([]),
  profiles = ref<any[]>([]),
  port = ref<number | undefined>(props.initialPort);
const config = computed(() => props.draftConfig || services.value.find((service) => service.port === port.value));
const configJSON = ref(''),
  sampleJSON = ref(''),
  pressureMode = ref('mixed'),
  pressurePath = ref('/'),
  sourcePage = ref(1);
const progress = ref<AssessmentProgress>({ state: 'preparing', done: 0, total: 0 });
const mode = ref('batch'),
  profileId = ref(''),
  loading = ref(false),
  busy = ref(false),
  sourcesLoading = ref(false);
const loadError = ref(''),
  profilesError = ref(''),
  sourcesError = ref(''),
  error = ref('');
const sources = ref<Array<{ ip: string; expect_access: boolean; country?: string; province?: string }>>([]);
const samples = ref<Sample[]>([]),
  builtins = ref(true),
  pressure = ref(500),
  importKind = ref('unknown');
const single = ref({ method: 'GET', path: '/', body: '', kind: 'unknown' as Sample['kind'], category: 'custom' }),
  headers = ref<Array<{ name: string; value: string }>>([]);
const result = ref<any>(null),
  completedKey = ref(''),
  page = ref(1);
const inputKey = computed(() =>
  JSON.stringify([
    config.value,
    sources.value,
    samples.value,
    builtins.value,
    pressure.value,
    mode.value,
    profileId.value,
    single.value,
    headers.value,
    configJSON.value,
    sampleJSON.value,
    pressureMode.value,
    pressurePath.value,
  ]),
);
const stale = computed(() => inputKey.value !== completedKey.value);
const visibleRows = computed(() =>
  (result.value?.assessment.rows || [])
    .slice((page.value - 1) * 20, page.value * 20)
    .map((row: any, index: number) => ({ ...row, rowKey: index })),
);
let controller: AbortController | undefined,
  live = true,
  sourceVersion = 0;
function payload() {
  return {
    port: config.value?.port,
    expected_revision: config.value?.revision,
    ...(configJSON.value.trim()
      ? { config: JSON.parse(configJSON.value) }
      : props.draftConfig
        ? { config: props.draftConfig }
        : {}),
  };
}
async function loadProfiles() {
  profilesError.value = '';
  try {
    const data = await camouflageGet<any>('config');
    if (live) profiles.value = data.config.profiles;
  } catch (cause: any) {
    if (live) profilesError.value = cause.message || '画像读取失败，请重试';
  }
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
    if (live) loadError.value = cause.message || '服务读取失败，请重试';
  } finally {
    if (live) loading.value = false;
  }
  await loadProfiles();
}
async function sampleSources() {
  if (!config.value || sourcesLoading.value) return;
  const version = ++sourceVersion;
  const key = JSON.stringify(config.value);
  sourcesLoading.value = true;
  sourcesError.value = '';
  try {
    const data = await pipelinePost<any>('assessment/sources', payload());
    if (live && version === sourceVersion && key === JSON.stringify(config.value)) applyPlan(data);
  } catch (cause: any) {
    if (live && version === sourceVersion) sourcesError.value = cause.message || '数据库取样失败，请手动添加 IP 或重试';
  } finally {
    if (live && version === sourceVersion) sourcesLoading.value = false;
  }
}
function addSource() {
  sources.value.push({ ip: '', expect_access: true });
}
async function importLog(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  error.value = '';
  try {
    if (file.size > 4 * 1024 * 1024) throw new Error('日志文件不能超过 4 MiB');
    const parsed = importAssessmentLog(JSON.parse(await file.text()));
    if (live) {
      samples.value = parsed;
      importKind.value = 'unknown';
    }
  } catch (cause: any) {
    error.value = cause.message || '日志导入失败';
  } finally {
    input.value = '';
  }
}
function markImported() {
  for (const sample of samples.value) sample.kind = importKind.value as Sample['kind'];
}
function cancel() {
  controller?.abort();
}
async function run() {
  if (busy.value || !config.value) return;
  error.value = '';
  busy.value = true;
  const key = inputKey.value;
  controller = new AbortController();
  try {
    const inputSamples: Sample[] =
      mode.value === 'single'
        ? [
            {
              id: 'single',
              ...single.value,
              headers: Object.fromEntries(
                headers.value
                  .filter((header) => header.name.trim())
                  .map((header) => [header.name.trim(), header.value]),
              ),
              truncated: false,
            },
          ]
        : sampleJSON.value.trim()
          ? JSON.parse(sampleJSON.value)
          : samples.value;
    const data = {
      ...payload(),
      sources: sources.value.map(({ ip, expect_access }) => ({ ip, expect_access })),
      samples: inputSamples,
      builtins: mode.value === 'batch' && builtins.value,
      pressure: mode.value === 'batch' ? pressure.value : 0,
      profile_id: profileId.value,
      pressure_mode: pressureMode.value,
      pressure_path: pressurePath.value,
    };
    if (new TextEncoder().encode(JSON.stringify(data)).length > 950000)
      throw new Error('请求数据超过检测接口上限，请减少样本或正文后重试');
    const assessment = await runAssessment(data, controller.signal, (value) => {
      if (live) progress.value = value;
    });
    if (live) {
      result.value = assessment;
      completedKey.value = key;
      page.value = 1;
    }
  } catch (cause: any) {
    if (live)
      error.value = controller.signal.aborted
        ? '检测已取消，输入已保留。'
        : cause.message || '检测失败，输入已保留，请重试。';
  } finally {
    if (live) busy.value = false;
  }
}
watch(
  () => config.value?.port,
  () => {
    sourceVersion++;
    sourcesLoading.value = false;
    sources.value = [];
    configJSON.value = '';
    sampleJSON.value = '';
    sourcePage.value = 1;
    result.value = null;
  },
);
function applyPlan(plan: any) {
  sources.value = plan.sources || [];
  samples.value = plan.samples || [];
  pressure.value = plan.pressure || 0;
  builtins.value = false;
  sourcePage.value = 1;
  sourcesError.value = (plan.warnings || []).join(' ');
}
onMounted(async () => {
  await load();
  await nextTick();
  if (props.initialPlan) applyPlan(props.initialPlan);
  else if (config.value) await sampleSources();
});
onBeforeUnmount(() => {
  live = false;
  sourceVersion++;
  cancel();
});
</script>

<style scoped>
.score-detection {
  display: grid;
  gap: 18px;
  min-width: 0;
  color: var(--hw-ink);
}
.refresh-config {
  justify-self: start;
}
h2,
h3,
p {
  margin: 0;
}
h2 {
  font-size: 18px;
}
h3 {
  font-size: 16px;
}
header,
section,
details {
  display: grid;
  gap: 12px;
  min-width: 0;
}
header p,
.muted,
small {
  color: var(--hw-muted);
  line-height: 1.7;
}
.score-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
label {
  display: grid;
  gap: 7px;
  min-width: 0;
  font-size: 14px;
}
fieldset {
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  margin: 0;
  padding: 16px;
  display: grid;
  gap: 14px;
  min-width: 0;
}
legend {
  padding: 0 8px;
  font-weight: 600;
}
.score-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}
.score-actions .el-select {
  width: 250px;
}
.source-row {
  display: grid;
  grid-template-columns: minmax(150px, 1fr) minmax(100px, 1fr) 170px auto;
  gap: 12px;
  align-items: end;
}
.target-line {
  display: grid;
  gap: 6px;
}
code {
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
}
.sample-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 120px;
  gap: 12px;
  align-items: center;
}
.header-row {
  display: flex;
  gap: 12px;
}
.assessment-summary {
  display: grid;
  gap: 8px;
  padding: 18px;
  background: var(--hw-subtle);
  border-left: 3px solid var(--hw-accent);
}
.assessment-summary strong {
  font: 700 32px var(--font-mono);
}
small {
  display: block;
}
li {
  margin: 6px 0;
  line-height: 1.7;
}
summary {
  cursor: pointer;
}
input[type='file'] {
  max-width: 100%;
}
.el-pagination {
  max-width: 100%;
  flex-wrap: wrap;
}
@media (max-width: 760px) {
  .score-fields,
  .source-row {
    grid-template-columns: 1fr;
  }
  .header-row {
    flex-wrap: wrap;
  }
  fieldset {
    padding: 12px;
  }
  .score-actions {
    align-items: stretch;
  }
  .score-actions .el-select {
    width: 100%;
  }
}
</style>
