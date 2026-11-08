<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="cookie-security-config">
    <hw-alert theme="info" :message="$t('page.host.cookie_security.intro')" style="margin-bottom: 16px;" />
    <hw-form-item :label="$t('page.host.cookie_security.is_enable')">
      <hw-radio-group v-model="local.is_enable" @change="updateParent">
        <hw-radio value="0">{{ $t('page.host.cookie_security.disable') }}</hw-radio>
        <hw-radio value="1">{{ $t('page.host.cookie_security.enable') }}</hw-radio>
      </hw-radio-group>
      <hw-button size="small" variant="outline" style="margin-left: 12px;" @click="applyRecommended">
        {{ $t('page.host.cookie_security.recommended') }}
      </hw-button>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cookie_security.http_only')">
      <hw-tooltip class="placement top center" :content="$t('page.host.cookie_security.http_only_tips')" placement="top"
                 :overlay-style="{ width: '320px' }" show-arrow>
        <hw-radio-group v-model="local.http_only" @change="updateParent">
          <hw-radio value="0">{{ $t('page.host.cookie_security.no_touch') }}</hw-radio>
          <hw-radio value="1">{{ $t('page.host.cookie_security.add_if_missing') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cookie_security.secure')">
      <hw-tooltip class="placement top center" :content="$t('page.host.cookie_security.secure_tips')" placement="top"
                 :overlay-style="{ width: '340px' }" show-arrow>
        <hw-select v-model="local.secure" :style="{ width: '280px' }" @change="updateParent">
          <hw-option value="0" :label="$t('page.host.cookie_security.secure_0')" />
          <hw-option value="2" :label="$t('page.host.cookie_security.secure_2')" />
          <hw-option value="1" :label="$t('page.host.cookie_security.secure_1')" />
        </hw-select>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cookie_security.same_site')">
      <hw-tooltip class="placement top center" :content="$t('page.host.cookie_security.same_site_tips')" placement="top"
                 :overlay-style="{ width: '340px' }" show-arrow>
        <hw-select v-model="local.same_site" :style="{ width: '280px' }" @change="updateParent">
          <hw-option value="" :label="$t('page.host.cookie_security.same_site_empty')" />
          <hw-option value="Lax" label="Lax" />
          <hw-option value="Strict" label="Strict" />
          <hw-option value="None" label="None" />
        </hw-select>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cookie_security.exclude_cookies')">
      <hw-tooltip class="placement top center" :content="$t('page.host.cookie_security.exclude_cookies_tips')" placement="top"
                 :overlay-style="{ width: '360px' }" show-arrow>
        <hw-textarea v-model="local.exclude_cookies" :placeholder="$t('page.host.cookie_security.exclude_cookies_ph')"
                    :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: '480px' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
export default {
  name: 'CookieSecurityConfig',
  props: {
    cookieSecurityConfig: {
      type: Object,
      required: true
    }
  },
  data() {
    return { local: JSON.parse(JSON.stringify(this.cookieSecurityConfig)) };
  },
  watch: {
    cookieSecurityConfig: {
      handler(newVal) {
        this.local = JSON.parse(JSON.stringify(newVal));
      },
      deep: true
    }
  },
  methods: {
    updateParent() {
      this.$emit('update', { ...this.local });
    },
    applyRecommended() {
      this.local = { is_enable: '1', http_only: '1', secure: '2', same_site: 'Lax', exclude_cookies: this.local.exclude_cookies || '' };
      this.updateParent();
    }
  }
};
</script>
