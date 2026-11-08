<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="cache-config">
    <hw-form-item :label="$t('page.host.cache.is_enable')">
      <hw-radio-group v-model="localConfig.is_enable_cache" @change="updateParent">
        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cache.cache_location')" v-if="localConfig.is_enable_cache == '1'">
      <hw-radio-group v-model="localConfig.cache_location" @change="updateParent">
        <hw-radio value="memory">{{ $t('page.host.cache.cache_location_memory') }}</hw-radio>
        <hw-radio value="file">{{ $t('page.host.cache.cache_location_file') }}</hw-radio>
        <hw-radio value="all">{{ $t('page.host.cache.cache_location_all') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cache.cache_dir')" v-if="localConfig.is_enable_cache == '1' && (localConfig.cache_location === 'file' || localConfig.cache_location === 'all')">
      <hw-input v-model="localConfig.cache_dir" @change="updateParent" :placeholder="$t('page.host.cache.cache_dir_placeholder')" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cache.max_file_size_mb')" v-if="localConfig.is_enable_cache == '1' && (localConfig.cache_location === 'file' || localConfig.cache_location === 'all')">
      <hw-input v-model="localConfig.max_file_size_mb" @change="updateParent" :placeholder="$t('page.host.cache.max_file_size_mb_placeholder')" />
    </hw-form-item>
    <hw-form-item :label="$t('page.host.cache.max_memory_size_mb')" v-if="localConfig.is_enable_cache == '1' && (localConfig.cache_location === 'memory' || localConfig.cache_location === 'all')">
      <hw-input v-model="localConfig.max_memory_size_mb" @change="updateParent" :placeholder="$t('page.host.cache.max_memory_size_mb_placeholder')" />
    </hw-form-item>
    <hw-form-item v-if="localConfig.is_enable_cache == '1'" class="cache-rule-container">
      <cache-rule :prop-host-code="propHostCode"></cache-rule>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
import CacheRule from "@/pages/waf/cache_rule/index.vue";
export default {
  name: 'CacheConfig',
  props: {
    cacheConfig: {
      type: Object,
      required: true
    },
    propHostCode:{
      type:String,
      required: true
    }
  },
  components:{
    CacheRule
  },
  data() {
    return {
      localConfig: JSON.parse(JSON.stringify(this.cacheConfig))
    };
  },
  watch: {
    cacheConfig: {
      handler(newVal) {
        this.localConfig = JSON.parse(JSON.stringify(newVal));
      },
      immediate: true
    },
  },
  methods: {
    updateParent() {
      this.$emit('update', JSON.parse(JSON.stringify(this.localConfig)));
    }
  }
};
</script>

<style lang="less" scoped>
.cache-rule-container {
  width: 100%;
  overflow-x: auto;
  max-width: 100%;
}

.cache-rule-container :deep(.t-form__item-content) {
  width: 100%;
  overflow-x: auto;
}
</style>