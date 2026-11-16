<template>
  <section class="honey-preview" data-testid="honeypot-preview" aria-label="蜜罐反馈预览">
    <div class="preview-heading">
      <h3>对方会看到什么？</h3>
      <span>草稿预览</span>
    </div>
    <label v-if="lures.length > 1" class="preview-choice">
      预览页面
      <ElSelect v-model="selected" aria-label="预览页面">
        <ElOption
          v-for="(item, index) in lures"
          :key="item.id"
          :value="index"
          :label="`${index + 1} · ${item.title || '未填写标题'}`"
        />
      </ElSelect>
    </label>
    <fieldset class="preview-states">
      <legend>模拟访问结果</legend>
      <label v-for="item in states" :key="item.value" :class="{ selected: state === item.value }">
        <input v-model="state" type="radio" :name="`${id}-response`" :value="item.value" />{{ item.label }}
      </label>
    </fieldset>
    <div class="response-meta" role="status">
      <strong>{{ state === 'busy' ? 'HTTP 503 · 服务繁忙' : 'HTTP 200 · 仿真页面' }}</strong>
      <p>
        {{
          state === 'busy'
            ? '并发名额用满时返回这段文字，并提示 1 秒后重试。'
            : state === 'login'
              ? '蜜罐收到登录提交后，始终显示这条失败提示；不提供真实登录。'
              : '请求进入蜜罐后显示此页。账号和密码输入框由系统提供，无需配置账号。'
        }}
      </p>
    </div>
    <p class="preview-note">下方为只读外观。使用上方选项或“模拟提交登录”查看反馈。</p>
    <iframe
      :srcdoc="html"
      title="蜜罐响应外观"
      sandbox=""
      tabindex="-1"
      class="preview-frame"
      :class="{ compact: state === 'busy', 'login-feedback': state === 'login' }"
    />
    <ElButton v-if="state === 'visit'" @click="state = 'login'">模拟提交登录</ElButton>
    <p class="preview-note">只在本地预览，不发送请求、不保存设置。实际是否触发，请在服务启用后查看访问记录。</p>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import type { PropType } from 'vue';
import { ElButton, ElOption, ElSelect } from 'element-plus';
import {
  defaultHoneypotLure,
  honeypotPreviewHTML,
  type HoneypotLure,
  type HoneypotPreviewState,
} from './honeypotPresentation';
const props = defineProps({
  lures: { type: Array as PropType<HoneypotLure[]>, required: true },
  modelValue: { type: Number, required: true },
});
const emit = defineEmits(['update:modelValue']);
const selected = computed({ get: () => props.modelValue, set: (value) => emit('update:modelValue', value) });
const state = ref<HoneypotPreviewState>('visit'),
  id = useId();
const states = [
  { value: 'visit', label: '首次访问' },
  { value: 'login', label: '提交登录后' },
  { value: 'busy', label: '并发已满' },
];
watch(
  () => props.lures.length,
  (length) => {
    if (selected.value >= length) selected.value = 0;
  },
);
const html = computed(() => honeypotPreviewHTML(props.lures[selected.value] || defaultHoneypotLure, state.value));
</script>
<style scoped>
.honey-preview {
  min-width: 0;
  padding: 24px;
  background: var(--hw-panel-soft, var(--el-fill-color-light));
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  align-self: start;
}
.preview-heading {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}
.preview-heading h3 {
  margin: 0;
  font-size: 16px;
}
.preview-heading span,
.preview-note,
.response-meta p {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
}
.preview-choice {
  display: grid;
  gap: 8px;
  margin: 16px 0;
  font-size: 14px;
}
.preview-states {
  border: 0;
  padding: 0;
  margin: 20px 0 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.preview-states legend {
  margin-bottom: 8px;
  font-size: 13px;
}
.preview-states label {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
  padding: 0 10px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  cursor: pointer;
  font-size: 13px;
  background: var(--hw-panel);
}
.preview-states label.selected {
  border-color: var(--el-color-primary);
}
.preview-states label:focus-within {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.response-meta strong {
  font-size: 13px;
}
.response-meta p {
  margin: 8px 0 16px;
}
.preview-frame {
  display: block;
  width: 100%;
  height: 490px;
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  background: var(--hw-panel);
}
.preview-frame.compact {
  height: 160px;
}
.preview-frame.login-feedback {
  height: 600px;
}
.honey-preview :deep(.el-button) {
  margin-top: 16px;
  min-height: 44px;
}
.preview-note {
  margin: 12px 0 0;
}
@media (max-width: 600px) {
  .preview-frame:not(.compact) {
    height: 600px;
  }
  .preview-frame.login-feedback {
    height: 680px;
  }
  .honey-preview {
    padding: 16px;
  }
}
</style>
