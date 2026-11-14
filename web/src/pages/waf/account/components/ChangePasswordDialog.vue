<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-dialog
    :header="$t('page.account.change_password')"
    v-model:visible="innerVisible"
    :width="560"
    :footer="false"
    :close-btn="!forced"
    :close-on-overlay-click="!forced"
    :close-on-esc-keydown="!forced"
    @close="handleClose"
  >
    <template #body><div >
      <hw-alert v-if="forced && reason" theme="warning" :message="reason" :style="{ marginBottom: '12px' }" />
      <!-- autocomplete 关掉：否则浏览器会把保存的登录口令自动填进三个密码框 -->
      <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="120" autocomplete="off">
        <hw-form-item v-if="!hideOld" :label="$t('page.account.old_password')" name="old_password">
          <hw-input :style="{ width: 'min(100%, 380px)' }" type="password" autocomplete="new-password" v-model="formData.old_password"
                   :placeholder="$t('common.placeholder') + $t('page.account.old_password')"></hw-input>
        </hw-form-item>
        <hw-form-item :label="$t('page.account.new_password')" name="new_password">
          <hw-input :style="{ width: 'min(100%, 380px)' }" type="password" autocomplete="new-password" v-model="formData.new_password"
                   :placeholder="$t('common.placeholder') + $t('page.account.new_password')"></hw-input>
        </hw-form-item>
        <hw-form-item :label="$t('page.account.confirm_password')" name="new_password2">
          <hw-input :style="{ width: 'min(100%, 380px)' }" type="password" autocomplete="new-password" v-model="formData.new_password2"
                   :placeholder="$t('common.placeholder') + $t('page.account.confirm_password')"></hw-input>
        </hw-form-item>
        <hw-form-item style="float: right">
          <hw-button v-if="!forced" variant="outline" @click="handleClose">{{ $t('common.close') }}</hw-button>
          <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
        </hw-form-item>
      </hw-form>
    </div></template>
  </hw-dialog>
</template>
<script lang="ts">
import {defineComponent} from 'vue';


import { account_change_my_password_api } from '@/apis/account';

export default defineComponent({
  name: 'ChangePasswordDialog',
  props: {
    visible: { type: Boolean, default: false },
    // forced=true 用于首次登录/口令到期强制改密：不可关闭
    forced: { type: Boolean, default: false },
    reason: { type: String, default: '' },
    // 强制改密场景下，用户刚刚登录用的密码即旧密码，可预置后隐藏旧密码输入
    presetOldPassword: { type: String, default: '' },
  },
  data() {
    return {
      formData: { old_password: '', new_password: '', new_password2: '' },
      rules: {
        old_password: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.account.old_password'), type: 'error' }],
        new_password: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.account.new_password'), type: 'error' }],
        new_password2: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.account.confirm_password'), type: 'error' }],
      },
    };
  },
  computed: {
    innerVisible: {
      get(): boolean { return this.visible; },
      set(val: boolean) { this.$emit('update:visible', val); },
    },
    hideOld(): boolean {
      return this.forced && !!this.presetOldPassword;
    },
  },
  watch: {
    visible(val: boolean) {
      if (val) {
        // 每次打开清空输入
        this.formData = { old_password: '', new_password: '', new_password2: '' };
      }
    },
  },
  methods: {
    handleClose() {
      if (this.forced) return; // 强制改密不允许关闭
      this.innerVisible = false;
    },
    onSubmit({ validateResult, firstError }): void {
      if (validateResult !== true) {
        this.$message.warning(firstError);
        return;
      }
      if (this.formData.new_password !== this.formData.new_password2) {
        this.$message.warning(this.$t('page.account.password_mismatch_warning'));
        return;
      }
      const oldPwd = this.hideOld ? this.presetOldPassword : this.formData.old_password;
      account_change_my_password_api({
        old_password: oldPwd,
        new_password: this.formData.new_password,
        new_password2: this.formData.new_password2,
      })
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
            this.innerVisible = false;
            this.$emit('success');
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch((e: Error) => {
          console.log(e);
        });
    },
  },
});
</script>
