<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-dialog
    :header="$t('page.notify_subscription.batch_config_title') + ' - ' + channelLabel"
    v-model:visible="innerVisible"
    :width="640"
    :footer="false"
    @close="handleClose"
  >
    <template #body><div >
      <hw-form :labelWidth="130">
        <hw-form-item :label="$t('page.notify_subscription.label_throttle_mode')">
          <hw-radio-group v-model="form.throttle_mode">
            <hw-radio value="inherit">{{ $t('page.notify_subscription.throttle_mode_inherit') }}</hw-radio>
            <hw-radio value="realtime">{{ $t('page.notify_subscription.throttle_mode_realtime') }}</hw-radio>
            <hw-radio value="aggregate">{{ $t('page.notify_subscription.throttle_mode_aggregate') }}</hw-radio>
            <hw-radio value="cooldown">{{ $t('page.notify_subscription.throttle_mode_cooldown') }}</hw-radio>
          </hw-radio-group>
        </hw-form-item>

        <hw-form-item v-if="form.throttle_mode === 'aggregate'" :label="$t('page.notify_subscription.label_aggregate_window')">
          <hw-input-number v-model="form.throttle.aggregate_window_sec" :min="0" :max="3600" theme="normal" :style="{ width: '160px' }" />
        </hw-form-item>

        <hw-form-item v-if="form.throttle_mode === 'cooldown'" :label="$t('page.notify_subscription.label_cooldown_steps')">
          <hw-input v-model="cooldownStepsText" placeholder="60,300,900" :style="{ width: '260px' }" />
          <div class="form-tip">{{ $t('page.notify_subscription.cooldown_steps_tip') }}</div>
        </hw-form-item>

        <hw-form-item v-if="form.throttle_mode !== 'inherit'" :label="$t('page.notify_subscription.label_max_per_hour')">
          <hw-input-number v-model="form.throttle.max_per_hour" :min="0" :max="10000" theme="normal" :style="{ width: '160px' }" />
          <div class="form-tip">{{ $t('page.notify_subscription.max_per_hour_tip') }}</div>
        </hw-form-item>

        <hw-form-item v-if="form.throttle_mode !== 'inherit'" :label="$t('page.notify_subscription.label_quiet_hours')">
          <hw-input v-model="form.throttle.quiet_hours" placeholder="23:00-07:00" :style="{ width: '200px' }" />
          <div class="form-tip">{{ $t('page.notify_subscription.quiet_hours_tip') }}</div>
        </hw-form-item>

        <hw-form-item>
          <!-- 只套用勾选的部分：批量最怕的就是把别人配好的模板一起覆盖掉 -->
          <hw-checkbox v-model="applyThrottle">{{ $t('page.notify_subscription.batch_config_apply_throttle') }}</hw-checkbox>
        </hw-form-item>

        <hw-form-item style="float: right">
          <hw-button variant="outline" @click="handleClose">{{ $t('common.close') }}</hw-button>
          <hw-button theme="primary" @click="handleSubmit" :loading="saving" :style="{ marginLeft: '8px' }">
            {{ $t('common.confirm') }}
          </hw-button>
        </hw-form-item>
      </hw-form>
    </div></template>
  </hw-dialog>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { MessagePlugin } from '@/ui';
import { batchNotifySubscriptionConfig } from '@/apis/notify_subscription';

export default defineComponent({
  name: 'NotifySubscriptionBatchConfigDialog',
  props: {
    visible: { type: Boolean, default: false },
    channelType: { type: String, default: '' },
    channelLabel: { type: String, default: '' },
  },
  data() {
    return {
      innerVisible: false,
      saving: false,
      applyThrottle: true,
      cooldownStepsText: '',
      form: {
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
      },
    };
  },
  watch: {
    visible(val: boolean) {
      this.innerVisible = val;
    },
  },
  methods: {
    async handleSubmit() {
      if (!this.applyThrottle) {
        MessagePlugin.warning(this.$t('page.notify_subscription.batch_config_apply_throttle') as string);
        return;
      }
      const steps = (this.cooldownStepsText || '')
        .split(/[\n,]/)
        .map((s) => parseInt(s.trim(), 10))
        .filter((n) => !isNaN(n) && n > 0);

      this.saving = true;
      try {
        const res = await batchNotifySubscriptionConfig({
          channel_type: this.channelType,
          throttle_mode: this.form.throttle_mode,
          throttle: { ...this.form.throttle, cooldown_steps_sec: steps },
          filter: { domains: [], exclude_ips: [], keywords: [], min_severity: '' },
          title_template: '',
          content_template: '',
          apply_throttle: true,
          apply_template: false,
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
    handleClose() {
      this.innerVisible = false;
      this.$emit('update:visible', false);
    },
  },
});
</script>

<style lang="less" scoped>
.form-tip {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
}
</style>
