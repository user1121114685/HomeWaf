<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <!-- 总开关就地可改：它决定整个功能开不开，让用户为了它跑一趟【系统配置】既绕又容易找不到 -->
      <div class="master-bar" :class="{ 'is-on': masterEnabled }">
        <div class="master-left">
          <hw-switch v-model="masterEnabled" :loading="masterLoading" @change="onMasterChange"></hw-switch>
          <span class="master-title">{{ $t('page.access.config.master_switch') }}</span>
          <hw-tag v-if="masterEnabled" theme="success" variant="light">{{ $t('page.access.config.master_on') }}</hw-tag>
          <hw-tag v-else theme="default" variant="light">{{ $t('page.access.config.master_off') }}</hw-tag>
        </div>
        <div class="master-tips">{{ $t('page.access.config.master_switch_tips') }}</div>
        <hw-button variant="outline" size="small" @click="gotoAccount">
          {{ $t('page.access.config.goto_account') }}
        </hw-button>
      </div>

      <hw-form :data="formData" ref="cfgForm" @submit="onSubmit" :labelWidth="180" :style="{ marginTop: '8px' }">
        <hw-tabs v-model="activeTab">

          <!-- ① 基础设置：不配这几项功能就跑不起来 / 一定会踩坑，其余全部收进高级设置 -->
          <hw-tab-panel value="basic" :label="$t('page.access.config.tab_basic')">
            <div class="tab-body">

              <hw-form-item :label="$t('page.access.config.label_center_origin')" name="center_origin" requiredMark>
                <hw-select :style="{ width: 'min(100%, 520px)' }" v-model="formData.center_origin" filterable creatable clearable
                  :placeholder="$t('page.access.config.center_origin_placeholder')"
                  @create="onCreateCenterOrigin">
                  <hw-option v-for="h in centerHostOptions" :key="h.origin" :value="h.origin" :label="h.label"></hw-option>
                </hw-select>
                <div class="form-tips">{{ $t('page.access.config.center_origin_tips') }}</div>
                <hw-alert v-if="centerHostOptions.length === 0" theme="warning" :style="{ width: 'min(100%, 520px)', marginTop: '8px' }"
                  :message="$t('page.access.config.center_origin_empty')"></hw-alert>
              </hw-form-item>

              <hw-form-item :label="$t('page.access.config.label_session_ttl')" name="session_ttl_minutes">
                <hw-input-number :style="{ width: 'min(100%, 220px)' }" v-model="formData.session_ttl_minutes" :min="1" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_minute') }}</span>
                <div class="form-tips">{{ $t('page.access.config.session_ttl_tips') }}</div>
              </hw-form-item>

              <hw-form-item :label="$t('page.access.config.label_require_otp')" name="require_otp">
                <hw-radio-group v-model="formData.require_otp">
                  <hw-radio :value="0">{{ $t('page.access.disable') }}</hw-radio>
                  <hw-radio :value="1">{{ $t('page.access.enable') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.require_otp_tips') }}</div>
              </hw-form-item>

              <hw-form-item :label="$t('page.access.config.label_max_fail')" name="max_fail_count">
                <hw-input-number :style="{ width: '160px' }" v-model="formData.max_fail_count" :min="1" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_times') }}</span>
                <span class="unit">{{ $t('page.access.config.lock_join') }}</span>
                <hw-input-number :style="{ width: '160px' }" v-model="formData.lock_minutes" :min="1" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_minute') }}</span>
                <div class="form-tips">{{ $t('page.access.config.max_fail_tips') }}</div>
              </hw-form-item>

              <hw-form-item :label="$t('page.access.config.label_exclude_paths')" name="global_exclude_paths">
                <hw-textarea :style="{ width: 'min(100%, 520px)' }" v-model="formData.global_exclude_paths"
                  :autosize="{ minRows: 3, maxRows: 8 }" :placeholder="excludePathsPlaceholder"></hw-textarea>
                <div class="form-tips">{{ $t('page.access.config.exclude_paths_tips') }}</div>
              </hw-form-item>

              <hw-form-item :label="$t('page.access.config.label_bypass_ip_group')" name="bypass_ip_group_code">
                <hw-select :style="{ width: 'min(100%, 520px)' }" v-model="formData.bypass_ip_group_code" clearable
                  :placeholder="$t('common.select_placeholder')">
                  <hw-option v-for="g in ipGroups" :key="g.group_code" :value="g.group_code" :label="g.group_name"></hw-option>
                </hw-select>
                <div class="form-tips">{{ $t('page.access.config.bypass_ip_group_tips') }}</div>
              </hw-form-item>

            </div>
          </hw-tab-panel>

          <!-- ② 高级设置：默认值对绝大多数人都是对的，改错了容易把自己挡在外面 -->
          <hw-tab-panel value="advanced" :label="$t('page.access.config.tab_advanced')">
            <div class="tab-body">

              <hw-divider align="left">{{ $t('page.access.config.section_path') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_path_prefix')" name="path_prefix">
                <hw-input :style="{ width: 'min(100%, 520px)' }" v-model="formData.path_prefix" placeholder="/samwaf_access"></hw-input>
                <div class="form-tips">{{ $t('page.access.config.path_prefix_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_cookie_prefix')" name="cookie_prefix">
                <hw-input :style="{ width: 'min(100%, 520px)' }" v-model="formData.cookie_prefix" placeholder="samwaf_ac"></hw-input>
                <div class="form-tips">{{ $t('page.access.config.cookie_prefix_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_force_secure')" name="force_secure_cookie">
                <hw-radio-group v-model="formData.force_secure_cookie">
                  <hw-radio :value="0">{{ $t('page.access.config.secure_auto') }}</hw-radio>
                  <hw-radio :value="1">{{ $t('page.access.config.secure_force') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.force_secure_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_hmac_secret')">
                <hw-tag v-if="hasHmacSecret" theme="success" variant="light">{{ $t('page.access.config.secret_set') }}</hw-tag>
                <hw-tag v-else theme="warning" variant="light">{{ $t('page.access.config.secret_unset') }}</hw-tag>
                <hw-button variant="outline" size="small" :style="{ marginLeft: '12px' }"
                  @click="regenerateVisible = true">{{ $t('page.access.config.button_regenerate') }}</hw-button>
                <div class="form-tips">{{ $t('page.access.config.hmac_secret_tips') }}</div>
              </hw-form-item>

              <hw-divider align="left">{{ $t('page.access.config.section_ttl') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_token_ttl')" name="token_ttl_minutes">
                <hw-input-number :style="{ width: 'min(100%, 220px)' }" v-model="formData.token_ttl_minutes" :min="1" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_minute') }}</span>
                <div class="form-tips">{{ $t('page.access.config.token_ttl_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_ticket_ttl')" name="ticket_ttl_seconds">
                <hw-input-number :style="{ width: 'min(100%, 220px)' }" v-model="formData.ticket_ttl_seconds" :min="1" :max="300" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_second') }}</span>
                <div class="form-tips">{{ $t('page.access.config.ticket_ttl_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_idle_timeout')" name="idle_timeout_minutes">
                <hw-input-number :style="{ width: 'min(100%, 220px)' }" v-model="formData.idle_timeout_minutes" :min="0" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_minute') }}</span>
                <div class="form-tips">{{ $t('page.access.config.idle_timeout_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_cache_ttl')" name="cache_positive_ttl_sec">
                <hw-input-number :style="{ width: 'min(100%, 220px)' }" v-model="formData.cache_positive_ttl_sec" :min="1" :max="60" theme="column"></hw-input-number>
                <span class="unit">{{ $t('page.access.unit_second') }}</span>
                <div class="form-tips">{{ $t('page.access.config.cache_ttl_tips') }}</div>
              </hw-form-item>

              <hw-divider align="left">{{ $t('page.access.config.section_bind') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_bind_ip')" name="bind_ip">
                <hw-radio-group v-model="formData.bind_ip">
                  <hw-radio :value="0">{{ $t('page.access.disable') }}</hw-radio>
                  <hw-radio :value="1">{{ $t('page.access.enable') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.bind_ip_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_bind_fingerprint')" name="bind_fingerprint">
                <hw-radio-group v-model="formData.bind_fingerprint">
                  <hw-radio :value="0">{{ $t('page.access.disable') }}</hw-radio>
                  <hw-radio :value="1">{{ $t('page.access.enable') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.bind_fingerprint_tips') }}</div>
              </hw-form-item>

              <hw-divider align="left">{{ $t('page.access.config.section_bypass') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_service_token_header')" name="service_token_header">
                <hw-input :style="{ width: 'min(100%, 520px)' }" v-model="formData.service_token_header"
                  placeholder="X-Service-Token"></hw-input>
                <div class="form-tips">{{ $t('page.access.config.service_token_header_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_service_tokens')" name="service_tokens">
                <hw-textarea :style="{ width: 'min(100%, 520px)' }" v-model="formData.service_tokens"
                  :autosize="{ minRows: 2, maxRows: 6 }"
                  :placeholder="hasServiceToken ? $t('page.access.config.service_tokens_set_placeholder') : ''"></hw-textarea>
                <div class="form-tips">{{ $t('page.access.config.service_tokens_tips') }}</div>
              </hw-form-item>

              <hw-divider align="left">{{ $t('page.access.config.section_cors') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_cors_allow_origins')" name="cors_allow_origins">
                <hw-textarea :style="{ width: 'min(100%, 520px)' }" v-model="formData.cors_allow_origins"
                  :autosize="{ minRows: 2, maxRows: 6 }"
                  placeholder="https://app.example.com&#10;https://admin.example.com:8443"></hw-textarea>
                <div class="form-tips">{{ $t('page.access.config.cors_allow_origins_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_cors_allow_methods')" name="cors_allow_methods">
                <hw-input :style="{ width: 'min(100%, 520px)' }" v-model="formData.cors_allow_methods"
                  placeholder="GET,POST,PUT,PATCH,DELETE,OPTIONS"></hw-input>
                <div class="form-tips">{{ $t('page.access.config.cors_allow_methods_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_cors_allow_headers')" name="cors_allow_headers">
                <hw-input :style="{ width: 'min(100%, 520px)' }" v-model="formData.cors_allow_headers"
                  placeholder="Content-Type,Authorization,X-Requested-With"></hw-input>
                <div class="form-tips">{{ $t('page.access.config.cors_allow_headers_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_cors_max_age')" name="cors_max_age">
                <hw-input-number :style="{ width: 'min(100%, 200px)' }" v-model="formData.cors_max_age"
                  theme="column" :min="0" :max="7200"></hw-input-number>
                <div class="form-tips">{{ $t('page.access.config.cors_max_age_tips') }}</div>
              </hw-form-item>

              <hw-divider align="left">{{ $t('page.access.config.section_behavior') }}</hw-divider>

              <hw-form-item :label="$t('page.access.config.label_unauth_action')" name="unauth_action">
                <hw-radio-group v-model="formData.unauth_action">
                  <hw-radio value="auto">{{ $t('page.access.unauth_auto') }}</hw-radio>
                  <hw-radio value="redirect">{{ $t('page.access.unauth_redirect') }}</hw-radio>
                  <hw-radio value="401">{{ $t('page.access.unauth_401') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.unauth_action_tips') }}</div>
              </hw-form-item>
              <hw-form-item :label="$t('page.access.config.label_pass_identity')" name="pass_identity_header">
                <hw-radio-group v-model="formData.pass_identity_header">
                  <hw-radio :value="0">{{ $t('page.access.disable') }}</hw-radio>
                  <hw-radio :value="1">{{ $t('page.access.enable') }}</hw-radio>
                </hw-radio-group>
                <div class="form-tips">{{ $t('page.access.config.pass_identity_tips') }}</div>
              </hw-form-item>

            </div>
          </hw-tab-panel>
        </hw-tabs>

        <!-- 保存按钮放在 tabs 外面：提交的是整个 formData，两个 tab 的改动一次保存 -->
        <div class="form-footer">
          <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          <hw-button variant="outline" :style="{ marginLeft: '12px' }" @click="getDetail">{{ $t('common.reset') }}</hw-button>
        </div>
      </hw-form>
    </hw-card>

    <hw-dialog :header="$t('page.access.config.button_regenerate')"
      :body="$t('page.access.config.regenerate_confirm')"
      v-model:visible="regenerateVisible" @confirm="onRegenerate"></hw-dialog>

    <!-- 开总开关是唯一能把整站锁住的动作，必须确认；关掉是止血动作，不拦 -->
    <hw-dialog :header="$t('page.access.config.master_switch')" v-model:visible="enableConfirmVisible"
      @confirm="onConfirmEnable" @close="onCancelEnable">
      <template #body><div >
        <p>{{ $t('page.access.config.master_enable_confirm') }}</p>
        <hw-alert v-if="!formData.center_origin" theme="error" :style="{ marginTop: '12px' }"
          :message="$t('page.access.config.master_enable_no_center')"></hw-alert>
      </div></template>
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { prefix } from '@/config/global';
import {
  wafAccessConfigDetailApi, wafAccessConfigSaveApi,
  wafAccessConfigRegenerateSecretApi, wafAccessConfigHostOptionsApi,
} from '@/apis/access';
import { wafIPGroupOptionsApi } from '@/apis/ipgroup';
import { get_detail_by_item_api, edit_system_config_by_item_api } from '@/apis/systemconfig';

// 总开关存在 system_config 的 access_enable 里，不在本页的 access_config 表。
// 走 editByItem 接口改，后端改完会顺手调 TaskLoadSetting 重载并发布运行时快照。
const MASTER_ITEM = 'access_enable';

const INITIAL_CONFIG = {
  center_origin: '',
  path_prefix: '/samwaf_access',
  cookie_prefix: 'samwaf_ac',
  session_ttl_minutes: 720,
  token_ttl_minutes: 720,
  ticket_ttl_seconds: 60,
  idle_timeout_minutes: 0,
  bind_ip: 0,
  bind_fingerprint: 0,
  require_otp: 0,
  max_fail_count: 10,
  lock_minutes: 3,
  global_exclude_paths: '',
  bypass_ip_group_code: '',
  service_token_header: '',
  // 服务令牌明文只在提交时传，后端只存 sha256；留空表示保持原样不动
  service_tokens: '',
  // 跨源(CORS)：全空即不启用，存量部署行为不变
  cors_allow_origins: '',
  cors_allow_methods: '',
  cors_allow_headers: '',
  cors_max_age: 0,
  unauth_action: 'auto',
  pass_identity_header: 0,
  force_secure_cookie: 0,
  cache_positive_ttl_sec: 60,
};

export default defineComponent({
  name: 'WafAccessConfig',
  data() {
    return {
      prefix,
      activeTab: 'basic',
      // 换行只能从 JS 传：写在模板的 placeholder 属性里会被当成字面量 &#10; 显示出来
      excludePathsPlaceholder: '/healthz\n/api/webhook',
      formData: { ...INITIAL_CONFIG },
      hasHmacSecret: false,
      hasServiceToken: false,
      regenerateVisible: false,
      ipGroups: [],
      centerHostOptions: [],
      masterEnabled: false,
      masterLoading: false,
      enableConfirmVisible: false,
    };
  },
  mounted() {
    this.getDetail();
    this.getIpGroups();
    this.getCenterHostOptions();
    this.getMasterSwitch();
  },
  methods: {
    getDetail() {
      const that = this;
      wafAccessConfigDetailApi().then((res) => {
        if (res.code === 0 && res.data) {
          const d = res.data;
          that.formData = {
            ...INITIAL_CONFIG,
            center_origin: d.center_origin ?? '',
            path_prefix: d.path_prefix || INITIAL_CONFIG.path_prefix,
            cookie_prefix: d.cookie_prefix || INITIAL_CONFIG.cookie_prefix,
            session_ttl_minutes: d.session_ttl_minutes || INITIAL_CONFIG.session_ttl_minutes,
            token_ttl_minutes: d.token_ttl_minutes || INITIAL_CONFIG.token_ttl_minutes,
            ticket_ttl_seconds: d.ticket_ttl_seconds || INITIAL_CONFIG.ticket_ttl_seconds,
            idle_timeout_minutes: d.idle_timeout_minutes ?? 0,
            bind_ip: d.bind_ip ?? 0,
            bind_fingerprint: d.bind_fingerprint ?? 0,
            require_otp: d.require_otp ?? 0,
            max_fail_count: d.max_fail_count || INITIAL_CONFIG.max_fail_count,
            lock_minutes: d.lock_minutes || INITIAL_CONFIG.lock_minutes,
            global_exclude_paths: d.global_exclude_paths ?? '',
            bypass_ip_group_code: d.bypass_ip_group_code ?? '',
            service_token_header: d.service_token_header ?? '',
            service_tokens: '',
            cors_allow_origins: d.cors_allow_origins ?? '',
            cors_allow_methods: d.cors_allow_methods ?? '',
            cors_allow_headers: d.cors_allow_headers ?? '',
            cors_max_age: d.cors_max_age ?? 0,
            unauth_action: d.unauth_action || 'auto',
            pass_identity_header: d.pass_identity_header ?? 0,
            force_secure_cookie: d.force_secure_cookie ?? 0,
            cache_positive_ttl_sec: d.cache_positive_ttl_sec || INITIAL_CONFIG.cache_positive_ttl_sec,
          };
          // 密钥类字段后端不回显，只给"是否已设置"的标志位
          that.hasHmacSecret = d.has_hmac_secret === true;
          that.hasServiceToken = d.has_service_token === true;
        }
      }).catch((e) => console.log(e));
    },
    gotoAccount() {
      this.$router.push({ name: 'WafAccessAccount' });
    },
    getMasterSwitch() {
      const that = this;
      get_detail_by_item_api({ item: MASTER_ITEM }).then((res) => {
        if (res.code === 0 && res.data) {
          that.masterEnabled = String(res.data.value) === '1';
        }
      }).catch(() => { /* 读不到就当关闭，用户一点开关会立刻收到真实结果 */ });
    },
    // 开总开关是唯一能把整站锁住的动作，先弹确认；关掉是止血动作，立即生效不打断
    onMasterChange(v) {
      if (v) {
        this.enableConfirmVisible = true;
        return;
      }
      this.applyMaster(false);
    },
    onConfirmEnable() {
      this.enableConfirmVisible = false;
      this.applyMaster(true);
    },
    // 取消要把开关拨回去，否则界面显示"开"而库里是"关"
    onCancelEnable() {
      this.enableConfirmVisible = false;
      this.masterEnabled = false;
    },
    applyMaster(on) {
      const that = this;
      this.masterLoading = true;
      edit_system_config_by_item_api({ item: MASTER_ITEM, value: on ? '1' : '0' }).then((res) => {
        if (res.code === 0) {
          that.masterEnabled = on;
          that.$message.success(on
            ? that.$t('page.access.config.master_on_ok')
            : that.$t('page.access.config.master_off_ok'));
        } else {
          that.$message.warning(res.msg);
          that.getMasterSwitch();
        }
      }).catch(() => {
        that.getMasterSwitch();
      }).finally(() => {
        that.masterLoading = false;
      });
    },
    getIpGroups() {
      const that = this;
      wafIPGroupOptionsApi().then((res) => {
        if (res.code === 0) {
          that.ipGroups = res.data ?? [];
        }
      }).catch(() => { /* IP组接口不可用时不影响本页其余配置 */ });
    },
    getCenterHostOptions() {
      const that = this;
      wafAccessConfigHostOptionsApi().then((res) => {
        if (res.code === 0) {
          that.centerHostOptions = res.data ?? [];
        }
      }).catch(() => { /* 取不到候选不影响手填 */ });
    },
    // 允许填一个不在候选里的地址（比如刚加完站点还没刷新页面），合法性由后端把关
    onCreateCenterOrigin(v) {
      const val = String(v || '').trim();
      if (!val) return;
      if (!this.centerHostOptions.some((h) => h.origin === val)) {
        this.centerHostOptions = [...this.centerHostOptions, { origin: val, label: val, host_code: '' }];
      }
      this.formData.center_origin = val;
    },
    onSubmit({ firstError }) {
      if (firstError) {
        this.$message.warning(firstError);
        return;
      }
      // 认证中心域名是必填：整个功能就是"先跳到它登录"，没有它就无处可跳
      if (!String(this.formData.center_origin || '').trim()) {
        this.activeTab = 'basic';
        this.$message.warning(this.$t('page.access.config.center_origin_required'));
        return;
      }
      const that = this;
      wafAccessConfigSaveApi({ ...this.formData }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.getDetail();
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    onRegenerate() {
      const that = this;
      wafAccessConfigRegenerateSecretApi().then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.getDetail();
        } else {
          that.$message.warning(res.msg);
        }
        that.regenerateVisible = false;
      });
    },
  },
});
</script>

<style lang="less" scoped>
@import '@/style/variables';

.list-card-container {
  padding: @spacer-2 @spacer-3;
}

.master-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px 16px;
  border-radius: var(--td-radius-medium);
  border: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container-hover);

  &.is-on {
    border-color: var(--td-success-color-3);
    background: var(--td-success-color-1);
  }
}

.master-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.master-title {
  font-weight: 500;
}

.master-tips {
  flex: 1;
  min-width: 260px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 20px;
}

.tab-body {
  padding: @spacer-2 0 0 0;
}

.form-footer {
  padding: @spacer-2 0 0 0;
  border-top: 1px solid var(--td-component-stroke);
  margin-top: @spacer-2;
}

.form-tips {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
  line-height: 20px;
  margin-top: 4px;
  // t-form-item 的控件区是 flex 容器，光靠 div 是块级元素并不会换行——
  // 说明文字会被挤到输入框右边一路溢出去。占满一行强制它换到下一行。
  flex-basis: 100%;
  width: 100%;
  max-width: 640px;
}

.unit {
  margin-left: 8px;
  color: var(--td-text-color-secondary);
}
</style>
