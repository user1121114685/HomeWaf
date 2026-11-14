<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-alert theme="info" :message="$t('page.notify_channel.webhook_config_tip')" style="margin-bottom: 16px;"></hw-alert>

    <hw-form-item :label="$t('page.notify_channel.webhook_preset')" name="webhook_preset">
      <div :style="{ width: 'min(100%, 480px)' }">
        <hw-select :value="preset" :style="{ width: 'min(100%, 480px)' }" @change="applyPreset">
          <hw-option v-for="item in presetOptions" :key="item.value" :value="item.value" :label="item.label"></hw-option>
        </hw-select>
        <div class="webhook-tip">💡 {{ $t('page.notify_channel.webhook_preset_tip') }}</div>
      </div>
    </hw-form-item>

    <hw-form-item :label="$t('page.notify_channel.label_webhook_url')" name="webhook_url">
      <div :style="{ width: 'min(100%, 480px)' }">
        <hw-input v-model="form.webhook_url" :placeholder="urlPlaceholder"></hw-input>
        <div v-if="presetHint" class="webhook-tip">📝 {{ presetHint }}</div>
      </div>
    </hw-form-item>

    <hw-form-item :label="$t('page.notify_channel.webhook_method')" name="webhook_method">
      <hw-select v-model="form.webhook_method" :style="{ width: 'min(100%, 480px)' }">
        <hw-option v-for="m in methods" :key="m" :value="m" :label="m"></hw-option>
      </hw-select>
    </hw-form-item>

    <hw-form-item :label="$t('page.notify_channel.webhook_content_type')" name="webhook_content_type">
      <hw-select v-model="form.webhook_content_type" :style="{ width: 'min(100%, 480px)' }" filterable creatable>
        <hw-option v-for="c in contentTypes" :key="c" :value="c" :label="c"></hw-option>
      </hw-select>
    </hw-form-item>

    <hw-form-item :label="$t('page.notify_channel.webhook_headers')" name="webhook_headers">
      <div :style="{ width: 'min(100%, 480px)' }">
        <div v-for="(header, index) in form.webhook_headers" :key="index" class="webhook-header-row">
          <hw-input v-model="header.key" :placeholder="$t('page.notify_channel.webhook_header_key')" style="width: 180px;"></hw-input>
          <hw-input v-model="header.value" :placeholder="$t('page.notify_channel.webhook_header_value')" style="width: min(100%, 240px);"></hw-input>
          <hw-button variant="text" theme="danger" @click="removeHeader(index)">{{ $t('common.delete') }}</hw-button>
        </div>
        <hw-button variant="dashed" size="small" :disabled="form.webhook_headers.length >= maxHeaders" @click="addHeader">
          + {{ $t('page.notify_channel.webhook_header_add') }}
        </hw-button>
        <div class="webhook-tip">💡 {{ $t('page.notify_channel.webhook_header_tip') }}</div>
      </div>
    </hw-form-item>

    <hw-form-item :label="$t('page.notify_channel.webhook_body')" name="webhook_body_template">
      <div :style="{ width: 'min(100%, 480px)' }">
        <!-- 两层模板最容易被混淆，直接在变量上方讲清楚分工，否则用户会把订阅模板的
             Domain / Ip 之类变量写到这里，保存时才被后端拦下来 -->
        <hw-alert theme="info" :message="$t('page.notify_channel.webhook_body_layer_tip')"
          style="margin-bottom: 8px;"></hw-alert>
        <div class="webhook-vars">
          <hw-tag v-for="v in vars" :key="v.name" theme="primary" variant="light" class="webhook-var-tag"
            @click="insertVar(v.name)">
            {{ v.label }}
          </hw-tag>
        </div>
        <hw-textarea ref="bodyInput" v-model="form.webhook_body_template" :rows="8"
          :placeholder="$t('page.notify_channel.webhook_body_placeholder')"></hw-textarea>
        <div class="webhook-tip">💡 {{ $t('page.notify_channel.webhook_body_tip') }}</div>
      </div>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { WEBHOOK_PRESETS, WEBHOOK_MAX_HEADERS } from '../webhook_presets';

export default defineComponent({
  name: 'WebhookConfig',
  props: {
    // 直接接收父级表单对象并就地修改属性（不重新赋值 prop 本身），
    // 这样新增/编辑两个弹窗可以共用同一份配置界面，不必把十几个字段来回 emit。
    form: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      methods: ['POST', 'PUT', 'PATCH', 'GET', 'DELETE'],
      contentTypes: ['application/json', 'application/x-www-form-urlencoded', 'text/plain', 'text/xml'],
      maxHeaders: WEBHOOK_MAX_HEADERS,
      // 与后端 wafnotify/webhook.Message 的字段一一对应
      // label 在这里拼好：模板里写 '{{' 会被 Vue 当成插值起始，直接编译不过
      vars: [
        'Title',
        'Content',
        'Time',
        'MessageType',
        'MessageTypeName',
        'Severity',
        'ServerName',
      ].map((name) => ({ name, label: `{{.${name}}}` })),
    };
  },
  computed: {
    preset(): string {
      return this.form.webhook_preset || 'custom';
    },
    presetOptions(): any[] {
      return WEBHOOK_PRESETS.map((p: any) => ({ value: p.value, label: this.$t(p.labelKey) }));
    },
    currentPreset(): any {
      return WEBHOOK_PRESETS.find((p: any) => p.value === this.preset) || WEBHOOK_PRESETS[0];
    },
    urlPlaceholder(): string {
      return this.currentPreset.urlPlaceholder || this.$t('page.notify_channel.webhook_placeholder');
    },
    presetHint(): string {
      return this.currentPreset.hintKey ? this.$t(this.currentPreset.hintKey) : '';
    },
  },
  methods: {
    applyPreset(value: string) {
      this.$set(this.form, 'webhook_preset', value);
      const preset = WEBHOOK_PRESETS.find((p: any) => p.value === value);
      if (!preset || value === 'custom') {
        return;
      }
      // 只覆盖报文相关字段，URL 由用户自己填（预设里放的是占位示例，不是真实地址）
      this.$set(this.form, 'webhook_method', preset.method);
      this.$set(this.form, 'webhook_content_type', preset.contentType);
      this.$set(this.form, 'webhook_headers', (preset.headers || []).map((h: any) => ({ ...h })));
      this.$set(this.form, 'webhook_body_template', preset.bodyTemplate);
    },
    addHeader() {
      if (this.form.webhook_headers.length >= this.maxHeaders) {
        return;
      }
      this.form.webhook_headers.push({ key: '', value: '' });
    },
    removeHeader(index: number) {
      this.form.webhook_headers.splice(index, 1);
    },
    insertVar(name: string) {
      this.$set(this.form, 'webhook_body_template', (this.form.webhook_body_template || '') + `{{.${name}}}`);
    },
  },
});
</script>

<style lang="less" scoped>
.webhook-tip {
  font-size: 13px;
  color: var(--hw-muted);
  margin-top: 4px;
  line-height: 1.5;
}

.webhook-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.webhook-vars {
  margin-bottom: 8px;
}

.webhook-var-tag {
  margin-right: 6px;
  margin-bottom: 6px;
  cursor: pointer;
  font-family: var(--font-mono);
}
</style>
