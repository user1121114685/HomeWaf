<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="static-site-config">
    <hw-form-item :label="$t('page.host.static_site.is_enable')">
      <hw-radio-group v-model="localConfig.is_enable_static_site" @change="updateParent">
        <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
        <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
      </hw-radio-group>
    </hw-form-item>

    <!-- 仅主机全局静态站点需要的路径字段 -->
    <hw-form-item :label="$t('page.host.static_site.static_site_path')" v-if="localConfig.is_enable_static_site == '1'">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.static_site_path_tips')" placement="top"
               :overlay-style="{ width: '300px' }" show-arrow>
        <hw-input v-model="localConfig.static_site_path" @change="updateParent" :placeholder="$t('page.host.static_site.static_site_path_placeholder')" />
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.static_site.static_site_prefix')" v-if="localConfig.is_enable_static_site == '1'">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.static_site_prefix_tips')" placement="top"
               :overlay-style="{ width: '300px' }" show-arrow>
        <hw-input v-model="localConfig.static_site_prefix" @change="updateParent" :placeholder="$t('page.host.static_site.static_site_prefix_placeholder')" />
      </hw-tooltip>
    </hw-form-item>

    <!-- 安全配置区域：始终显示，同时作用于主机全局静态和路径规则静态文件服务 -->
    <hw-divider align="left" style="margin: 16px 0 8px;">{{ $t('page.host.static_site.security_section_title') }}</hw-divider>
    <hw-form-item>
      <hw-alert theme="info" :message="$t('page.host.static_site_security_shared_tips')" :close="false" style="margin-bottom:12px;" />
    </hw-form-item>

    <hw-form-item :label="$t('page.host.static_site.sensitive_paths')">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.sensitive_paths_tips')" placement="top"
               :overlay-style="{ width: '400px' }" show-arrow>
        <hw-textarea v-model="localConfig.sensitive_paths" @change="updateParent" :placeholder="$t('page.host.static_site.sensitive_paths_placeholder')" :rows="3" />
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.static_site.sensitive_extensions')">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.sensitive_extensions_tips')" placement="top"
               :overlay-style="{ width: '400px' }" show-arrow>
        <hw-input v-model="localConfig.sensitive_extensions" @change="updateParent" :placeholder="$t('page.host.static_site.sensitive_extensions_placeholder')" />
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.static_site.allowed_extensions')">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.allowed_extensions_tips')" placement="top"
               :overlay-style="{ width: '400px' }" show-arrow>
        <hw-input v-model="localConfig.allowed_extensions" @change="updateParent" :placeholder="$t('page.host.static_site.allowed_extensions_placeholder')" />
      </hw-tooltip>
    </hw-form-item>

    <hw-form-item :label="$t('page.host.static_site.sensitive_patterns')">
      <hw-tooltip class="placement top center" :content="$t('page.host.static_site.sensitive_patterns_tips')" placement="top"
               :overlay-style="{ width: '400px' }" show-arrow>
        <hw-textarea v-model="localConfig.sensitive_patterns" @change="updateParent" :placeholder="$t('page.host.static_site.sensitive_patterns_placeholder')" :rows="3" />
      </hw-tooltip>
    </hw-form-item>

    <!-- 安全响应头配置 -->
    <hw-form-item>
      <template #label>
        <hw-tooltip :content="$t('page.host.static_site.security_headers_tips')" placement="top"
                   :overlay-style="{ width: '420px' }" show-arrow>
          <span>{{ $t('page.host.static_site.security_headers') }}</span>
        </hw-tooltip>
      </template>
      <div class="security-headers-wrapper">
        <div v-if="localConfig.security_headers && localConfig.security_headers.length > 0">
          <div
            v-for="(header, index) in localConfig.security_headers"
            :key="index"
            class="security-header-row"
          >
            <hw-input
              v-model="header.header_name"
              :placeholder="$t('page.host.static_site.security_header_name')"
              class="header-name-input"
              @change="updateParent"
            />
            <hw-input
              v-model="header.header_value"
              :placeholder="$t('page.host.static_site.security_header_value')"
              class="header-value-input"
              @change="updateParent"
            />
            <hw-button theme="danger" variant="text" @click="removeSecurityHeader(index)">
              {{ $t('page.host.static_site.security_header_delete') }}
            </hw-button>
          </div>
        </div>
        <div v-else class="security-headers-empty">
          <hw-tag theme="default" variant="light">{{ $t('common.default_value') || '使用系统默认' }}</hw-tag>
        </div>
        <div class="security-header-actions">
          <hw-button theme="primary" variant="outline" size="small" @click="addSecurityHeader">
            + {{ $t('page.host.static_site.security_header_add') }}
          </hw-button>
          <hw-button theme="default" variant="outline" size="small" @click="resetSecurityHeaders">
            {{ $t('page.host.static_site.security_headers_reset') }}
          </hw-button>
        </div>
      </div>
    </hw-form-item>
  </div>
</template>

<script lang="ts">
const DEFAULT_SECURITY_HEADERS = [
  { header_name: 'X-Content-Type-Options',  header_value: 'nosniff' },
  { header_name: 'X-Frame-Options',         header_value: 'DENY' },
  { header_name: 'X-XSS-Protection',        header_value: '1; mode=block' },
  { header_name: 'Referrer-Policy',         header_value: 'strict-origin-when-cross-origin' },
  { header_name: 'Content-Security-Policy', header_value: "default-src 'self'" },
  { header_name: 'Cache-Control',           header_value: 'public, max-age=3600' },
];

export default {
  name: 'StaticSiteConfig',
  props: {
    staticSiteConfig: {
      type: Object,
      required: true
    }
  },
  data() {
    return {
      localConfig: JSON.parse(JSON.stringify(this.staticSiteConfig))
    };
  },
  watch: {
    staticSiteConfig: {
      handler(newVal) {
        this.localConfig = JSON.parse(JSON.stringify(newVal));
        if (!Array.isArray(this.localConfig.security_headers)) {
          this.localConfig.security_headers = [];
        }
      },
      immediate: true
    },
  },
  methods: {
    updateParent() {
      this.$emit('update', JSON.parse(JSON.stringify(this.localConfig)));
    },
    addSecurityHeader() {
      if (!Array.isArray(this.localConfig.security_headers)) {
        this.localConfig.security_headers = [];
      }
      this.localConfig.security_headers.push({ header_name: '', header_value: '' });
      this.updateParent();
    },
    removeSecurityHeader(index) {
      this.localConfig.security_headers.splice(index, 1);
      this.updateParent();
    },
    resetSecurityHeaders() {
      this.localConfig.security_headers = JSON.parse(JSON.stringify(DEFAULT_SECURITY_HEADERS));
      this.updateParent();
    }
  }
};
</script>

<style lang="less" scoped>
.static-site-config {
  .t-form-item {
    margin-bottom: 16px;
  }
}

.security-headers-wrapper {
  width: 100%;
}

.security-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;

  .header-name-input {
    width: 220px;
    flex-shrink: 0;
  }

  .header-value-input {
    flex: 1;
  }
}

.security-headers-empty {
  margin-bottom: 8px;
}

.security-header-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}
</style>