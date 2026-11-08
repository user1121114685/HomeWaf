<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="csrf-config">
    <hw-alert theme="info" :message="$t('page.host.csrf.intro')" style="margin-bottom: 16px;" />
    <hw-form-item :label="$t('page.host.csrf.is_enable')">
      <hw-radio-group v-model="local.is_enable" @change="updateParent">
        <hw-radio value="0">{{ $t('page.host.csrf.disable') }}</hw-radio>
        <hw-radio value="1">{{ $t('page.host.csrf.enable') }}</hw-radio>
      </hw-radio-group>
      <hw-button size="small" variant="outline" style="margin-left: 12px;" @click="applyRecommended">
        {{ $t('page.host.csrf.recommended') }}
      </hw-button>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.csrf.protect_methods')">
      <hw-tooltip class="placement top center" :content="$t('page.host.csrf.protect_methods_tips')" placement="top"
                 :overlay-style="{ width: '320px' }" show-arrow>
        <hw-checkbox-group v-model="local.protect_methods" @change="updateParent">
          <hw-checkbox value="POST">POST</hw-checkbox>
          <hw-checkbox value="PUT">PUT</hw-checkbox>
          <hw-checkbox value="DELETE">DELETE</hw-checkbox>
          <hw-checkbox value="PATCH">PATCH</hw-checkbox>
        </hw-checkbox-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.csrf.allowed_origins')">
      <hw-tooltip class="placement top center" :content="$t('page.host.csrf.allowed_origins_tips')" placement="top"
                 :overlay-style="{ width: '360px' }" show-arrow>
        <hw-textarea v-model="local.allowed_origins" :placeholder="$t('page.host.csrf.allowed_origins_ph')"
                    :autosize="{ minRows: 2, maxRows: 5 }" :style="{ width: '480px' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.csrf.allow_empty_ref')">
      <hw-tooltip class="placement top center" :content="$t('page.host.csrf.allow_empty_ref_tips')" placement="top"
                 :overlay-style="{ width: '340px' }" show-arrow>
        <hw-radio-group v-model="local.allow_empty_ref" @change="updateParent">
          <hw-radio value="1">{{ $t('page.host.csrf.allow_empty_ref_pass') }}</hw-radio>
          <hw-radio value="0">{{ $t('page.host.csrf.allow_empty_ref_block') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.csrf.exclude_paths')">
      <hw-tooltip class="placement top center" :content="$t('page.host.csrf.exclude_paths_tips')" placement="top"
                 :overlay-style="{ width: '360px' }" show-arrow>
        <hw-textarea v-model="local.exclude_paths" :placeholder="$t('page.host.csrf.exclude_paths_ph')"
                    :autosize="{ minRows: 2, maxRows: 5 }" :style="{ width: '480px' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
export default {
  name: 'CsrfConfig',
  props: {
    csrfConfig: {
      type: Object,
      required: true
    }
  },
  data() {
    return { local: JSON.parse(JSON.stringify(this.csrfConfig)) };
  },
  watch: {
    csrfConfig: {
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
      this.local = {
        is_enable: '1',
        protect_methods: ['POST', 'PUT', 'DELETE', 'PATCH'],
        allowed_origins: this.local.allowed_origins || '',
        allow_empty_ref: '1',
        exclude_paths: this.local.exclude_paths || '',
      };
      this.updateParent();
    }
  }
};
</script>
