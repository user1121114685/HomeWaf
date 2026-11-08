<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-dialog
    :header="$t('page.notify_subscription.global_config_title')"
    v-model:visible="innerVisible"
    :width="660"
    :footer="false"
    @close="handleClose"
  >
    <template #body><div >
      <hw-alert theme="info" :message="$t('page.notify_subscription.throttle_mode_inherit_tip')" />
      <hw-form :labelWidth="140" :style="{ marginTop: '12px' }">
        <hw-form-item :label="$t('page.notify_subscription.label_throttle_mode')">
          <hw-radio-group v-model="form.mode">
            <hw-radio value="realtime">{{ $t('page.notify_subscription.throttle_mode_realtime') }}</hw-radio>
            <hw-radio value="aggregate">{{ $t('page.notify_subscription.throttle_mode_aggregate') }}</hw-radio>
            <hw-radio value="cooldown">{{ $t('page.notify_subscription.throttle_mode_cooldown') }}</hw-radio>
          </hw-radio-group>
        </hw-form-item>

        <hw-form-item :label="$t('page.notify_subscription.label_aggregate_window')">
          <hw-input-number v-model="form.throttle.aggregate_window_sec" :min="1" :max="3600" theme="normal" :style="{ width: '160px' }" />
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.label_aggregate_max_detail')">
          <hw-input-number v-model="form.throttle.aggregate_max_detail" :min="1" :max="50" theme="normal" :style="{ width: '160px' }" />
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.label_cooldown_steps')">
          <hw-input v-model="cooldownStepsText" placeholder="60,300,900" :style="{ width: '260px' }" />
          <div class="form-tip">{{ $t('page.notify_subscription.cooldown_steps_tip') }}</div>
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.label_cooldown_reset')">
          <hw-input-number v-model="form.throttle.cooldown_reset_sec" :min="1" :max="86400" theme="normal" :style="{ width: '160px' }" />
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.label_max_per_hour')">
          <hw-input-number v-model="form.throttle.max_per_hour" :min="0" :max="10000" theme="normal" :style="{ width: '160px' }" />
          <div class="form-tip">{{ $t('page.notify_subscription.max_per_hour_tip') }}</div>
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.label_dedup_keys')">
          <hw-checkbox-group v-model="form.throttle.dedup_keys">
            <hw-checkbox value="message_type">{{ $t('page.notify_subscription.dedup_key_message_type') }}</hw-checkbox>
            <hw-checkbox value="domain">{{ $t('page.notify_subscription.dedup_key_domain') }}</hw-checkbox>
            <hw-checkbox value="ip">{{ $t('page.notify_subscription.dedup_key_ip') }}</hw-checkbox>
            <hw-checkbox value="attack_type">{{ $t('page.notify_subscription.dedup_key_attack_type') }}</hw-checkbox>
            <hw-checkbox value="rule">{{ $t('page.notify_subscription.dedup_key_rule') }}</hw-checkbox>
          </hw-checkbox-group>
          <div class="form-tip">{{ $t('page.notify_subscription.dedup_keys_tip') }}</div>
        </hw-form-item>
        <hw-form-item :label="$t('page.notify_subscription.global_debug_mode')">
          <hw-switch v-model="form.debug_mode" />
          <div class="form-tip">{{ $t('page.notify_subscription.global_debug_mode_tip') }}</div>
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
import { getNotifyGlobalThrottle, updateNotifyGlobalThrottle } from '@/apis/notify_subscription';

export default defineComponent({
  name: 'NotifyGlobalThrottleDialog',
  props: {
    visible: { type: Boolean, default: false },
  },
  data() {
    return {
      innerVisible: false,
      saving: false,
      cooldownStepsText: '60,300,900',
      form: {
        mode: 'aggregate',
        debug_mode: false,
        throttle: {
          aggregate_window_sec: 10,
          aggregate_max_detail: 10,
          cooldown_steps_sec: [] as number[],
          cooldown_reset_sec: 1800,
          max_per_hour: 0,
          dedup_keys: ['message_type', 'domain', 'attack_type'] as string[],
          quiet_hours: '',
          quiet_hours_bypass_severity: '',
        },
      },
    };
  },
  watch: {
    visible(val: boolean) {
      this.innerVisible = val;
      if (val) this.loadConfig();
    },
  },
  methods: {
    async loadConfig() {
      try {
        const res = await getNotifyGlobalThrottle();
        if (res.code === 0) {
          this.form.mode = res.data.mode || 'aggregate';
          this.form.debug_mode = !!res.data.debug_mode;
          Object.assign(this.form.throttle, res.data.throttle || {});
          if (!Array.isArray(this.form.throttle.dedup_keys)) this.form.throttle.dedup_keys = [];
          this.cooldownStepsText = (this.form.throttle.cooldown_steps_sec || []).join(',');
        }
      } catch (e) {
        console.error(e);
      }
    },
    async handleSubmit() {
      const steps = (this.cooldownStepsText || '')
        .split(/[\n,]/)
        .map((s) => parseInt(s.trim(), 10))
        .filter((n) => !isNaN(n) && n > 0);

      this.saving = true;
      try {
        const res = await updateNotifyGlobalThrottle({
          mode: this.form.mode,
          debug_mode: this.form.debug_mode,
          throttle: { ...this.form.throttle, cooldown_steps_sec: steps },
        });
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
  line-height: 1.6;
}
</style>
