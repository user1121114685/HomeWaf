<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-dialog
    :header="$t('page.hostguard.ban_dialog_title')"
    :visible="visible"
    :width="560"
    :footer="false"
    @close="onClose"
  >
    <template #body><div >
      <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="110">
        <hw-form-item :label="$t('page.hostguard.col_ip')" name="ip">
          <hw-input v-model="formData.ip" :style="{ width: '360px' }" />
        </hw-form-item>
        <hw-form-item :label="$t('page.hostguard.ban_minutes')" name="ban_minutes">
          <hw-input-number v-model="formData.ban_minutes" :min="0" theme="column" :style="{ width: '360px' }" />
          <div class="desc">{{ $t('page.hostguard.ban_minutes_desc') }}</div>
        </hw-form-item>
        <hw-form-item :label="$t('page.hostguard.ban_reason')" name="reason">
          <hw-input v-model="formData.reason" :style="{ width: '360px' }" />
        </hw-form-item>
        <hw-form-item style="float: right">
          <hw-button variant="outline" @click="onClose">{{ $t('common.close') }}</hw-button>
          <hw-button theme="primary" type="submit" style="margin-left: 8px">{{ $t('common.confirm') }}</hw-button>
        </hw-form-item>
      </hw-form>
    </div></template>
  </hw-dialog>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { wafHostGuardBanManualApi } from '@/apis/hostguard';

export default defineComponent({
  name: 'HostGuardBanDialog',
  props: {
    visible: { type: Boolean, default: false },
    ip: { type: String, default: '' },
    source: { type: String, default: 'ssh' },
  },
  data() {
    return {
      formData: { ip: '', ban_minutes: 60, reason: '', source: 'ssh' },
      rules: {
        ip: [{ required: true, message: this.$t('common.required'), type: 'error' }],
      },
    };
  },
  watch: {
    visible(val) {
      if (val) {
        this.formData = {
          ip: this.ip,
          ban_minutes: 60,
          reason: '',
          source: this.source || 'ssh',
        };
      }
    },
  },
  methods: {
    onClose() {
      this.$emit('update:visible', false);
    },
    onSubmit({ firstError }) {
      if (firstError) {
        this.$message.warning(firstError);
        return;
      }
      wafHostGuardBanManualApi(this.formData)
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
            this.onClose();
            this.$emit('done');
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch(() => {});
    },
  },
});
</script>

<style lang="less" scoped>
// 同 SettingPanel：让说明文字独占一行，别和控件抢 flex 行内空间
:deep(.el-form-item__content) {
  flex-wrap: wrap;
}
.desc {
  flex-basis: 100%;
  width: 100%;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  margin-top: 4px;
}
</style>
