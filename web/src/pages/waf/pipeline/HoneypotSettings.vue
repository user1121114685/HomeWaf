<template>
  <section class="honeypot-settings" data-testid="honeypot-settings">
    <h2>{{ shared ? '1. 准备蜜罐页面' : '蜜罐设置' }}</h2>
    <p class="help">选一个模板，再按提示修改文字。第一次使用也可以直接保留内置页面，下面的可选设置不用填写。</p>
    <Fields v-if="!shared" :model="model" :definitions="group('enabled', 'capture_denied')" />
    <div class="page-workspace">
      <div class="page-editor">
        <h3>从模板开始</h3>
        <p class="help">四种模板共用仿真登录表单，只改变标题和说明。无需真实账号，也无需另建网站。</p>
        <label v-if="lures.length > 1" class="form-field"
          >将模板填入哪一页？
          <ElSelect v-model="templateTarget" aria-label="模板填写目标">
            <ElOption
              v-for="(lure, index) in lures"
              :key="lure.id"
              :value="index"
              :label="'页面 ' + (index + 1) + ' · ' + (lure.title || '未填写标题')"
            />
          </ElSelect>
        </label>
        <div class="template-options">
          <button
            v-for="template in honeypotTemplates"
            :key="template.id"
            type="button"
            :disabled="applyingTemplate"
            :aria-label="'使用' + template.name + '模板'"
            @click="applyTemplate(template)"
          >
            <strong>{{ template.name }}</strong
            ><span>{{ template.description }}</span>
          </button>
        </div>
        <p v-if="!lures.length" class="default-note">
          当前使用内置“管理控制台”页面，可直接保存；选择模板后可修改文案。
        </p>
        <p v-if="templateNotice" class="template-notice" role="status">{{ templateNotice }}</p>
        <ElButton class="preview-shortcut" @click="showPreview">查看当前反馈预览</ElButton>
        <fieldset v-for="(lure, index) in lures" :key="lure.id || index" class="lure-row">
          <legend>页面 {{ index + 1 }}</legend>
          <label class="form-field" :for="id + '-title-' + index"
            >页面标题
            <ElInput
              :id="id + '-title-' + index"
              v-model="lure.title"
              :aria-label="'蜜罐提示' + (index + 1) + '标题'"
              :aria-describedby="id + '-title-help-' + index"
              :aria-invalid="!!titleError(lure)"
              @focus="previewIndex = index"
            />
            <small :id="id + '-title-help-' + index">显示在页面顶部，例如“设备管理”。不用填写真实设备名称。</small>
          </label>
          <p v-if="titleError(lure)" class="field-error" role="alert">{{ titleError(lure) }}</p>
          <label class="form-field" :for="id + '-message-' + index"
            >登录前的说明
            <ElInput
              :id="id + '-message-' + index"
              v-model="lure.message"
              type="textarea"
              :rows="3"
              :aria-label="'蜜罐提示' + (index + 1) + '内容'"
              :aria-describedby="id + '-message-help-' + index"
              @focus="previewIndex = index"
            />
            <small :id="id + '-message-help-' + index"
              >例如“请使用管理员账号登录”。只显示文字，不执行 HTML 或脚本；登录失败提示由系统提供。</small
            >
          </label>
          <details class="weight-details" :open="lures.length > 1">
            <summary>{{ lures.length > 1 ? '分配给多少来源' : '页面分配比例 · 只有一页时无需调整' }}</summary>
            <label class="form-field"
              >分配份数（相对权重）
              <div class="number-explanation">
                <ElInputNumber
                  v-model="lure.weight"
                  :min="1"
                  :max="10000"
                  :precision="0"
                  :aria-label="'蜜罐提示' + (index + 1) + '权重'"
                />
                <span>约 {{ share(lure) }}% 的来源</span>
              </div>
              <small
                >两页填 1 和 3，约分给 25% 和 75% 的来源。同一来源在同一服务上稳定看到同一页，不会每次刷新换页。</small
              >
            </label>
          </details>
          <ElButton plain type="danger" :aria-label="'移除蜜罐提示' + (index + 1)" @click="removeLure(index)"
            >移除此页</ElButton
          >
        </fieldset>
        <ElButton :disabled="lures.length >= 32" @click="addLure">添加蜜罐页面</ElButton>
        <p class="help">一页就能使用。需要给不同来源展示不同文字时再添加，最多 32 页；移除全部后恢复内置页面。</p>
        <p v-for="error in otherLureErrors" :key="error" class="field-error" role="alert">{{ error }}</p>
      </div>
      <HoneypotPreview :id="id + '-preview'" v-model="previewIndex" :lures="lures" />
    </div>
    <section class="settings-section">
      <h3>额外保存提交内容 <span class="optional">可选</span></h3>
      <p class="help">来源 IP、时间和访问路径会记入蜜罐记录。只有想分析对方提交了什么时，才需要开启下面的额外记录。</p>
      <label class="switch-field">
        <span
          >保存脱敏后的提交内容<small
            >口令、令牌等会脱敏；不执行、不解压提交内容。不适合保存的正文只留摘要。</small
          ></span
        >
        <ElSwitch v-model="model.archive_body" aria-label="保存脱敏后的提交内容" />
      </label>
      <div v-if="model.archive_body || archiveError" class="archive-limit">
        <label class="form-field"
          >每次最多采样多少字节？
          <ElInputNumber
            v-model="model.archive_max_bytes"
            :min="0"
            :max="65536"
            :precision="0"
            aria-label="归档正文采样上限（字节）"
          />
          <small
            >默认 16384 字节（16 KiB），通常无需修改。可填 1024–65536；0
            使用默认值。超限正文不保存可读内容，只留摘要。</small
          >
        </label>
      </div>
      <p v-if="archiveError" class="field-error" role="alert">{{ archiveError }}</p>
    </section>
    <section class="settings-section">
      <h3>设置诱饵网址 <span class="optional">可选 · 陷阱路径</span></h3>
      <p class="help">如果有人访问这些路径，就由蜜罐响应。留空也能使用蜜罐：被安全策略拒绝的请求仍可触发。</p>
      <label class="form-field" :for="id + '-traps'"
        >确认不存在的路径，每行一条
        <ElInput
          :id="id + '-traps'"
          v-model="trapDraft"
          type="textarea"
          :rows="3"
          aria-label="确认不存在的陷阱路径"
          :aria-describedby="id + '-trap-help'"
          :aria-invalid="!!trapError"
          placeholder="例如 /__decoy_test，填写前先确认业务没有使用它"
        />
        <small :id="id + '-trap-help'"
          >只填 / 开头的具体路径，不填域名或查询参数。{{
            shared ? '所有使用统一蜜罐的服务都必须没有这些业务路径。' : '当前服务必须没有这些业务路径。'
          }}真实登录页不能填在这里。</small
        >
      </label>
      <p v-if="trapError" class="field-error" role="alert">{{ trapError }}</p>
    </section>
    <details class="settings-section runtime-settings">
      <summary>
        运行限制 <span>{{ model.max_concurrent }} 个并发 · 随机等待 0–{{ model.max_delay_ms }} 毫秒</span>
      </summary>
      <p class="help">用于控制蜜罐占用的资源，通常保持当前值即可，不影响页面文案。</p>
      <Fields :model="model" :definitions="resourceFields" />
    </details>
    <section v-if="!shared" class="settings-section">
      <h3>当前服务的旧版处罚设置</h3>
      <ElAlert
        v-if="!hitPolicyEnabled"
        title="自动处罚当前关闭，以下命中阈值不会生效。"
        type="warning"
        :closable="false"
        ><ElButton text @click="emit('open-punishment')">配置自动处罚</ElButton></ElAlert
      >
      <Fields
        :model="model"
        :definitions="[...group('ttl_seconds', 'auth_fail_threshold', 'weak_password_threshold'), ...dropFields]"
      />
    </section>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import type { PropType } from 'vue';
import { ElAlert, ElButton, ElInput, ElInputNumber, ElMessageBox, ElOption, ElSelect, ElSwitch } from 'element-plus';
import Fields from './Fields.vue';
import HoneypotPreview from './HoneypotPreview.vue';
import { fields, parseLines } from './model';
import { honeypotTemplates, type HoneypotLure } from './honeypotPresentation';
const props = defineProps({
  model: { type: Object as PropType<Record<string, any>>, required: true },
  hitPolicyEnabled: { type: Boolean, default: false },
  shared: Boolean,
});
const emit = defineEmits(['invalid', 'open-punishment']);
const id = useId(),
  previewIndex = ref(0),
  templateTarget = ref(0),
  applyingTemplate = ref(false),
  templateNotice = ref('');
const group = (...paths: string[]) => fields.honeypot.filter((field) => paths.includes(field.path));
const dropFields = fields.honeypot.filter(
  (field) => field.path.startsWith('drop_') || field.path.startsWith('distributed_drop.'),
);
const resourceFields = group('max_concurrent', 'max_delay_ms').map((field) => ({
  ...field,
  label: field.path === 'max_concurrent' ? '同时处理的蜜罐请求数' : '每次响应最多等待（毫秒）',
  help:
    field.path === 'max_concurrent'
      ? '默认 32。单个服务有效上限为 32，整个实例最多 64 个；用满时返回预览中的“服务繁忙”。'
      : '默认 250 毫秒（四分之一秒）。每次在 0 到此值之间随机等待；0 表示立即响应，最多 2000 毫秒。',
}));
const lures = computed<HoneypotLure[]>(() => (Array.isArray(props.model.lures) ? props.model.lures : []));
const trapDraft = ref('');
watch(
  () => props.model,
  () => {
    trapDraft.value = (props.model.trap_paths || []).join('\n');
    templateNotice.value = '';
  },
  { immediate: true },
);
watch(trapDraft, (text) => {
  const paths = parseLines(text);
  if (JSON.stringify(paths) !== JSON.stringify(props.model.trap_paths || [])) props.model.trap_paths = paths;
});
watch(
  () => props.model.trap_paths,
  (paths) => {
    if (JSON.stringify(parseLines(trapDraft.value)) !== JSON.stringify(paths || []))
      trapDraft.value = (paths || []).join('\n');
  },
);
watch(
  () => lures.value.length,
  (length) => {
    if (templateTarget.value >= length) templateTarget.value = 0;
  },
);
const byteLength = (value: string) => new TextEncoder().encode(value).length;
function titleError(lure: HoneypotLure) {
  return !String(lure.title || '').trim()
    ? '请填写页面标题。'
    : byteLength(lure.title) > 128
      ? '标题过长，请缩短到 128 字节以内（中文通常约 42 字）。'
      : '';
}
const otherLureErrors = computed(() => {
  const errors: string[] = [],
    ids = new Set<string>();
  let total = 0;
  if (lures.value.length > 32) errors.push('最多配置32条蜜罐提示。');
  lures.value.forEach((lure, index) => {
    if (!lure.id || ids.has(lure.id)) errors.push('页面' + (index + 1) + '的标识无效，请移除后重新添加。');
    ids.add(lure.id);
    if (byteLength(String(lure.message || '')) > 4096)
      errors.push('页面' + (index + 1) + '的说明过长，请缩短到4096字节以内。');
    if (!Number.isInteger(lure.weight) || lure.weight < 1 || lure.weight > 10000)
      errors.push('页面' + (index + 1) + '的分配份数必须是1–10000的整数。');
    total += byteLength(String(lure.title || '')) + byteLength(String(lure.message || ''));
  });
  if (total > 32768) errors.push('所有页面的标题和说明合计最多32KiB。');
  return errors;
});
const archiveError = computed(() => {
  const value = Number(props.model.archive_max_bytes || 0);
  return value !== 0 && (!Number.isInteger(value) || value < 1024 || value > 65536)
    ? '采样上限请填 1024–65536 字节，或填 0 使用默认值。'
    : '';
});
const trapError = computed(() => {
  const paths = parseLines(trapDraft.value);
  if (paths.length > 128) return '最多填写 128 条陷阱路径。';
  return paths.some((path) => !path.startsWith('/') || /[?#]/.test(path) || byteLength(path) > 1024)
    ? '请填写 / 开头的路径，不要包含域名或查询参数，每条最多 1024 字节。'
    : '';
});
watch(
  () =>
    lures.value.some((lure) => !!titleError(lure)) ||
    otherLureErrors.value.length > 0 ||
    !!archiveError.value ||
    !!trapError.value,
  (invalid) => emit('invalid', invalid),
  { immediate: true },
);
function share(lure: HoneypotLure) {
  const total = lures.value.reduce((sum, item) => sum + (Number(item.weight) || 0), 0);
  return total ? Math.round(((Number(lure.weight) || 0) / total) * 1000) / 10 : 0;
}
function addLure() {
  if (lures.value.length >= 32) return;
  props.model.lures = [
    ...lures.value,
    { id: crypto.randomUUID(), title: '管理控制台', message: '请输入管理账号。', weight: 1 },
  ];
  previewIndex.value = lures.value.length - 1;
  templateTarget.value = previewIndex.value;
}
async function applyTemplate(template: (typeof honeypotTemplates)[number]) {
  if (applyingTemplate.value) return;
  applyingTemplate.value = true;
  try {
    const target = lures.value[templateTarget.value];
    if (target && (target.title !== template.title || target.message !== template.message)) {
      await ElMessageBox.confirm(
        '用“' +
          template.name +
          '”模板替换页面 ' +
          (templateTarget.value + 1) +
          ' 的标题和说明？其他页面、分配份数和设置保持不变，保存后才会生效。',
        '替换本页文案',
        { type: 'warning', confirmButtonText: '替换文案', cancelButtonText: '保留当前文案' },
      );
    }
    if (target) {
      target.title = template.title;
      target.message = template.message;
    } else
      props.model.lures = [{ id: crypto.randomUUID(), title: template.title, message: template.message, weight: 1 }];
    previewIndex.value = templateTarget.value;
    templateNotice.value = '已将“' + template.name + '”填入草稿，可继续修改标题和说明；尚未保存。';
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error;
  } finally {
    applyingTemplate.value = false;
  }
}
function removeLure(index: number) {
  props.model.lures = lures.value.filter((_, i) => i !== index);
}
function showPreview() {
  const target = document.getElementById(id + '-preview');
  target?.scrollIntoView({ block: 'start' });
  target?.querySelector<HTMLInputElement>('input:checked')?.focus({ preventScroll: true });
}
</script>
<style scoped>
.honeypot-settings {
  min-width: 0;
}
h2 {
  margin: 0;
  font-size: 18px;
}
h3 {
  margin: 0 0 12px;
  font-size: 16px;
}
.help,
small,
.default-note,
.template-notice {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.8;
}
.help {
  margin: 8px 0 20px;
}
.page-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
  gap: 32px;
  margin-top: 24px;
}
.page-editor {
  min-width: 0;
}
.preview-shortcut {
  display: none;
}
.honeypot-settings :deep(input),
.honeypot-settings :deep(textarea),
.honeypot-settings :deep(button),
.honeypot-settings :deep(summary) {
  scroll-margin-block: 100px 180px;
}
.honeypot-settings :deep(.honey-preview) {
  scroll-margin-top: 96px;
}
.template-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.template-options button {
  display: grid;
  text-align: left;
  gap: 6px;
  min-height: 76px;
  padding: 14px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  background: var(--hw-panel);
  color: var(--hw-ink);
  cursor: pointer;
  font: inherit;
}
.template-options button:hover {
  border-color: var(--el-color-primary);
  background: var(--el-fill-color-light);
}
.template-options button span {
  font-size: 12px;
  color: var(--hw-muted);
}
.template-options button:focus-visible,
summary:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 3px;
}
.lure-row {
  min-width: 0;
  margin: 24px 0 16px;
  padding: 16px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
}
.lure-row legend {
  padding: 0 8px;
  font-size: 14px;
  font-weight: 600;
}
.form-field {
  display: grid;
  gap: 8px;
  min-width: 0;
  margin-bottom: 16px;
  font-size: 14px;
  line-height: 1.6;
}
.form-field :deep(.el-select),
.form-field :deep(.el-input-number) {
  width: 100%;
}
.weight-details {
  margin: 8px 0 16px;
}
summary {
  cursor: pointer;
  min-height: 44px;
  align-content: center;
  font-size: 14px;
  line-height: 1.7;
}
.weight-details .form-field {
  margin-top: 12px;
}
.number-explanation {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
.number-explanation :deep(.el-input-number) {
  width: 160px;
}
.number-explanation span {
  color: var(--hw-muted);
  font-size: 13px;
}
.settings-section {
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--hw-line);
}
.optional {
  font-size: 13px;
  font-weight: 400;
  color: var(--hw-muted);
  margin-left: 8px;
}
.switch-field {
  display: flex;
  gap: 24px;
  align-items: center;
  justify-content: space-between;
  font-size: 14px;
}
.switch-field small {
  display: block;
  margin-top: 6px;
}
.archive-limit {
  margin-top: 20px;
  max-width: 640px;
}
.runtime-settings summary {
  font-weight: 600;
}
.runtime-settings summary span {
  font-weight: 400;
  color: var(--hw-muted);
  margin-left: 12px;
}
.honeypot-settings :deep(.el-button) {
  min-height: 44px;
}
.field-error {
  font-size: 13px;
  line-height: 1.7;
  color: var(--el-color-danger);
}
@media (max-width: 1100px) {
  .preview-shortcut {
    display: inline-flex;
    margin-bottom: 8px;
  }
  .page-workspace {
    grid-template-columns: minmax(0, 1fr);
    gap: 24px;
  }
}
@media (max-width: 600px) {
  .lure-row {
    padding: 12px;
  }
  .template-options {
    gap: 8px;
  }
  .template-options button {
    padding: 12px;
  }
  .runtime-settings summary span {
    display: block;
    margin-left: 0;
  }
}
</style>
