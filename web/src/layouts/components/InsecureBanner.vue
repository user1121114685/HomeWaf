<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-alert
    v-if="visible"
    theme="warning"
    class="insecure-banner"
    :close="$t('page.vpconfig.insecure_banner_dismiss')"
    @close="dismiss"
  >
    <template #message>
      <span>{{ $t('page.vpconfig.insecure_banner_message') }}</span>
      <hw-link theme="primary" hover="color" style="margin-left: 8px" @click="goSslSetting">
        {{ $t('page.vpconfig.insecure_banner_action') }}
      </hw-link>
    </template>
  </hw-alert>
</template>

<script>
import {defineComponent} from 'vue';


import { isLoopbackHost, INSECURE_BANNER_DISMISS_KEY } from '@/utils/insecure';

/**
 * 管理端以 HTTP 访问时的常驻提示条。
 *
 * 语义（计划 §十六.2，D9/D10）：
 *  - 回环地址豁免：localhost / 127.0.0.0/8 / ::1 与 HTTPS 同属可信来源（W3C Secure Contexts），
 *    浏览器自己都不标"不安全"，这里再挂一条常驻警告纯属骚扰；
 *  - 不提供永久关闭：只能本次会话收起，下次登录重现。真正"不消失"的那份状态挂在
 *    系统配置的 SSL 卡片里（那处不可关闭），所以收起横幅不等于信息丢失。
 */
export default defineComponent({
  name: 'InsecureBanner',
  data() {
    return {
      visible: false,
    };
  },
  created() {
    this.visible = this.shouldShow();
  },
  methods: {
    shouldShow() {
      if (typeof window === 'undefined') return false;
      if (window.location.protocol === 'https:') return false;
      if (isLoopbackHost(window.location.hostname)) return false;
      try {
        return sessionStorage.getItem(INSECURE_BANNER_DISMISS_KEY) !== '1';
      } catch {
        // 读不到 storage（隐私模式等）就照常显示，宁可多提示一次
        return true;
      }
    },
    dismiss() {
      this.visible = false;
      try {
        sessionStorage.setItem(INSECURE_BANNER_DISMISS_KEY, '1');
      } catch {
        // 存不下就下次刷新再显示，无妨
      }
    },
    goSslSetting() {
      // 带 section 参数，让管理配置页落地就停在「管理端访问与证书」那一节，
      // 不用用户到了页面再自己找——这一节在长页中部，是本条提示唯一的落点
      this.$router.push({ path: '/sys/VpConfig', query: { section: 'access' } }).catch(() => {});
    },
  },
});
</script>

<style scoped>
.insecure-banner {
  margin: 0;
  border-radius: 0;
}
</style>
