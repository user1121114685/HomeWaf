<template>
  <section class="single-test" data-testid="score-detection" :aria-busy="busy || loading">
    <ElAlert v-if="loadError" :title="loadError" type="error" :closable="false"
      ><ElButton text @click="load">重新读取服务</ElButton></ElAlert
    >
    <ElSkeleton v-if="loading" :rows="3" animated />
    <template v-else>
      <div class="service-line">
        <label v-if="!draftConfig"
          >我的服务<ElSelect v-model="port" aria-label="检测服务" :disabled="busy"
            ><ElOption
              v-for="s in services"
              :key="s.port"
              :value="s.port"
              :label="(s.name || '服务') + ' · ' + s.port" /></ElSelect
        ></label>
        <div v-if="config">
          <strong
            >{{ draftConfig ? '当前未保存草稿' : '已保存配置' }} · {{ config.name }} · v{{ config.revision }}</strong
          ><code>{{ target }}</code>
        </div>
        <ElButton v-if="!draftConfig" text :disabled="busy" @click="load">刷新配置</ElButton>
      </div>
      <p v-if="!config && !loadError">
        {{ initialPort ? '未找到原窗口所选服务，请重新选择。' : '请先在「我的服务」中添加要检测的服务。' }}
      </p>
      <form v-if="config" @submit.prevent="run" autocomplete="off">
        <fieldset :disabled="busy">
          <legend>请求行与来源</legend>
          <div class="row">
            <label
              >方法<ElSelect v-model="draft.method" filterable allow-create default-first-option aria-label="请求方法"
                ><ElOption v-for="m in methods" :key="m" :value="m" /></ElSelect
            ></label>
            <label
              >协议<ElSelect v-model="draft.scheme" aria-label="请求协议"
                ><ElOption label="跟随服务" value="" /><ElOption label="HTTP" value="http" /><ElOption
                  label="HTTPS"
                  value="https" /></ElSelect
            ></label>
            <label
              >HTTP 版本<ElSelect v-model="draft.version" aria-label="HTTP 版本"
                ><ElOption v-for="v in versions" :key="v" :value="v" /></ElSelect
            ></label>
          </div>
          <label
            >Host / :authority<ElInput
              v-model="draft.host"
              aria-label="请求 Host"
              :placeholder="authority"
              maxlength="255"
          /></label>
          <p class="hint">
            默认取所选服务的域名，无域名时用 IP 和端口。修改 Host 只改变规则看到的值，不会更换服务或连接该地址。
          </p>
          <div class="pair">
            <label>请求路径<ElInput v-model="draft.path" aria-label="请求路径" placeholder="/" /></label
            ><label
              >查询串<ElInput v-model="draft.query" aria-label="查询串" placeholder="tag=one&amp;tag=two"
            /></label>
          </div>
          <p class="hint">查询串不含开头的问号，保留原始编码、顺序和重复参数。</p>
          <div class="pair">
            <label>来源 IP<ElInput v-model="draft.source_ip" aria-label="来源 IP" /></label
            ><label
              >来源端口<ElInputNumber
                v-model="draft.source_port"
                aria-label="来源端口"
                :min="1"
                :max="65535"
                :precision="0"
            /></label>
          </div>
        </fieldset>
        <fieldset :disabled="busy">
          <legend>请求头与正文</legend>
          <p class="hint">
            Cookie、Authorization、User-Agent、Origin、Referer、Content-Type
            均可按原值填写，也可不传。转发头只按服务的可信代理策略解释。
          </p>
          <div v-for="(header, i) in draft.headers" :key="i" class="header-row">
            <label>名称<ElInput v-model="header.name" :aria-label="'请求头名称 ' + (i + 1)" /></label>
            <label>值<ElInput v-model="header.value" :aria-label="'请求头值 ' + (i + 1)" autocomplete="off" /></label>
            <ElButton text :aria-label="'移除请求头 ' + (i + 1)" @click="draft.headers.splice(i, 1)">移除</ElButton>
          </div>
          <div>
            <ElButton :disabled="draft.headers.length >= 64" @click="draft.headers.push({ name: '', value: '' })"
              >添加请求头</ElButton
            >
          </div>
          <label
            >正文输入格式<ElSelect v-model="draft.body_encoding" aria-label="正文输入格式"
              ><ElOption value="text" label="文本（含 JSON、表单等）" /><ElOption
                value="base64"
                label="Base64 编码的原始字节" /></ElSelect
          ></label>
          <label
            >请求正文<ElInput v-model="draft.body" type="textarea" :rows="4" aria-label="请求正文" autocomplete="off"
          /></label>
          <p class="hint">
            正文最多 64 KiB，长度按实际字节计算。HTTP 版本和协议是模拟属性，不进行 TLS / QUIC 握手，也不模拟传输分帧。
          </p>
        </fieldset>
        <details>
          <summary>从日志选取一条请求</summary>
          <label
            >请求日志 JSON（最多 4 MiB）<input
              type="file"
              accept=".json,application/json"
              :disabled="busy"
              @change="readLog"
          /></label>
          <div v-if="logs.length" class="service-line">
            <label
              >选取记录<ElSelect v-model="logIndex" aria-label="选取日志记录"
                ><ElOption
                  v-for="(s, i) in logs"
                  :key="i"
                  :value="i"
                  :label="i + 1 + '. ' + s.method + ' ' + s.path" /></ElSelect></label
            ><ElButton :disabled="busy" @click="applyLog">填入当前请求</ElButton>
          </div>
          <p class="hint">
            仅所选记录会填入编辑器，不会自动执行。历史域名和凭据类请求头不导入；请检查正文及查询串中的敏感信息。
          </p>
        </details>
        <details>
          <summary>预期结果、画像与测试策略</summary>
          <div class="pair">
            <label
              >预期判定<ElSelect v-model="expected" aria-label="预期判定"
                ><ElOption label="只查看判定" value="" /><ElOption
                  v-for="k in ['passed', 'blocked', 'observe', 'challenge']"
                  :key="k"
                  :label="outcomeLabels[k]"
                  :value="k" /></ElSelect
            ></label>
            <label
              >假设先命中画像<ElSelect v-model="profileId" clearable aria-label="联动伪装画像" placeholder="不模拟联动"
                ><ElOption v-for="p in profiles" :key="p.id" :label="p.name + ' · ' + p.port" :value="p.id" /></ElSelect
            ></label>
          </div>
          <ElAlert v-if="profileError" type="warning" :title="profileError" :closable="false"
            ><ElButton text @click="loadProfiles">重读画像</ElButton></ElAlert
          >
          <label
            >测试策略 JSON（可选）<ElInput
              v-model="configJSON"
              type="textarea"
              :rows="4"
              :disabled="busy"
              aria-label="测试策略 JSON"
          /></label>
          <div>
            <ElButton :disabled="busy" @click="configJSON = JSON.stringify(config, null, 2)"
              >载入当前策略以编辑</ElButton
            >
          </div>
          <p class="hint">留空使用上方配置。策略修改仅用于本次判定，不保存到服务。</p>
        </details>
        <ElAlert v-if="error" :title="error" type="error" :closable="false" />
        <div class="actions">
          <ElButton native-type="submit" type="primary" :loading="busy" :disabled="!!loadError">判定这一条请求</ElButton
          ><ElButton v-if="busy" @click="controller?.abort()">取消检测</ElButton
          ><span class="hint">一次点击只判定一次，不生成压力流量。</span>
        </div>
      </form>
      <section v-if="result" aria-label="单条判定结果" class="decision">
        <ElAlert v-if="stale" type="warning" :closable="false" title="输入或配置已变化，下方为上一次判定。" />
        <h2>判定结果：{{ outcomeLabels[result.decision.outcome] || result.decision.outcome }}</h2>
        <p v-if="completedExpected">
          预期{{ outcomeLabels[completedExpected] }} ·
          {{ completedExpected === result.decision.outcome ? '符合预期' : '与预期不符' }}
        </p>
        <p class="hint">
          连接来源 {{ result.decision.peer }} · 策略识别 IP {{ result.decision.client_ip }} · 正文
          {{ result.decision.body_bytes }} 字节 · 配置 v{{ result.decision.revision }}
        </p>
        <Trace :result="result.decision.result" />
        <p v-if="result.profile">
          画像 {{ result.profile.name }}：{{
            result.profile.local
              ? '局域网豁免'
              : result.profile.running
                ? '命中画像端口后预计封禁'
                : '未运行，不计入联动防御'
          }}。这是单独预测，未执行封禁。
        </p>
      </section>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch, type PropType } from 'vue';
import { ElAlert, ElButton, ElInput, ElInputNumber, ElOption, ElSelect, ElSkeleton } from 'element-plus';
import { pipelineGet } from '@/apis/pipeline';
import { camouflageGet } from '@/apis/camouflage';
import request from '@/utils/request';
import Trace from '../pipeline/Trace.vue';
import type { PortConfig } from '../pipeline/model';
import { importAssessmentLog, outcomeLabels, serviceTarget, type Sample } from './assessment';
const props = defineProps({ draftConfig: Object as PropType<PortConfig>, initialPort: Number, initialSource: String });
const services = ref<PortConfig[]>([]),
  port = ref(props.initialPort),
  profiles = ref<Array<{ id: string; name: string; port: number }>>([]);
const config = computed(() => props.draftConfig || services.value.find((s) => s.port === port.value));
const target = computed(() => (config.value ? serviceTarget(config.value) : ''));
const authority = computed(() => target.value.split('://')[1]);
const versions = ['HTTP/1.0', 'HTTP/1.1', 'HTTP/2.0', 'HTTP/3.0'];
const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'TRACE', 'CONNECT'];
function defaults() {
  return {
    source_ip: props.initialSource || '192.0.2.25',
    source_port: 12345,
    scheme: '',
    version: 'HTTP/1.1',
    method: 'GET',
    host: '',
    path: '/',
    query: '',
    headers: [] as { name: string; value: string }[],
    body_encoding: 'text',
    body: '',
  };
}
const draft = reactive(defaults()),
  expected = ref(''),
  profileId = ref(''),
  configJSON = ref(''),
  busy = ref(false),
  loading = ref(false),
  loadError = ref(''),
  profileError = ref(''),
  error = ref('');
const logs = ref<Sample[]>([]),
  logIndex = ref(0);
type Decision = {
  peer: string;
  client_ip: string;
  outcome: string;
  revision: number;
  body_bytes: number;
  result: Record<string, unknown>;
};
type ResponseData = { decision: Decision; profile?: { name: string; local: number; running: boolean } };
const result = ref<ResponseData | null>(null),
  completedExpected = ref(''),
  generation = ref(0),
  lastGeneration = ref(-1);
const stale = computed(() => generation.value !== lastGeneration.value);
let live = true,
  controller: AbortController | undefined;
watch(
  [config, draft, expected, profileId, configJSON],
  () => {
    generation.value++;
  },
  { deep: true, flush: 'sync' },
);
watch(port, (next, previous) => {
  if (previous !== undefined && previous !== next) {
    Object.assign(draft, defaults());
    configJSON.value = '';
    profileId.value = '';
    expected.value = '';
    logs.value = [];
    result.value = null;
    error.value = '';
  }
});
async function loadProfiles() {
  try {
    const data = await camouflageGet<any>('config');
    if (live) {
      profiles.value = data.config.profiles;
      profileError.value = '';
    }
  } catch {
    if (live) profileError.value = '无法读取画像，可重试或只判定业务请求。';
  }
}
async function load() {
  if (loading.value) return;
  loading.value = true;
  loadError.value = '';
  try {
    if (!props.draftConfig) {
      const data = await pipelineGet<PortConfig[]>('ports');
      if (!live) return;
      services.value = data;
      if (port.value === undefined) port.value = data[0]?.port;
    }
  } catch (cause: any) {
    if (live) loadError.value = cause.message || '服务读取失败，请重试';
  } finally {
    if (live) loading.value = false;
  }
}
async function readLog(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  const at = port.value;
  try {
    if (file.size > 4 * 1024 * 1024) throw new Error('日志超过 4 MiB');
    const rows = importAssessmentLog(JSON.parse(await file.text()));
    if (live && port.value === at) {
      logs.value = rows;
      logIndex.value = 0;
      error.value = '';
    }
  } catch (cause: any) {
    if (live) error.value = cause.message || '无法读取日志';
  } finally {
    input.value = '';
  }
}
function applyLog() {
  const s = logs.value[logIndex.value];
  if (!s) return;
  const at = s.path.indexOf('?');
  draft.method = s.method;
  draft.path = at < 0 ? s.path : s.path.slice(0, at);
  draft.query = at < 0 ? '' : s.path.slice(at + 1);
  draft.host = '';
  draft.body_encoding = 'text';
  draft.body = s.body;
  draft.headers = Object.entries(s.headers)
    .filter(([k]) => !['origin', 'referer'].includes(k.toLowerCase()))
    .map(([name, value]) => ({ name, value }));
  if (s.truncated) error.value = '历史正文已截断，只能判定当前填入的内容。';
  else error.value = '';
}
async function run() {
  if (!config.value || busy.value || loadError.value) return;
  busy.value = true;
  error.value = '';
  const submitted = generation.value,
    expectation = expected.value;
  controller = new AbortController();
  try {
    const payload = {
      port: config.value.port,
      expected_revision: config.value.revision,
      config: configJSON.value.trim() ? JSON.parse(configJSON.value) : props.draftConfig,
      profile_id: profileId.value,
      request: { ...draft, headers: draft.headers.filter((h) => h.name || h.value) },
    };
    const reply = (await request({
      url: '/pipeline/assessment/single',
      method: 'post',
      data: payload,
      signal: controller.signal,
      timeout: 20000,
    })) as any;
    if (reply.code !== 0) throw new Error(reply.msg || '判定失败');
    if (!reply.data?.decision?.result?.verdict) throw new Error('服务未返回完整判定，请检查服务版本后重试。');
    if (live) {
      result.value = reply.data;
      lastGeneration.value = submitted;
      completedExpected.value = expectation;
    }
  } catch (cause: any) {
    if (live)
      error.value = controller.signal.aborted
        ? '已取消等待，输入保留。'
        : cause instanceof SyntaxError
          ? '策略 JSON 无效，请检查后重试。'
          : cause.message || '判定失败，输入已保留。';
  } finally {
    if (live) busy.value = false;
  }
}
onMounted(() => {
  void load();
  void loadProfiles();
});
onBeforeUnmount(() => {
  live = false;
  controller?.abort();
});
</script>
<style scoped>
.single-test,
form,
fieldset,
.decision,
details[open] {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.single-test {
  max-width: 1080px;
  color: var(--hw-ink);
}
fieldset,
.decision,
details {
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  padding: 16px 20px;
  margin: 0;
}
legend {
  padding: 0 7px;
  font-weight: 600;
}
h2,
p {
  margin: 0;
}
h2 {
  font-size: 18px;
}
.hint {
  font-size: 13px;
  line-height: 1.6;
  color: var(--hw-muted);
}
label,
.service-line > div {
  display: grid;
  gap: 6px;
  min-width: 0;
}
code {
  overflow-wrap: anywhere;
}
.service-line,
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  align-items: center;
}
.service-line label {
  min-width: min(230px, 100%);
}
.row,
.pair,
.header-row {
  display: grid;
  gap: 14px;
  align-items: end;
}
.row {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.pair {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.header-row {
  grid-template-columns: minmax(100px, 1fr) minmax(180px, 2fr) auto;
}
summary {
  cursor: pointer;
  font-weight: 600;
}
input[type='file'] {
  max-width: 100%;
}
.actions {
  padding: 4px 0;
}
.decision {
  border-inline-start: 3px solid var(--hw-accent);
}
@media (max-width: 700px) {
  .row,
  .pair,
  .header-row {
    grid-template-columns: 1fr;
  }
  .service-line {
    align-items: stretch;
    flex-direction: column;
  }
  fieldset,
  .decision,
  details {
    padding: 12px;
  }
  .header-row > button {
    justify-self: start;
  }
}
</style>
