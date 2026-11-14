<template>
  <main class="login-page">
    <section class="login-intro" aria-labelledby="login-product-title">
      <RouterLink to="/help" class="login-brand"><Logo aria-hidden="true" /><span>HomeWaf</span></RouterLink>
      <div class="login-intro-copy">
        <span class="login-category">网站与家庭服务防护</span>
        <h2 id="login-product-title">你的服务，<br />由你掌握。</h2>
        <p>连接现有网站、NAS 和设备。<br />在同一个控制台管理访问、策略与运行记录。</p>
      </div>
      <div class="login-flow" aria-label="请求经过 HomeWaf 到达现有服务">
        <span>访问请求</span><i aria-hidden="true"></i><strong>HomeWaf</strong><i aria-hidden="true"></i
        ><span>你的服务</span>
      </div>
      <span class="login-intro-foot">部署在自己的设备上</span>
    </section>
    <section class="login-form-side">
      <div class="login-top-actions">
        <RouterLink to="/help">使用指南 ↗</RouterLink
        ><ElButton text circle aria-label="界面设置" @click="store.commit('setting/toggleSettingPanel', true)"
          ><Setting
        /></ElButton>
      </div>
      <div class="login-container">
        <span class="login-eyebrow">管理控制台</span>
        <h1>{{ loginTitle }}</h1>
        <p class="login-description">使用此实例的管理员账号登录。</p>
        <Login />
        <p class="login-support">首次使用的初始口令保存在实例的<br /><code>data/initial_password.txt</code> 中。</p>
      </div>
      <footer class="login-footer">
        <span>HomeWaf · 非商业使用免费</span><RouterLink to="/help">使用与许可说明</RouterLink>
      </footer>
    </section>
    <HomeWafSetting />
  </main>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { ElButton } from 'element-plus';
import { Setting } from '@element-plus/icons-vue';
import Login from './components/components-login.vue';
import HomeWafSetting from '@/layouts/setting.vue';
import Logo from '@/assets/assets-t-logo.svg';
import { adminTitles } from '@/utils/adminTitles';
import store from '@/store';
const loginTitle = computed(() => adminTitles.admin_title || '登录 HomeWaf');
</script>
<style lang="less">
@import './index.less';
</style>
