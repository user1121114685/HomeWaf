<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="login-wrapper">
   <!-- <login-header /> -->

    <div class="login-container">
      <div class="title-container">
        <h1 class="title margin-no">{{$t('login.login_title')}}</h1>
        <h1 class="title admin-login-title">{{loginTitle}}</h1>
        <div class="sub-title">
        </div>
      </div>

      <login v-if="type === 'login'" />
      <register v-else @register-success="switchType('login')" />
      <homewaf-setting />
    </div>

    <!-- Upstream UI: Copyright 2022-present SamWaf authors; HomeWaf modifications are separately licensed. -->
    <footer class="copyright">HomeWaf · 非商业使用免费 · 商业使用需授权  <hw-link theme="primary" @click="handleJumpOnlineUrl"> {{$t('login.login_has_question')}} </hw-link>  <a href="#/help" target="_blank" rel="noopener noreferrer">{{$t('login.login_online_document')}} </a></footer>
  </div>
</template>
<script>
import Login from './components/components-login.vue';
import Register from './components/components-register.vue';
import LoginHeader from './components/components-header.vue';
import HomeWafSetting from '@/layouts/setting.vue';
import { adminTitles } from '@/utils/adminTitles';

export default {
  name: 'LoginIndex',
  components: {
    LoginHeader,
    Login,
    Register,
    HomeWafSetting,
  },
  data() {
    return {
      type: 'login',
      copyright:'',
    };
  },
  computed: {
    loginTitle() {
      return adminTitles.admin_title || this.$t('login.login_sub_title');
    },
  },
  methods: {
    switchType(val) {
      this.type = val;
    },
    //跳转界面
    handleJumpOnlineUrl(){
      window.open(this.samwafglobalconfig.getOnlineUrl()+"", '_blank', 'noopener,noreferrer');
    },
  },
};
</script>
<style lang="less">
@import url('./index.less');
</style>
