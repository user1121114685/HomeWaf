<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <help-block :summary="$t('page.owasp.upgrade.alert_message')" doc="guide/Owasp" />
    <hw-descriptions :column="1" bordered>
      <hw-descriptions-item :label="$t('page.owasp.upgrade.current_version')">
        {{ info.current_version || '-' }}
      </hw-descriptions-item>
      <hw-descriptions-item :label="$t('page.owasp.upgrade.latest_version')">
        {{ info.latest_version || '-' }}
      </hw-descriptions-item>
      <hw-descriptions-item :label="$t('page.owasp.upgrade.need_update')">
        <hw-tag v-if="info.need_update" theme="warning" variant="light">{{ $t('page.owasp.upgrade.yes') }}</hw-tag>
        <hw-tag v-else theme="success" variant="light">{{ $t('page.owasp.upgrade.no') }}</hw-tag>
      </hw-descriptions-item>
      <hw-descriptions-item :label="$t('page.owasp.upgrade.last_check_at')">
        {{ info.last_check_at || '-' }}
      </hw-descriptions-item>
      <hw-descriptions-item :label="$t('page.owasp.upgrade.changelog')">
        <pre class="changelog">{{ info.changelog || '-' }}</pre>
      </hw-descriptions-item>
    </hw-descriptions>

    <div style="margin-top:16px">
      <hw-button theme="primary" :loading="checking" @click="onCheck(true)">
        {{ $t('page.owasp.upgrade.check') }}
      </hw-button>
      <hw-button theme="warning" :disabled="!info.need_update" :loading="applying" @click="onApply">
        {{ $t('page.owasp.upgrade.apply') }}
      </hw-button>
    </div>

    <hw-alert
      v-if="applying"
      style="margin-top:16px"
      theme="info"
      :message="$t('page.owasp.upgrade.applying_tip')"
    />
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { owaspUpdateCheckApi, owaspUpdateApplyApi } from '@/apis/owasp';

export default defineComponent({
  name: 'OwaspUpgradeTab',
  data() {
    return {
      info: {
        current_version: '',
        latest_version: '',
        need_update: false,
        last_check_at: '',
        changelog: '',
      },
      checking: false,
      applying: false,
    };
  },
  mounted() {
    this.onCheck();
  },
  methods: {
    // manual=true 是用户点按钮；mounted 自动那次走后台通道
    onCheck(manual = false) {
      this.checking = true;
      owaspUpdateCheckApi(manual ? undefined : { background: true })
        .then((res) => {
          if (res.code === 0) {
            this.info = { ...this.info, ...res.data };
          } else {
            this.$message.warning(res.msg);
          }
        })
        .finally(() => (this.checking = false));
    },
    onApply() {
      this.applying = true;
      owaspUpdateApplyApi()
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
          } else {
            this.$message.warning(res.msg);
          }
        })
        .finally(() => {
          setTimeout(() => {
            this.applying = false;
            this.onCheck();
          }, 3000);
        });
    },
  },
});
</script>

<style lang="less" scoped>
.changelog {
  white-space: pre-wrap;
  margin: 0;
}
</style>
