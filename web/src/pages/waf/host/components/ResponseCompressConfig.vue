<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="response-compress-config">
    <hw-alert theme="info" :message="$t('page.host.response_compress.intro')" style="margin-bottom: 16px;" />
    <hw-form-item :label="$t('page.host.response_compress.is_enable')">
      <hw-radio-group v-model="local.is_enable" @change="updateParent">
        <hw-radio value="0">{{ $t('page.host.response_compress.disable') }}</hw-radio>
        <hw-radio value="1">{{ $t('page.host.response_compress.enable') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.prefer')">
      <hw-tooltip class="placement top center" :content="$t('page.host.response_compress.prefer_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 320px)' }" show-arrow>
        <hw-select v-model="local.prefer" :style="{ width: 'min(100%, 280px)' }" @change="updateParent">
          <hw-option value="zstd_first" :label="$t('page.host.response_compress.prefer_zstd_first')" />
          <hw-option value="br_first" :label="$t('page.host.response_compress.prefer_br_first')" />
          <hw-option value="gzip_only" :label="$t('page.host.response_compress.prefer_gzip_only')" />
          <hw-option value="br_only" :label="$t('page.host.response_compress.prefer_br_only')" />
          <hw-option value="zstd_only" :label="$t('page.host.response_compress.prefer_zstd_only')" />
        </hw-select>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.min_length')">
      <hw-tooltip class="placement top center" :content="$t('page.host.response_compress.min_length_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 280px)' }" show-arrow>
        <hw-input-number v-model="local.min_length" :min="0" :style="{ width: '160px' }" @change="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.include_types')">
      <hw-tooltip class="placement top center" :content="$t('page.host.response_compress.include_types_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 360px)' }" show-arrow>
        <hw-textarea v-model="local.include_types" :placeholder="$t('page.host.response_compress.include_types_ph')"
                    :autosize="{ minRows: 2, maxRows: 6 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.include_extensions')">
      <hw-textarea v-model="local.include_extensions" :placeholder="$t('page.host.response_compress.include_extensions_ph')"
                  :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.exclude_extensions')">
      <hw-textarea v-model="local.exclude_extensions" :placeholder="$t('page.host.response_compress.exclude_extensions_ph')"
                  :autosize="{ minRows: 2, maxRows: 4 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.exclude_paths')">
      <hw-textarea v-model="local.exclude_paths" :placeholder="$t('page.host.response_compress.exclude_paths_ph')"
                  :autosize="{ minRows: 2, maxRows: 6 }" :style="{ width: 'min(100%, 480px)' }" @blur="updateParent" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.response_compress.compress_static')">
      <hw-tooltip class="placement top center" :content="$t('page.host.response_compress.compress_static_tips')" placement="top"
                 :overlay-style="{ width: 'min(100%, 380px)' }" show-arrow>
        <hw-radio-group v-model="local.compress_when_static_assist" @change="updateParent">
          <hw-radio value="0">{{ $t('page.host.response_compress.no') }}</hw-radio>
          <hw-radio value="1">{{ $t('page.host.response_compress.yes') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
export default {
  name: 'ResponseCompressConfig',
  props: {
    responseCompressConfig: {
      type: Object,
      required: true
    }
  },
  data() {
    const p = JSON.parse(JSON.stringify(this.responseCompressConfig));
    const n = parseInt(p.min_length, 10);
    p.min_length = isNaN(n) ? 256 : n;
    return { local: p };
  },
  watch: {
    responseCompressConfig: {
      handler(newVal) {
        const p = JSON.parse(JSON.stringify(newVal));
        const n = parseInt(p.min_length, 10);
        p.min_length = isNaN(n) ? 256 : n;
        this.local = p;
      },
      deep: true
    }
  },
  methods: {
    updateParent() {
      const out = { ...this.local, min_length: String(this.local.min_length) };
      this.$emit('update', out);
    }
  }
};
</script>
