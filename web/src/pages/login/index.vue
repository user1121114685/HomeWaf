<template>
  <main class="login-page">
    <section class="login-intro" aria-labelledby="login-product-title">
      <RouterLink to="/help" class="login-brand"><Logo aria-hidden="true" /><span>HomeWaf</span></RouterLink>
      <div class="login-product">
        <div class="login-intro-copy">
          <span class="login-category">家庭服务与网站防护</span>
          <h2 id="login-product-title">
            <span class="login-title-first">自己的服务，</span><span>清楚掌握每次访问。</span>
          </h2>
          <p class="login-lead">接入服务、调整防护、查看访问记录。</p>
          <p class="login-detail">从 NAS 到个人网站，<br class="login-detail-break" />守好家庭服务的访问入口。</p>
          <div class="login-license">
            <CircleCheck aria-hidden="true" /><span>家庭非商业使用<strong>免费</strong></span>
          </div>
        </div>
        <div class="login-flow" aria-label="访问请求经过 HomeWaf 防护后到达家庭服务">
          <div class="login-flow-end"><span>访问请求</span><small>家人 · 朋友 · 自己</small></div>
          <ArrowRight class="login-flow-arrow" aria-hidden="true" />
          <div class="login-flow-gate"><Logo aria-hidden="true" /><strong>HomeWaf</strong></div>
          <ArrowRight class="login-flow-arrow" aria-hidden="true" />
          <div class="login-flow-end"><span>家庭服务</span><small>NAS · 网站 · 设备</small></div>
        </div>
      </div>
      <div class="login-intro-foot"><span>部署在自己的设备上</span><span>为家庭服务而设计</span></div>
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
import { ArrowRight, CircleCheck, Setting } from '@element-plus/icons-vue';
import Login from './components/components-login.vue';
import HomeWafSetting from '@/layouts/setting.vue';
import Logo from '@/assets/homewaf-logo.svg';
import { adminTitles } from '@/utils/adminTitles';
import store from '@/store';
const loginTitle = computed(() => adminTitles.admin_title || '登录 HomeWaf');
</script>
<style lang="less">
@import './index.less';
</style>
