// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { createApp, watchEffect } from 'vue';
import { createHead } from '@unhead/vue/client';
import App from './App.vue';
import router from './router';
import store from './store';
import i18n from './i18n';
import HomeWafUI from './ui';
import bus from './bus/bus';
import axiosInstance from './utils/request';
import globalconfig from './utils/globalconfig';
import { purifyHtml } from './utils/purify';
import { adminTitles, loadAdminTitles } from './utils/adminTitles';
import { setupMessageGuard } from './utils/messageguard';
import HelpBlock from './components/help-block/index.vue';
import IpLookup from './components/ip-lookup/index.vue';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import './style/index.less';
import './ui/theme.css';
import './permission';
async function mountApp() {
  await loadAdminTitles();
  const app = createApp(App);
  app.use(store).use(router).use(i18n).use(createHead()).use(HomeWafUI);
  Object.assign(app.config.globalProperties, {
    $bus: bus,
    $request: axiosInstance,
    $purifyHtml: purifyHtml,
    samwafglobalconfig: globalconfig,
    $set: (object, key, value) => {
      object[key] = value;
    },
    $delete: (object, key) => {
      delete object[key];
    },
    $applyFilter(name, value, ...args) {
      const fn = this.$options.filters?.[name];
      return fn ? fn.call(this, value, ...args) : value;
    },
    async $copyText(text) {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(String(text));
        return;
      }
      const input = document.createElement('textarea');
      input.value = String(text);
      document.body.append(input);
      input.select();
      const copied = document.execCommand('copy');
      input.remove();
      if (!copied) throw new Error('Copy unavailable; select and copy manually.');
    },
  });
  app.component('help-block', HelpBlock).component('ip-lookup', IpLookup);
  setupMessageGuard();
  watchEffect(() => {
    i18n.global.locale.value = store.state.language.currentLanguage;
    document.title = adminTitles.admin_title || 'HomeWaf Web Application Firewall';
    document.documentElement.classList.toggle('dark', store.getters['setting/mode'] === 'dark');
  });
  window.vm = app.mount('#app');
}
mountApp();
