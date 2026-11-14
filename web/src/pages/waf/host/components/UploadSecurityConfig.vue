<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="upload-security-config">
    <hw-alert theme="info" :message="$t('page.host.upload_security.intro')" style="margin-bottom: 16px;" />

    <hw-form-item :label="$t('page.host.upload_security.is_enable')">
      <hw-radio-group v-model="local.is_enable" @change="updateParent">
        <hw-radio value="0">{{ $t('page.host.upload_security.disable') }}</hw-radio>
        <hw-radio value="1">{{ $t('page.host.upload_security.enable') }}</hw-radio>
      </hw-radio-group>
      <hw-button size="small" variant="outline" style="margin-left: 12px;" @click="applyRecommended">
        {{ $t('page.host.upload_security.recommended') }}
      </hw-button>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.upload_security.check_ext')">
      <hw-radio-group v-model="local.check_ext" @change="updateParent">
        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item v-if="local.check_ext === '1'" :label="$t('page.host.upload_security.ext_blacklist')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.ext_blacklist_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-textarea v-model="local.ext_blacklist" :placeholder="$t('page.host.upload_security.ext_blacklist_ph')"
                    :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.upload_security.check_content')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.check_content_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-radio-group v-model="local.check_content" @change="updateParent">
          <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
          <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.upload_security.check_magic')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.check_magic_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-radio-group v-model="local.check_magic" @change="updateParent">
          <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
          <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.upload_security.check_size')">
      <hw-radio-group v-model="local.check_size" @change="updateParent">
        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.upload_security.max_size_kb')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.max_size_kb_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-input-number v-model="local.max_size_kb" :min="1" :style="{ width: '180px' }" @change="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.upload_security.over_limit_action')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.over_limit_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-radio-group v-model="local.over_limit_action" @change="updateParent">
          <hw-radio value="block">{{ $t('page.host.upload_security.over_limit_block') }}</hw-radio>
          <hw-radio value="pass">{{ $t('page.host.upload_security.over_limit_pass') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.upload_security.include_paths')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.include_paths_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-textarea v-model="local.include_paths" :placeholder="$t('page.host.upload_security.include_paths_ph')"
                    :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.upload_security.exclude_paths')">
      <hw-tooltip class="placement top center" :content="$t('page.host.upload_security.exclude_paths_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-textarea v-model="local.exclude_paths" :placeholder="$t('page.host.upload_security.exclude_paths_ph')"
                    :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
export default {
  name: 'UploadSecurityConfig',
  props: {
    uploadSecurityConfig: {
      type: Object,
      required: true,
    },
  },
  data() {
    return { local: JSON.parse(JSON.stringify(this.uploadSecurityConfig)) };
  },
  watch: {
    uploadSecurityConfig: {
      handler(newVal) {
        this.local = JSON.parse(JSON.stringify(newVal));
      },
      deep: true,
    },
  },
  methods: {
    updateParent() {
      this.$emit('update', { ...this.local });
    },
    applyRecommended() {
      this.local = {
        is_enable: '1',
        check_ext: '1',
        ext_blacklist: this.local.ext_blacklist || '',
        check_content: '1',
        check_magic: '1',
        check_size: '1',
        max_size_kb: 10240,
        over_limit_action: 'block',
        include_paths: this.local.include_paths || '',
        exclude_paths: this.local.exclude_paths || '',
      };
      this.updateParent();
    },
  },
};
</script>
