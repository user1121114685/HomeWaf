<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="access-config">
    <hw-alert theme="info" :message="$t('page.host.access.intro')" style="margin-bottom: 16px;" />
    <hw-form-item :label="$t('page.host.access.mode')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.mode_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-radio-group v-model="local.mode" @change="updateParent">
          <hw-radio value="0">{{ $t('page.host.access.mode_inherit') }}</hw-radio>
          <hw-radio value="1">{{ $t('page.host.access.mode_force_on') }}</hw-radio>
          <hw-radio value="2">{{ $t('page.host.access.mode_force_off') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.exclude_paths')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.exclude_paths_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-textarea v-model="local.exclude_paths" :placeholder="$t('page.host.access.exclude_paths_ph')"
                    :autosize="{ minRows: 3, maxRows: 6 }" :style="{ width: '480px' }" @blur="updateParent" />
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.allow_ip_group')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.allow_ip_group_tips')" placement="top"
                 :overlay-style="{ width: '360px' }" show-arrow>
        <hw-select v-model="local.allow_ip_group_code" clearable :style="{ width: '480px' }"
                  :placeholder="$t('common.select_placeholder')" @change="updateParent">
          <hw-option v-for="g in ipGroups" :key="g.group_code" :value="g.group_code" :label="g.group_name"></hw-option>
        </hw-select>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.require_otp')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.require_otp_tips')" placement="top"
                 :overlay-style="{ width: '360px' }" show-arrow>
        <hw-radio-group v-model="local.require_otp" @change="updateParent">
          <hw-radio value="0">{{ $t('page.host.access.mode_inherit') }}</hw-radio>
          <hw-radio value="1">{{ $t('page.host.access.otp_force') }}</hw-radio>
          <hw-radio value="2">{{ $t('page.host.access.otp_exempt') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.unauth_action')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.unauth_action_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-radio-group v-model="local.unauth_action" @change="updateParent">
          <hw-radio value="">{{ $t('page.host.access.mode_inherit') }}</hw-radio>
          <hw-radio value="auto">{{ $t('page.host.access.unauth_auto') }}</hw-radio>
          <hw-radio value="redirect">{{ $t('page.host.access.unauth_redirect') }}</hw-radio>
          <hw-radio value="401">{{ $t('page.host.access.unauth_401') }}</hw-radio>
        </hw-radio-group>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.cors_allow_origins')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.cors_allow_origins_tips')" placement="top"
                 :overlay-style="{ width: '420px' }" show-arrow>
        <hw-textarea v-model="local.cors_allow_origins" :style="{ width: '480px' }"
                    :autosize="{ minRows: 2, maxRows: 5 }" @change="updateParent"
                    placeholder="https://app.example.com"></hw-textarea>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.cors_allow_methods')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.cors_allow_methods_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-input v-model="local.cors_allow_methods" :style="{ width: '480px' }" @change="updateParent"
                 placeholder="GET,POST,PUT,PATCH,DELETE,OPTIONS"></hw-input>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.cors_allow_headers')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.cors_allow_headers_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-input v-model="local.cors_allow_headers" :style="{ width: '480px' }" @change="updateParent"
                 placeholder="Content-Type,Authorization"></hw-input>
      </hw-tooltip>
    </hw-form-item>
    <hw-form-item :label="$t('page.host.access.cors_max_age')">
      <hw-tooltip class="placement top center" :content="$t('page.host.access.cors_max_age_tips')" placement="top"
                 :overlay-style="{ width: '380px' }" show-arrow>
        <hw-input-number v-model="local.cors_max_age" :style="{ width: '200px' }" theme="column"
                        :min="0" :max="7200" @change="updateParent"></hw-input-number>
      </hw-tooltip>
    </hw-form-item>

    <hw-alert v-if="cacheConflict" theme="error" :message="$t('page.host.access.cache_conflict')"
             style="margin-top: 16px;" />
  </div>
</template>

<script lang="ts">
import { wafIPGroupOptionsApi } from '@/apis/ipgroup';

export default {
  name: 'AccessConfig',
  props: {
    accessConfig: {
      type: Object,
      required: true
    },
    // 站点是否同时开了 web 缓存。开着的话必须显式警告：
    // 缓存键不含身份，A 用户的私有页面可能被缓存后返回给 B 用户 —— 认证挡住了未登录的人，
    // 却挡不住登录用户之间的串号。
    cacheEnabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      local: JSON.parse(JSON.stringify(this.accessConfig)),
      ipGroups: []
    };
  },
  computed: {
    cacheConflict() {
      return this.cacheEnabled && this.local.mode !== '2';
    }
  },
  watch: {
    accessConfig: {
      handler(newVal) {
        this.local = JSON.parse(JSON.stringify(newVal));
      },
      deep: true
    }
  },
  mounted() {
    const that = this;
    wafIPGroupOptionsApi().then((res) => {
      if (res.code === 0) {
        that.ipGroups = res.data ?? [];
      }
    }).catch(() => { /* IP组接口不可用时不影响本页其余配置 */ });
  },
  methods: {
    updateParent() {
      this.$emit('update', { ...this.local });
    }
  }
};
</script>

<style lang="less" scoped>
.access-config {
  padding: 8px 0;
}
</style>
