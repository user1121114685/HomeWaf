<template>
  <div class="guided-setup" data-testid="guided-setup" :aria-busy="busy">
    <ol class="setup-steps" aria-label="接入进度">
      <li
        v-for="(label, index) in ['选择服务', '连接服务', '确认配置']"
        :key="label"
        :class="{ current: step === index, complete: step > index }"
        :aria-current="step === index ? 'step' : undefined"
      >
        <span>{{ step > index ? '✓' : index + 1 }}</span
        >{{ label }}
      </li>
    </ol>
    <div v-if="error" class="setup-error" role="alert">{{ error }}</div>
    <div :inert="busy || undefined">
      <section v-if="step === 0" class="setup-section">
        <h2>选择要保护的服务</h2>
        <p class="setup-lead">选一个使用场景，HomeWaf 会带入对应配置。</p>
        <div class="scene-options" role="radiogroup" aria-label="服务场景">
          <label v-if="!config.template?.id" class="scene-option" :class="{ selected: usingDraft }"
            ><input v-model="selectedTemplate" type="radio" name="service-scene" value="__draft__" /><Connection
              class="scene-icon"
              aria-hidden="true"
            /><strong>保留复制的配置</strong><span>沿用来源服务的策略，凭据按需重新设置。</span
            ><span class="scene-check" aria-hidden="true">{{ usingDraft ? '✓' : '' }}</span></label
          >
          <label
            v-for="scene in availableScenes"
            :key="scene.id"
            class="scene-option"
            :class="{ selected: selectedTemplate === scene.id }"
          >
            <input v-model="selectedTemplate" type="radio" name="service-scene" :value="scene.id" />
            <component :is="sceneIcons[scene.icon]" class="scene-icon" aria-hidden="true" />
            <strong>{{ scene.name }}</strong
            ><span>{{ scene.description }}</span>
            <span class="scene-check" aria-hidden="true">{{ selectedTemplate === scene.id ? '✓' : '' }}</span>
          </label>
        </div>
        <details v-if="otherTemplates.length" class="setup-details">
          <summary>使用其他策略模板</summary>
          <label class="setup-field"
            ><span>策略模板</span
            ><ElSelect v-model="selectedTemplate" aria-label="策略模板"
              ><ElOption v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" /></ElSelect
          ></label>
        </details>
        <p class="setup-note">
          {{
            usingDraft
              ? '保留复制后的策略草稿，新的服务仍需独立检查和启用。'
              : selected
                ? templateDescription(selected)
                : '请选择一个可用的模板。'
          }}
        </p>
      </section>
      <section v-else-if="step === 1" class="setup-section">
        <h2>连接现有服务</h2>
        <p class="setup-lead">填写现在能访问这个服务的地址，其余设置已按场景带入。</p>
        <label class="setup-field"
          ><span>现有服务地址 <em>必填</em></span>
          <ElInput
            ref="addressInput"
            v-model="draft.upstream"
            placeholder="例如 192.168.1.20:5000"
            aria-label="现有服务地址"
            :aria-invalid="!!fieldErrors.upstream"
            aria-describedby="upstream-help upstream-error"
            autocomplete="off"
            @keydown.enter.prevent="next"
          />
          <small id="upstream-help">这是原来服务的地址，不是 HomeWaf 管理地址。未填写协议时使用 HTTP。</small>
          <span v-if="fieldErrors.upstream" id="upstream-error" class="field-error" role="alert">{{
            fieldErrors.upstream
          }}</span>
        </label>
        <div class="connection-preview" aria-label="请求转发路径">
          <span>访客</span><ArrowRight aria-hidden="true" /><span
            >HomeWaf <b>:{{ draft.port }}</b></span
          ><ArrowRight aria-hidden="true" /><span>{{ draft.upstream || '你的服务' }}</span>
        </div>
        <AccessRestrictions ref="accessRestrictions" :policy="draft.policy" />
        <div v-if="draft.policy.gate.enabled" class="setup-account">
          <h3>设置访问账号</h3>
          <p>访客先用这个账号登录，再进入你的服务。它与 HomeWaf 管理账号独立。</p>
          <div class="setup-columns">
            <label class="setup-field"
              ><span>访问账号</span
              ><ElInput
                ref="usernameInput"
                v-model="secrets.initial_username"
                aria-label="访问账号"
                autocomplete="off"
                maxlength="128"
                :aria-invalid="!!fieldErrors.username"
                aria-describedby="username-error"
              /><span v-if="fieldErrors.username" id="username-error" class="field-error" role="alert">{{
                fieldErrors.username
              }}</span></label
            >
            <label class="setup-field"
              ><span>访问密码</span
              ><ElInput
                ref="passwordInput"
                v-model="secrets.initial_password"
                type="password"
                show-password
                aria-label="访问密码"
                autocomplete="new-password"
                maxlength="72"
                :aria-invalid="!!fieldErrors.password"
                aria-describedby="password-help password-error"
              /><small id="password-help">至少 10 个字符，避免常见弱密码。</small
              ><span v-if="fieldErrors.password" id="password-error" class="field-error" role="alert">{{
                fieldErrors.password
              }}</span></label
            >
          </div>
        </div>
        <p v-if="draft.policy.gate.require_token" class="setup-note">
          此场景需要服务令牌。保存后，在服务配置中生成令牌，再启用服务。
        </p>
        <details class="setup-details">
          <summary>连接选项 <span>名称、端口、HTTPS 与域名</span></summary>
          <div class="setup-columns">
            <label class="setup-field"
              ><span>服务名称 <em>可选</em></span
              ><ElInput v-model="draft.name" placeholder="留空自动命名" aria-label="服务名称" maxlength="100"
            /></label>
            <label class="setup-field"
              ><span>HomeWaf 监听端口</span
              ><ElInputNumber
                v-model="draft.port"
                :min="1"
                :max="65535"
                controls-position="right"
                aria-label="HomeWaf 监听端口"
              /><small>已避开已知配置，下一步检查实际占用情况。</small></label
            >
            <label class="setup-field"
              ><span>访客连接协议</span
              ><ElSelect v-model="draft.protocol" aria-label="访客连接协议"
                ><ElOption label="HTTP" value="http" /><ElOption label="HTTPS（需要证书）" value="https" /></ElSelect
              ><small>仅影响访客到 HomeWaf 的连接，与现有服务协议独立。</small></label
            >
            <label class="setup-field"
              ><span>允许访问的域名 <em>可选</em></span
              ><ElInput v-model="hostNames" placeholder="例如 nas.example.com" aria-label="允许访问的域名" /><small
                >多个域名用逗号分隔。留空接受所有域名。</small
              ></label
            >
          </div>
          <div v-if="draft.protocol === 'https'" class="setup-columns certificate-fields">
            <label class="setup-field"
              ><span>TLS 证书 PEM</span
              ><ElInput v-model="secrets.tls_certificate" type="textarea" :rows="4" aria-label="TLS 证书 PEM"
            /></label>
            <label class="setup-field"
              ><span>TLS 私钥 PEM</span
              ><ElInput
                v-model="secrets.tls_private_key"
                type="textarea"
                :rows="4"
                aria-label="TLS 私钥 PEM"
                autocomplete="off"
            /></label>
          </div>
        </details>
      </section>
      <section v-else class="setup-section">
        <h2>确认服务配置</h2>
        <p class="setup-lead">连接检查已完成。保存后，再由你决定何时启用。</p>
        <dl class="setup-review">
          <div>
            <dt>服务名称</dt>
            <dd>{{ draft.name }}</dd>
          </div>
          <div>
            <dt>现有服务</dt>
            <dd>{{ draft.upstream }}</dd>
          </div>
          <div>
            <dt>新入口</dt>
            <dd>{{ draft.protocol.toUpperCase() }} · HomeWaf 所在机器的 {{ draft.port }} 端口</dd>
          </div>
          <div>
            <dt>策略模板</dt>
            <dd>{{ selected?.name || '当前草稿' }}</dd>
          </div>
          <div>
            <dt>访问认证</dt>
            <dd>
              {{
                draft.policy.gate.require_token
                  ? '需要服务令牌，保存后生成'
                  : draft.policy.gate.enabled
                    ? `独立账号 · ${secrets.initial_username}`
                    : '访客直接访问，继续进行防护检测'
              }}
            </dd>
          </div>
          <div>
            <dt>来源限制</dt>
            <dd>{{ accessSourceSummary(draft.policy) }}</dd>
          </div>
          <div>
            <dt>IP 协议</dt>
            <dd>
              {{
                draft.policy.network.listen_ip_version === 'ipv4'
                  ? '仅 IPv4'
                  : draft.policy.network.listen_ip_version === 'ipv6'
                    ? '仅 IPv6'
                    : 'IPv4 + IPv6'
              }}
            </dd>
          </div>
        </dl>
        <div class="setup-check-result">
          <CircleCheck aria-hidden="true" />
          <div>
            <strong>端口检查与 TCP 连接通过</strong>
            <p>这不代表业务登录或 HTTPS 证书已经验证，请在启用后实际访问一次。</p>
          </div>
        </div>
        <details class="setup-details">
          <summary>查看请求检测结果</summary>
          <Trace :result="preview" />
        </details>
        <p class="setup-note">
          {{
            draft.policy.gate.require_token
              ? '下一步：生成服务令牌，然后启用服务。'
              : '下一步：启用服务，并将实际访问指向 HomeWaf 的新入口。'
          }}原服务地址仍可直接访问。
        </p>
      </section>
    </div>
    <footer class="setup-actions">
      <span role="status">{{ busy ? progress : `第 ${step + 1} 步，共 3 步` }}</span>
      <ElButton
        v-if="step > 0"
        :disabled="busy"
        @click="
          step--;
          error = '';
        "
        >上一步</ElButton
      >
      <ElButton v-if="step < 2" type="primary" :loading="busy" :disabled="!selected && !usingDraft" @click="next"
        >{{ step === 0 ? '继续填写地址' : '检查连接并继续' }}<ArrowRight class="button-icon" aria-hidden="true"
      /></ElButton>
      <ElButton v-else type="primary" :loading="busy" :disabled="!preview || created" @click="save"
        >保存服务配置</ElButton
      >
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { ElButton, ElInput, ElInputNumber, ElOption, ElSelect } from 'element-plus';
import { ArrowRight, CircleCheck, Connection, FolderOpened, Monitor, VideoCamera } from '@element-plus/icons-vue';
import { pipelinePost } from '@/apis/pipeline';
import { clone, parseLines, type PolicyTemplate, type PortConfig } from './model';
import {
  normalizeServiceAddress,
  optionalAccessDefaults,
  serviceScenes,
  suggestedName,
  templateDescription,
} from './onboarding';
import Trace from './Trace.vue';
import AccessRestrictions from './AccessRestrictions.vue';
import { accessSourceSummary } from './accessPolicy';

const props = defineProps({
  config: { type: Object as PropType<PortConfig>, required: true },
  templates: { type: Array as PropType<PolicyTemplate[]>, required: true },
});
const emit = defineEmits(['saved', 'busy']);
const draft = ref(clone(props.config));
if (props.config.template?.id) optionalAccessDefaults(draft.value);
const accessRestrictions = ref<any>();
const selectedTemplate = ref(props.config.template?.id || '__draft__');
const usingDraft = computed(() => selectedTemplate.value === '__draft__');
const selected = computed(() => props.templates.find((tpl) => tpl.id === selectedTemplate.value));
const availableScenes = computed(() =>
  serviceScenes.filter((scene) => props.templates.some((tpl) => tpl.id === scene.id)),
);
const otherTemplates = computed(() =>
  props.templates.filter((tpl) => !serviceScenes.some((scene) => scene.id === tpl.id)),
);
const sceneIcons = { Connection, FolderOpened, Monitor, VideoCamera };
const step = ref(0),
  busy = ref(false),
  created = ref(false),
  error = ref(''),
  progress = ref('');
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
const preview = ref<any>(null),
  previewKey = ref('');
const hostNames = ref(draft.value.host_names.join(', '));
const secrets = reactive({ initial_username: '', initial_password: '', tls_certificate: '', tls_private_key: '' });
const fieldErrors = reactive({ upstream: '', username: '', password: '' });
const addressInput = ref<any>(),
  usernameInput = ref<any>(),
  passwordInput = ref<any>();
const fingerprint = () => JSON.stringify({ config: draft.value, secrets });
watch(
  [draft, secrets, hostNames],
  () => {
    preview.value = null;
    previewKey.value = '';
  },
  { deep: true, flush: 'sync' },
);
async function run(work: () => Promise<void>) {
  if (busy.value || created.value) return;
  busy.value = true;
  error.value = '';
  try {
    await work();
  } catch (cause: any) {
    error.value = cause?.message || '操作未完成，请重试。已填写的内容会保留。';
  } finally {
    busy.value = false;
    progress.value = '';
  }
}
async function next() {
  await run(async () => {
    if (step.value === 0) {
      if (!selected.value && !usingDraft.value) throw new Error('请选择一个可用的服务模板。');
      if (usingDraft.value)
        draft.value = { ...draft.value, policy: clone(props.config.policy), template: clone(props.config.template) };
      if (!usingDraft.value && draft.value.template?.id !== selectedTemplate.value) {
        progress.value = '正在准备场景配置…';
        const result = await pipelinePost<{ config: PortConfig }>('templates/preview', {
          config: draft.value,
          template_id: selectedTemplate.value,
          sections: [],
        });
        draft.value = result.config;
        optionalAccessDefaults(draft.value);
      }
      step.value = 1;
      await nextTick();
      addressInput.value?.focus();
      return;
    }
    fieldErrors.upstream = '';
    fieldErrors.username = '';
    fieldErrors.password = '';
    try {
      draft.value.upstream = normalizeServiceAddress(draft.value.upstream);
    } catch (cause: any) {
      fieldErrors.upstream = cause.message;
    }
    if (draft.value.policy.gate.enabled) {
      if (!secrets.initial_username.trim()) fieldErrors.username = '填写供访客使用的访问账号。';
      if (secrets.initial_password.length < 10) fieldErrors.password = '访问密码至少需要 10 个字符。';
    }
    if (Object.values(fieldErrors).some(Boolean)) {
      await nextTick();
      (fieldErrors.upstream ? addressInput : fieldErrors.username ? usernameInput : passwordInput).value?.focus();
      return;
    }
    if (!accessRestrictions.value?.validate()) throw new Error('请完成所选的访问限制，或取消不需要的选项。');
    if (!Number.isInteger(draft.value.port) || draft.value.port < 1 || draft.value.port > 65535)
      throw new Error('请在连接选项中填写 1–65535 之间的端口。');
    if (draft.value.protocol === 'https' && (!secrets.tls_certificate.trim() || !secrets.tls_private_key.trim()))
      throw new Error('已选择 HTTPS，请在连接选项中填写 TLS 证书和私钥。');
    draft.value.name = draft.value.name.trim() || suggestedName(selectedTemplate.value, draft.value.upstream);
    draft.value.host_names = parseLines(hostNames.value);
    secrets.initial_username = secrets.initial_username.trim();
    progress.value = '正在检查端口是否可用…';
    await pipelinePost('ports/check', draft.value);
    progress.value = '正在从 HomeWaf 连接你的服务…';
    await pipelinePost('upstream/test', { upstream: draft.value.upstream });
    progress.value = '正在检查访问策略…';
    preview.value = await pipelinePost('explain', {
      config: draft.value,
      client_ip: '198.51.100.25',
      method: 'GET',
      path: '/',
      headers: { Accept: 'text/html' },
      body: '',
    });
    previewKey.value = fingerprint();
    step.value = 2;
  });
  if (step.value === 1 && !busy.value) {
    await nextTick();
    (fieldErrors.username && !fieldErrors.upstream
      ? usernameInput
      : fieldErrors.password && !fieldErrors.upstream
        ? passwordInput
        : addressInput
    ).value?.focus();
  }
}
async function save() {
  await run(async () => {
    if (!preview.value || previewKey.value !== fingerprint()) throw new Error('配置已变化，请返回上一步重新检查。');
    progress.value = '正在保存服务配置…';
    const result = await pipelinePost<PortConfig>('ports/save', {
      config: { ...draft.value, listen_enabled: false },
      initial_username: draft.value.policy.gate.enabled ? secrets.initial_username : '',
      initial_password: draft.value.policy.gate.enabled ? secrets.initial_password : '',
      tls_certificate: draft.value.protocol === 'https' ? secrets.tls_certificate : '',
      tls_private_key: draft.value.protocol === 'https' ? secrets.tls_private_key : '',
    });
    created.value = true;
    Object.keys(secrets).forEach((key) => {
      secrets[key] = '';
    });
    emit('saved', result);
  });
}
</script>

<style scoped>
.guided-setup {
  --setup-muted: var(--el-text-color-regular);
  color: var(--el-text-color-primary);
  font-size: 14px;
}
.setup-steps {
  display: flex;
  list-style: none;
  padding: 0 0 24px;
  margin: 0;
  border-bottom: 1px solid var(--el-border-color-light);
  gap: 24px;
}
.setup-steps li {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--setup-muted);
  flex: 1;
}
.setup-steps li > span {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--el-border-color);
  border-radius: 50%;
  flex-shrink: 0;
}
.setup-steps .current {
  color: var(--el-color-primary);
  font-weight: 500;
}
.setup-steps .current > span,
.setup-steps .complete > span {
  background: var(--el-color-primary);
  color: var(--el-color-white);
  border-color: var(--el-color-primary);
}
.setup-section {
  padding: 32px 0 8px;
}
.setup-section h2 {
  font-size: 26px;
  line-height: 1.4;
  font-weight: 500;
  letter-spacing: -0.6px;
  margin: 0 0 8px;
}
.setup-lead {
  margin: 0 0 24px;
  color: var(--setup-muted);
  line-height: 1.7;
}
.scene-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.scene-option {
  position: relative;
  display: grid;
  grid-template-columns: 36px 1fr;
  gap: 8px 16px;
  padding: 24px;
  border: 1px solid var(--el-border-color);
  border-radius: var(--hw-r-lg);
  cursor: pointer;
  transition:
    background 150ms,
    border-color 150ms,
    transform 150ms;
}
.scene-option input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
}
.scene-option strong {
  font-size: 16px;
  font-weight: 500;
  padding-right: 20px;
}
.scene-option > span:not(.scene-check) {
  grid-column: 2;
  color: var(--setup-muted);
  line-height: 1.7;
}
.scene-icon {
  width: 28px;
  height: 28px;
  grid-row: 1/3;
  color: var(--el-color-primary);
}
.scene-check {
  position: absolute;
  right: 16px;
  top: 24px;
  width: 20px;
  height: 20px;
  border: 1px solid var(--el-border-color);
  border-radius: 50%;
  text-align: center;
  font-size: 13px;
}
.scene-option.selected {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}
.selected .scene-check {
  color: white;
  background: var(--el-color-primary);
  border-color: var(--el-color-primary);
}
.scene-option:focus-within {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
.setup-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.setup-field > span:first-child {
  font-weight: 500;
}
.setup-field em {
  font-style: normal;
  font-size: 13px;
  color: var(--setup-muted);
  margin-left: 8px;
  font-weight: 400;
}
.setup-field small,
.setup-note {
  color: var(--setup-muted);
  line-height: 1.75;
}
.setup-field :deep(.el-input__wrapper) {
  min-height: 44px;
}
.setup-field :deep(.el-select),
.setup-field :deep(.el-input-number) {
  width: 100%;
}
.setup-field :deep(.el-select__wrapper) {
  min-height: 44px;
}
.setup-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px 24px;
}
.connection-preview {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 16px 0;
  margin: 8px 0 16px;
  font-size: 13px;
  color: var(--setup-muted);
}
.connection-preview > svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}
.connection-preview > span:last-child {
  overflow-wrap: anywhere;
}
.connection-preview b {
  font-family: var(--font-mono);
  font-weight: 500;
  color: var(--el-color-primary);
}
.setup-account {
  padding-top: 24px;
  margin: 8px 0 24px;
  border-top: 1px solid var(--el-border-color-light);
}
.setup-account h3 {
  font-size: 16px;
  font-weight: 500;
  margin: 0 0 8px;
}
.setup-account p {
  line-height: 1.7;
  color: var(--setup-muted);
  margin: 0 0 16px;
}
.setup-details {
  border-top: 1px solid var(--el-border-color-light);
  margin-top: 24px;
  padding: 8px 0;
}
.setup-details summary {
  cursor: pointer;
  padding: 12px 0;
  min-height: 44px;
  font-weight: 500;
}
.setup-details summary > span {
  color: var(--setup-muted);
  font-weight: 400;
  margin-left: 12px;
  font-size: 13px;
}
.setup-details[open] summary {
  margin-bottom: 16px;
}
.certificate-fields {
  margin-top: 24px;
}
.setup-note {
  margin: 16px 0 0;
  font-size: 13px;
}
.setup-actions {
  position: sticky;
  bottom: 0;
  z-index: 1;
  background: var(--el-bg-color);
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  padding: 16px 0;
  margin-top: 24px;
  border-top: 1px solid var(--el-border-color-light);
}
.setup-actions > span {
  margin-right: auto;
  color: var(--setup-muted);
  font-size: 13px;
}
.setup-actions :deep(.el-button) {
  min-height: 44px;
  margin-left: 0;
}
.button-icon {
  width: 16px;
  height: 16px;
  margin-left: 8px;
}
.setup-error,
.field-error {
  color: var(--el-color-danger);
  line-height: 1.6;
}
.setup-error {
  padding: 16px;
  background: var(--el-color-danger-light-9);
  border-radius: 8px;
  margin-top: 16px;
}
.setup-review {
  margin: 0;
}
.setup-review > div {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 24px;
  padding: 16px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.setup-review dt {
  color: var(--setup-muted);
}
.setup-review dd {
  margin: 0;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.setup-check-result {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  margin-top: 24px;
}
.setup-check-result svg {
  color: var(--el-color-success);
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}
.setup-check-result strong {
  font-weight: 500;
}
.setup-check-result p {
  margin: 8px 0 0;
  color: var(--setup-muted);
  line-height: 1.7;
  font-size: 13px;
}
@media (hover: hover) {
  .scene-option:hover {
    transform: translateY(-2px);
    border-color: var(--el-color-primary);
  }
}
@media (max-width: 600px) {
  .setup-steps {
    gap: 8px;
    padding-bottom: 16px;
  }
  .setup-steps li {
    gap: 6px;
    font-size: 13px;
  }
  .setup-steps li > span {
    width: 24px;
    height: 24px;
  }
  .setup-section {
    padding-top: 24px;
  }
  .setup-section h2 {
    font-size: 22px;
  }
  .scene-options,
  .setup-columns {
    grid-template-columns: 1fr;
  }
  .scene-option {
    padding: 16px;
  }
  .scene-check {
    top: 16px;
  }
  .setup-actions {
    flex-wrap: wrap;
    gap: 8px;
  }
  .setup-actions > span {
    width: 100%;
  }
  .setup-actions :deep(.el-button) {
    flex: 1;
  }
  .setup-details summary > span {
    display: block;
    margin: 8px 0 0;
  }
  .setup-review > div {
    grid-template-columns: 80px 1fr;
    gap: 12px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .scene-option {
    transition: none;
  }
  .scene-option:hover {
    transform: none;
  }
}
</style>
