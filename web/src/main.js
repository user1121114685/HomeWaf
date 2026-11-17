// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

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
import PageActions from './ui/PageActions.vue';
import EditorSection from './ui/EditorSection.vue';
import EditorTabs from './ui/EditorTabs.vue';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';
import './style/index.less';
import './ui/theme.css';
import './ui/tokens.css';
import './ui/console.css';
import './permission';
async function mountApp() {
  await loadAdminTitles();
  const app = createApp(App);
  app.use(store).use(router).use(i18n).use(createHead()).use(HomeWafUI);
  Object.assign(app.config.globalProperties, {
    $bus: bus,
    $request: axiosInstance,
    $purifyHtml: purifyHtml,
    homewafglobalconfig: globalconfig,
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
  app
    .component('help-block', HelpBlock)
    .component('ip-lookup', IpLookup)
    .component('PageActions', PageActions)
    .component('EditorSection', EditorSection)
    .component('EditorTabs', EditorTabs);
  setupMessageGuard();
  const systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
  const updateSystemTheme = (event) => {
    store.commit('setting/setSystemMode', event.matches ? 'dark' : 'light');
    if (store.state.setting.mode === 'auto') store.dispatch('setting/changeMode', { ...store.state.setting });
  };
  updateSystemTheme(systemTheme);
  systemTheme.addEventListener('change', updateSystemTheme);
  if (import.meta.hot) import.meta.hot.dispose(() => systemTheme.removeEventListener('change', updateSystemTheme));
  watchEffect(() => {
    i18n.global.locale.value = store.state.language.currentLanguage;
    document.documentElement.lang = store.state.language.currentLanguage === 'en_US' ? 'en' : 'zh-CN';
    document.title = adminTitles.admin_title || 'HomeWaf Web Application Firewall';
    document.documentElement.classList.toggle('dark', store.getters['setting/mode'] === 'dark');
  });
  window.vm = app.mount('#app');
}
mountApp();
