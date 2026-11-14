<template>
  <div class="account-overview">
    <section class="account-summary">
      <User class="account-avatar" aria-hidden="true" />
      <div>
        <h2>{{ account || '当前管理账号' }}</h2>
        <p>当前浏览器已登录此 HomeWaf 实例。</p>
      </div>
    </section>
    <section class="account-security">
      <h2>登录安全</h2>
      <div class="account-security-row">
        <div>
          <h3>登录密码</h3>
          <p>修改当前账号密码。请使用独立于其他服务的密码。</p>
        </div>
        <ElButton @click="changePassword = true">修改密码</ElButton>
      </div>
      <div class="account-security-row">
        <div>
          <h3>双因素认证</h3>
          <p>为管理登录增加验证码验证。</p>
        </div>
        <RouterLink to="/account/OTP">管理双因素认证<ArrowRight /></RouterLink>
      </div>
      <div class="account-security-row">
        <div>
          <h3>登录记录</h3>
          <p>核对登录来源与认证结果。</p>
        </div>
        <RouterLink to="/account/LoginHistory">查看登录记录<ArrowRight /></RouterLink>
      </div>
    </section>
    <ChangePasswordDialog v-model:visible="changePassword" :forced="false" />
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElButton } from 'element-plus';
import { ArrowRight, User } from '@element-plus/icons-vue';
import store from '@/store';
import ChangePasswordDialog from '@/pages/waf/account/components/ChangePasswordDialog.vue';
const account = computed(() => store.state.user.userInfo.name || '');
const changePassword = ref(false);
</script>
<style scoped>
.account-overview {
  display: grid;
  grid-template-columns: minmax(240px, 0.8fr) minmax(0, 2fr);
  gap: 24px;
  align-items: start;
}
.account-summary,
.account-security {
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-lg);
  background: var(--hw-panel);
  padding: 24px;
}
.account-avatar {
  width: 48px;
  height: 48px;
  padding: 12px;
  background: var(--hw-subtle);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-md);
  color: var(--hw-muted);
  margin-bottom: 20px;
}
h2 {
  font-size: 18px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
h3 {
  font-size: 14px;
  font-weight: 500;
}
p {
  color: var(--hw-muted);
  font-size: 14px;
  line-height: 1.8;
  margin-top: 8px;
}
.account-security-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 24px 0;
  border-top: 1px solid var(--hw-line);
}
.account-security-row:first-of-type {
  margin-top: 24px;
}
.account-security-row:last-child {
  padding-bottom: 0;
}
.account-security-row a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
  min-height: 44px;
}
.account-security-row svg {
  width: 14px;
}
@media (max-width: 1023px) {
  .account-overview {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 760px) {
  .account-security-row {
    flex-wrap: wrap;
    gap: 16px;
  }
  .account-summary,
  .account-security {
    padding: 20px 16px;
  }
}
</style>
