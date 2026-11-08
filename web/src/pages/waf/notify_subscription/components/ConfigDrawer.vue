<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-drawer
    :header="$t('page.notify_subscription.config_title') + ' - ' + headerSuffix"
    v-model:visible="innerVisible"
    :size="'680px'"
    :footer="false"
    @close="handleClose"
  >
    <hw-tabs v-model="activeTab">
      <!-- ===== 频率控制 ===== -->
      <hw-tab-panel value="throttle" :label="$t('page.notify_subscription.config_tab_throttle')">
        <div class="tab-body">
          <hw-form :labelWidth="120">
            <hw-form-item :label="$t('page.notify_subscription.label_throttle_mode')">
              <hw-radio-group v-model="form.throttle_mode">
                <hw-radio value="inherit">{{ $t('page.notify_subscription.throttle_mode_inherit') }}</hw-radio>
                <hw-radio value="realtime">{{ $t('page.notify_subscription.throttle_mode_realtime') }}</hw-radio>
                <hw-radio value="aggregate">{{ $t('page.notify_subscription.throttle_mode_aggregate') }}</hw-radio>
                <hw-radio value="cooldown">{{ $t('page.notify_subscription.throttle_mode_cooldown') }}</hw-radio>
              </hw-radio-group>
              <div class="form-tip">{{ modeTip }}</div>
            </hw-form-item>

            <!-- 聚合模式参数 -->
            <template v-if="form.throttle_mode === 'aggregate'">
              <hw-form-item :label="$t('page.notify_subscription.label_aggregate_window')">
                <hw-input-number v-model="form.throttle.aggregate_window_sec" :min="0" :max="3600" theme="normal" :style="{ width: '160px' }" />
                <span class="form-tip-inline">0 = {{ $t('page.notify_subscription.summary_inherit') }}</span>
              </hw-form-item>
              <hw-form-item :label="$t('page.notify_subscription.label_aggregate_max_detail')">
                <hw-input-number v-model="form.throttle.aggregate_max_detail" :min="0" :max="50" theme="normal" :style="{ width: '160px' }" />
              </hw-form-item>
            </template>

            <!-- 冷却模式参数 -->
            <template v-if="form.throttle_mode === 'cooldown'">
              <hw-form-item :label="$t('page.notify_subscription.label_cooldown_steps')">
                <hw-input v-model="cooldownStepsText" placeholder="60,300,900" :style="{ width: '260px' }" />
                <div class="form-tip">{{ $t('page.notify_subscription.cooldown_steps_tip') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.notify_subscription.label_cooldown_reset')">
                <hw-input-number v-model="form.throttle.cooldown_reset_sec" :min="0" :max="86400" theme="normal" :style="{ width: '160px' }" />
                <div class="form-tip">{{ $t('page.notify_subscription.cooldown_reset_tip') }}</div>
              </hw-form-item>
            </template>

            <hw-form-item :label="$t('page.notify_subscription.label_max_per_hour')" v-if="form.throttle_mode !== 'inherit'">
              <hw-input-number v-model="form.throttle.max_per_hour" :min="0" :max="10000" theme="normal" :style="{ width: '160px' }" />
              <div class="form-tip">{{ $t('page.notify_subscription.max_per_hour_tip') }}</div>
            </hw-form-item>

            <hw-form-item :label="$t('page.notify_subscription.label_dedup_keys')" v-if="form.throttle_mode === 'aggregate' || form.throttle_mode === 'cooldown'">
              <hw-checkbox-group v-model="form.throttle.dedup_keys">
                <hw-checkbox value="message_type">{{ $t('page.notify_subscription.dedup_key_message_type') }}</hw-checkbox>
                <hw-checkbox value="domain">{{ $t('page.notify_subscription.dedup_key_domain') }}</hw-checkbox>
                <hw-checkbox value="ip">{{ $t('page.notify_subscription.dedup_key_ip') }}</hw-checkbox>
                <hw-checkbox value="attack_type">{{ $t('page.notify_subscription.dedup_key_attack_type') }}</hw-checkbox>
                <hw-checkbox value="rule">{{ $t('page.notify_subscription.dedup_key_rule') }}</hw-checkbox>
              </hw-checkbox-group>
              <div class="form-tip">{{ $t('page.notify_subscription.dedup_keys_tip') }}</div>
            </hw-form-item>

            <hw-form-item :label="$t('page.notify_subscription.label_quiet_hours')">
              <hw-input v-model="form.throttle.quiet_hours" placeholder="23:00-07:00" :style="{ width: '200px' }" />
              <div class="form-tip">{{ $t('page.notify_subscription.quiet_hours_tip') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.label_quiet_bypass')" v-if="form.throttle.quiet_hours">
              <hw-select v-model="form.throttle.quiet_hours_bypass_severity" :style="{ width: '200px' }">
                <hw-option value="" :label="$t('page.notify_subscription.severity_none')"></hw-option>
                <hw-option value="warn" :label="$t('page.notify_subscription.severity_warn')"></hw-option>
                <hw-option value="critical" :label="$t('page.notify_subscription.severity_critical')"></hw-option>
              </hw-select>
              <div class="form-tip">{{ $t('page.notify_subscription.quiet_bypass_tip') }}</div>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-tab-panel>

      <!-- ===== 消息模板 ===== -->
      <hw-tab-panel value="template" :label="$t('page.notify_subscription.config_tab_template')">
        <div class="tab-body">
          <hw-form :labelWidth="120">
            <hw-form-item :label="$t('page.notify_subscription.label_title_template')">
              <hw-input v-model="form.title_template" :placeholder="defaultTitle" />
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.label_content_template')">
              <hw-textarea ref="contentTpl" v-model="form.content_template" :placeholder="defaultContent" :autosize="{ minRows: 6, maxRows: 14 }" />
              <div class="form-tip">{{ $t('page.notify_subscription.template_tip') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.template_vars')">
              <div class="var-list">
                <hw-tag
                  v-for="v in templateVars"
                  :key="v.name"
                  theme="primary"
                  variant="light"
                  class="var-tag"
                  @click="insertVar(v.name)"
                >{{ varLabel(v.name) }} <span class="var-desc">{{ v.desc }}</span></hw-tag>
              </div>
              <div class="form-tip">{{ $t('page.notify_subscription.template_insert_tip') }}</div>
            </hw-form-item>
            <hw-form-item>
              <hw-button variant="outline" size="small" @click="resetTemplate">{{ $t('page.notify_subscription.template_reset') }}</hw-button>
              <hw-button variant="outline" size="small" @click="doPreview" :style="{ marginLeft: '8px' }">{{ $t('page.notify_subscription.button_preview') }}</hw-button>
              <hw-button variant="outline" size="small" @click="applyToAllChannels" :style="{ marginLeft: '8px' }">{{ $t('page.notify_subscription.template_apply_all') }}</hw-button>
            </hw-form-item>
          </hw-form>

          <div class="preview-box" v-if="preview.title">
            <div class="preview-head">{{ $t('page.notify_subscription.preview_title') }}</div>
            <hw-alert v-if="preview.is_fallback" theme="error" :message="$t('page.notify_subscription.preview_fallback_warn')" />
            <div class="preview-title">{{ preview.title }}</div>
            <pre class="preview-content">{{ preview.content }}</pre>
          </div>
        </div>
      </hw-tab-panel>

      <!-- ===== 过滤条件 ===== -->
      <hw-tab-panel value="filter" :label="$t('page.notify_subscription.config_tab_filter')">
        <div class="tab-body">
          <hw-form :labelWidth="120">
            <hw-form-item :label="$t('page.notify_subscription.label_filter_domains')">
              <hw-textarea v-model="filterText.domains" :placeholder="domainsPlaceholder" :autosize="{ minRows: 3, maxRows: 6 }" />
              <div class="form-tip">{{ $t('page.notify_subscription.filter_domains_tip') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.label_filter_exclude_ips')">
              <hw-textarea v-model="filterText.exclude_ips" :placeholder="excludeIpsPlaceholder" :autosize="{ minRows: 3, maxRows: 6 }" />
              <div class="form-tip">{{ $t('page.notify_subscription.filter_exclude_ips_tip') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.label_filter_keywords')">
              <hw-textarea v-model="filterText.keywords" :autosize="{ minRows: 3, maxRows: 6 }" />
              <div class="form-tip">{{ $t('page.notify_subscription.filter_keywords_tip') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.label_filter_min_severity')">
              <hw-select v-model="form.filter.min_severity" :style="{ width: '200px' }">
                <hw-option value="" :label="$t('common.all')"></hw-option>
                <hw-option value="info" :label="$t('page.notify_subscription.severity_info')"></hw-option>
                <hw-option value="warn" :label="$t('page.notify_subscription.severity_warn')"></hw-option>
                <hw-option value="critical" :label="$t('page.notify_subscription.severity_critical')"></hw-option>
              </hw-select>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-tab-panel>

      <!-- ===== 测试与预览 ===== -->
      <hw-tab-panel value="debug" :label="$t('page.notify_subscription.config_tab_debug')">
        <div class="tab-body">
          <hw-form :labelWidth="120">
            <hw-form-item :label="$t('page.notify_subscription.button_dryrun')">
              <hw-button variant="outline" @click="doDryRun" :loading="dryRunLoading">{{ $t('page.notify_subscription.button_dryrun') }}</hw-button>
              <div class="form-tip">{{ $t('page.notify_subscription.dryrun_tip') }}</div>
            </hw-form-item>
            <hw-form-item v-if="dryRun.action">
              <hw-alert :theme="dryRun.would_send ? 'success' : 'warning'"
                :message="(dryRun.would_send ? $t('page.notify_subscription.dryrun_would_send') : $t('page.notify_subscription.dryrun_would_not_send')) + ' —— ' + dryRun.reason_text" />
              <div class="dryrun-stat">
                <span v-if="dryRun.cooldown_left > 0">{{ $t('page.notify_subscription.dryrun_cooldown_left', { sec: dryRun.cooldown_left }) }}</span>
                <span>{{ $t('page.notify_subscription.dryrun_hour_used', { count: dryRun.hour_used }) }}</span>
                <span v-if="dryRun.suppressed > 0">{{ $t('page.notify_subscription.dryrun_suppressed', { count: dryRun.suppressed }) }}</span>
              </div>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_subscription.button_test_send')">
              <hw-button theme="primary" variant="outline" @click="doTestSend" :loading="testLoading">{{ $t('page.notify_subscription.button_test_send') }}</hw-button>
              <div class="form-tip">{{ $t('page.notify_subscription.test_send_tip') }}</div>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-tab-panel>
    </hw-tabs>

    <div class="drawer-footer">
      <hw-button variant="outline" @click="handleClose">{{ $t('common.close') }}</hw-button>
      <hw-button theme="primary" @click="handleSave" :loading="saving">{{ $t('common.confirm') }}</hw-button>
    </div>
  </hw-drawer>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { MessagePlugin } from '@/ui';
import {
  saveNotifySubscriptionConfig,
  batchNotifySubscriptionConfig,
  previewNotifySubscription,
  testNotifySubscription,
  dryRunNotifySubscription,
  getNotifyTemplateVars,
} from '@/apis/notify_subscription';

// 空表单：所有字段留空 = 继承全局默认，与升级前行为一致
function emptyForm() {
  return {
    id: '',
    throttle_mode: 'inherit',
    throttle: {
      aggregate_window_sec: 0,
      aggregate_max_detail: 0,
      cooldown_steps_sec: [] as number[],
      cooldown_reset_sec: 0,
      max_per_hour: 0,
      dedup_keys: [] as string[],
      quiet_hours: '',
      quiet_hours_bypass_severity: '',
    },
    filter: {
      domains: [] as string[],
      exclude_ips: [] as string[],
      keywords: [] as string[],
      min_severity: '',
    },
    title_template: '',
    content_template: '',
  };
}

export default defineComponent({
  name: 'NotifySubscriptionConfigDrawer',
  props: {
    visible: { type: Boolean, default: false },
    subscription: { type: Object, default: () => ({}) },
    channelName: { type: String, default: '' },
    channelType: { type: String, default: '' },
    messageTypeName: { type: String, default: '' },
  },
  data() {
    return {
      innerVisible: false,
      activeTab: 'throttle',
      form: emptyForm(),
      // 多行文本 ↔ 数组：过滤条件用文本框填更顺手，提交前再切成数组
      filterText: { domains: '', exclude_ips: '', keywords: '' },
      // 换行只能从 JS 传：写在模板的 placeholder 属性里会被当成字面量 &#10; 显示出来
      domainsPlaceholder: 'www.example.com\n*.example.com',
      excludeIpsPlaceholder: '10.0.0.0/8\n192.168.1.1',
      cooldownStepsText: '',
      templateVars: [] as any[],
      defaultTitle: '',
      defaultContent: '',
      preview: { title: '', content: '', is_fallback: false },
      dryRun: {} as any,
      saving: false,
      testLoading: false,
      dryRunLoading: false,
    };
  },
  computed: {
    headerSuffix(): string {
      return `${this.messageTypeName} / ${this.channelName}`;
    },
    modeTip(): string {
      const key = `page.notify_subscription.throttle_mode_${this.form.throttle_mode}_tip`;
      return this.$t(key) as string;
    },
  },
  watch: {
    visible(val: boolean) {
      this.innerVisible = val;
      if (val) {
        this.loadFromSubscription();
        this.loadTemplateVars();
      }
    },
  },
  methods: {
    // 把订阅记录里的 JSON 字段摊开成表单
    loadFromSubscription() {
      const sub: any = this.subscription || {};
      const form = emptyForm();
      form.id = sub.id;
      form.throttle_mode = sub.throttle_mode || 'inherit';
      form.title_template = sub.title_template || '';
      form.content_template = sub.content_template || '';

      try {
        if (sub.throttle_json) {
          Object.assign(form.throttle, JSON.parse(sub.throttle_json) || {});
        }
      } catch (e) {
        // 脏数据不能把抽屉打不开，直接按"未配置"处理
        console.warn('throttle_json 解析失败', e);
      }
      try {
        if (sub.filter_json) {
          Object.assign(form.filter, JSON.parse(sub.filter_json) || {});
        }
      } catch (e) {
        console.warn('filter_json 解析失败', e);
      }

      if (!Array.isArray(form.throttle.dedup_keys)) form.throttle.dedup_keys = [];
      if (!Array.isArray(form.throttle.cooldown_steps_sec)) form.throttle.cooldown_steps_sec = [];

      this.form = form;
      this.cooldownStepsText = (form.throttle.cooldown_steps_sec || []).join(',');
      this.filterText = {
        domains: (form.filter.domains || []).join('\n'),
        exclude_ips: (form.filter.exclude_ips || []).join('\n'),
        keywords: (form.filter.keywords || []).join('\n'),
      };
      this.preview = { title: '', content: '', is_fallback: false };
      this.dryRun = {};
    },
    async loadTemplateVars() {
      try {
        const res = await getNotifyTemplateVars({ message_type: this.subscription.message_type });
        if (res.code === 0) {
          this.templateVars = res.data.vars || [];
          this.defaultTitle = res.data.default_title || '';
          this.defaultContent = res.data.default_content || '';
        }
      } catch (e) {
        console.error(e);
      }
    },
    // 变量的展示写法。不能直接写在模板里：{{ '{{.x}}' }} 会被 Vue 的插值解析器提前截断
    varLabel(name: string) {
      return `{{.${name}}}`;
    },
    insertVar(name: string) {
      this.form.content_template = `${this.form.content_template || ''}${this.varLabel(name)}`;
    },
    resetTemplate() {
      this.form.title_template = '';
      this.form.content_template = '';
      this.preview = { title: '', content: '', is_fallback: false };
    },
    splitLines(text: string): string[] {
      return (text || '')
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter((s) => s !== '');
    },
    buildPayload() {
      const steps = this.splitLines(this.cooldownStepsText)
        .map((s) => parseInt(s, 10))
        .filter((n) => !isNaN(n) && n > 0);
      return {
        id: this.form.id,
        throttle_mode: this.form.throttle_mode,
        throttle: {
          ...this.form.throttle,
          cooldown_steps_sec: steps,
        },
        filter: {
          domains: this.splitLines(this.filterText.domains),
          exclude_ips: this.splitLines(this.filterText.exclude_ips),
          keywords: this.splitLines(this.filterText.keywords),
          min_severity: this.form.filter.min_severity || '',
        },
        title_template: this.form.title_template || '',
        content_template: this.form.content_template || '',
      };
    },
    async handleSave() {
      this.saving = true;
      try {
        const res = await saveNotifySubscriptionConfig(this.buildPayload());
        if (res.code === 0) {
          MessagePlugin.success(this.$t('page.notify_subscription.config_save_success') as string);
          this.$emit('saved');
          this.handleClose();
        } else {
          MessagePlugin.error(res.msg || (this.$t('page.notify_subscription.config_save_failed') as string));
        }
      } catch (e) {
        MessagePlugin.error(this.$t('page.notify_subscription.config_save_failed') as string);
      } finally {
        this.saving = false;
      }
    },
    // 模板套用到本消息类型的所有渠道：省得一个格子一个格子配
    async applyToAllChannels() {
      try {
        const payload: any = this.buildPayload();
        const res = await batchNotifySubscriptionConfig({
          message_type: this.subscription.message_type,
          throttle_mode: payload.throttle_mode,
          throttle: payload.throttle,
          filter: payload.filter,
          title_template: payload.title_template,
          content_template: payload.content_template,
          apply_throttle: false,
          apply_template: true,
          apply_filter: false,
        });
        if (res.code === 0) {
          MessagePlugin.success(
            this.$t('page.notify_subscription.batch_config_success', {
              success: res.data.success,
              total: res.data.total,
            }) as string,
          );
          this.$emit('saved');
        } else {
          MessagePlugin.error(res.msg || (this.$t('page.notify_subscription.config_save_failed') as string));
        }
      } catch (e) {
        MessagePlugin.error(this.$t('page.notify_subscription.config_save_failed') as string);
      }
    },
    async doPreview() {
      try {
        const res = await previewNotifySubscription({
          message_type: this.subscription.message_type,
          channel_type: this.channelType,
          title_template: this.form.title_template || '',
          content_template: this.form.content_template || '',
        });
        if (res.code === 0) {
          this.preview = {
            title: res.data.title,
            content: res.data.content,
            is_fallback: res.data.is_fallback,
          };
        } else {
          MessagePlugin.error(res.msg);
        }
      } catch (e) {
        console.error(e);
      }
    },
    async doDryRun() {
      this.dryRunLoading = true;
      try {
        const res = await dryRunNotifySubscription({ id: this.form.id });
        if (res.code === 0) {
          this.dryRun = res.data;
        } else {
          MessagePlugin.error(res.msg);
        }
      } catch (e) {
        console.error(e);
      } finally {
        this.dryRunLoading = false;
      }
    },
    async doTestSend() {
      this.testLoading = true;
      try {
        const res = await testNotifySubscription({
          id: this.form.id,
          title_template: this.form.title_template || '',
          content_template: this.form.content_template || '',
        });
        if (res.code === 0) {
          MessagePlugin.success(res.msg);
        } else {
          MessagePlugin.error(res.msg);
        }
      } catch (e) {
        MessagePlugin.error('测试发送失败');
      } finally {
        this.testLoading = false;
      }
    },
    handleClose() {
      this.innerVisible = false;
      this.$emit('update:visible', false);
    },
  },
});
</script>

<style lang="less" scoped>
.tab-body {
  padding: 16px 8px;
}

.form-tip {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
  line-height: 1.6;
}

.form-tip-inline {
  font-size: 12px;
  color: #888;
  margin-left: 8px;
}

.var-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;

  .var-tag {
    cursor: pointer;

    .var-desc {
      color: #888;
      margin-left: 4px;
    }
  }
}

.preview-box {
  margin-top: 16px;
  padding: 12px;
  border: 1px solid #e7e7e7;
  border-radius: 6px;
  background: #fafafa;

  .preview-head {
    font-size: 12px;
    color: #888;
    margin-bottom: 8px;
  }

  .preview-title {
    font-weight: 600;
    margin-bottom: 8px;
  }

  .preview-content {
    white-space: pre-wrap;
    word-break: break-all;
    font-size: 13px;
    margin: 0;
  }
}

.dryrun-stat {
  margin-top: 8px;
  font-size: 12px;
  color: #666;

  span {
    margin-right: 12px;
  }
}

.drawer-footer {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 12px 24px;
  border-top: 1px solid #e7e7e7;
  background: #fff;
  text-align: right;

  .t-button + .t-button {
    margin-left: 8px;
  }
}
</style>
