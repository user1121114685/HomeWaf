<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="anti-leech-config">
    <hw-form-item :label="$t('page.host.anti_leech.is_enable')">
      <hw-radio-group v-model="localConfig.is_enable_anti_leech" @change="updateParent">
        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.anti_leech.file_types')" v-if="localConfig.is_enable_anti_leech == '1'">
      <hw-input v-model="localConfig.file_types" @change="updateParent" :placeholder="$t('page.host.anti_leech.file_types_placeholder')" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.anti_leech.valid_referers')" v-if="localConfig.is_enable_anti_leech == '1'">
        <hw-textarea v-model="localConfig.valid_referers" @change="updateParent" :placeholder="$t('page.host.anti_leech.valid_referers_placeholder')" />
      </hw-form-item>
      <hw-form-item  v-if="localConfig.is_enable_anti_leech == '1'">
        <hw-alert theme="info" :message="$t('page.host.anti_leech.valid_referers_desc')" style="margin-top: 4px;" />
      </hw-form-item>
    <hw-form-item :label="$t('page.host.anti_leech.action')" v-if="localConfig.is_enable_anti_leech == '1'">
      <hw-radio-group v-model="localConfig.action" @change="updateParent">
        <hw-radio value="block">{{ $t('page.host.anti_leech.action_block') }}</hw-radio>
        <hw-radio value="redirect">{{ $t('page.host.anti_leech.action_redirect') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.anti_leech.redirect_url')" v-if="localConfig.is_enable_anti_leech == '1' && localConfig.action === 'redirect'">
      <hw-input v-model="localConfig.redirect_url" @change="updateParent" :placeholder="$t('page.host.anti_leech.redirect_url_placeholder')" />
    </hw-form-item>
  </div>
</template>

<script lang="ts">
export default {
  name: 'AntiLeechConfig',
  props: {
    antiLeechConfig: {
      type: Object,
      required: true
    }
  },
  data() {
    return {
      localConfig: JSON.parse(JSON.stringify(this.antiLeechConfig))
    };
  },
  watch: {
    antiLeechConfig: {
      handler(newVal) {
        this.localConfig = JSON.parse(JSON.stringify(newVal));
      },
      immediate: true
    }
  },
  methods: {
    updateParent() {
      this.$emit('update', JSON.parse(JSON.stringify(this.localConfig)));
    }
  }
};
</script>