<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <hw-dialog
    :header="$t('page.account.change_password')"
    v-model:visible="innerVisible"
    :width="560"
    :footer="false"
    :close-btn="!forced && !submitting"
    :close-on-overlay-click="!forced && !submitting"
    :close-on-esc-keydown="!forced && !submitting"
    @close="handleClose"
  >
    <template #body
      ><div class="change-password-content">
        <hw-alert v-if="forced && reason" theme="warning" :message="reason" :style="{ marginBottom: '12px' }" />
        <hw-alert v-if="submitError" theme="error" :message="submitError" :style="{ marginBottom: '12px' }" />
        <!-- 新密码字段使用 new-password，避免浏览器把登录口令填进新密码。 -->
        <hw-form
          :data="formData"
          ref="form"
          :rules="rules"
          @submit="onSubmit"
          :disabled="submitting"
          layout="vertical"
          autocomplete="off"
        >
          <hw-form-item v-if="!hideOld" :label="$t('page.account.old_password')" name="old_password">
            <hw-input
              type="password"
              autocomplete="current-password"
              v-model="formData.old_password"
              :placeholder="$t('common.placeholder') + $t('page.account.old_password')"
            ></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.account.new_password')" name="new_password">
            <hw-input
              type="password"
              autocomplete="new-password"
              v-model="formData.new_password"
              :placeholder="$t('common.placeholder') + $t('page.account.new_password')"
            ></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.account.confirm_password')" name="new_password2">
            <hw-input
              type="password"
              autocomplete="new-password"
              v-model="formData.new_password2"
              :placeholder="$t('common.placeholder') + $t('page.account.confirm_password')"
            ></hw-input>
          </hw-form-item>
          <div class="change-password-actions">
            <hw-button v-if="!forced" variant="outline" :disabled="submitting" @click="handleClose">{{
              $t('common.close')
            }}</hw-button>
            <hw-button theme="primary" type="submit" :loading="submitting" :disabled="submitting">{{
              $t('common.confirm')
            }}</hw-button>
          </div>
        </hw-form>
      </div></template
    >
  </hw-dialog>
</template>
<script lang="ts">
import { defineComponent } from 'vue';

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
      submitting: false,
      submitError: '',
      rules: {
        old_password: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.account.old_password'),
            type: 'error',
          },
        ],
        new_password: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.account.new_password'),
            type: 'error',
          },
        ],
        new_password2: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.account.confirm_password'),
            type: 'error',
          },
        ],
      },
    };
  },
  computed: {
    innerVisible: {
      get(): boolean {
        return this.visible;
      },
      set(val: boolean) {
        this.$emit('update:visible', val);
      },
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
        this.submitError = '';
      }
    },
  },
  methods: {
    handleClose() {
      if (this.forced || this.submitting) return; // 强制改密或提交中不允许关闭
      this.innerVisible = false;
    },
    async onSubmit({ validateResult, firstError }): Promise<void> {
      if (this.submitting) return;
      this.submitError = '';
      if (validateResult !== true) {
        this.submitError = firstError;
        return;
      }
      if (this.formData.new_password !== this.formData.new_password2) {
        this.submitError = this.$t('page.account.password_mismatch_warning');
        return;
      }
      const oldPwd = this.hideOld ? this.presetOldPassword : this.formData.old_password;
      this.submitting = true;
      try {
        const res = await account_change_my_password_api({
          old_password: oldPwd,
          new_password: this.formData.new_password,
          new_password2: this.formData.new_password2,
        });
        if (res.code === 0) {
          this.$message.success(res.msg);
          this.innerVisible = false;
          this.$emit('success');
        } else {
            this.submitError = res.msg || '修改密码失败，请检查连接后重试。';
        }
      } catch {
        this.submitError = '修改密码失败，请检查连接后重试。';
      } finally {
        this.submitting = false;
      }
    },
  },
});
</script>
<style scoped>
.change-password-content {
  min-width: 0;
}
.change-password-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 8px;
}
</style>
