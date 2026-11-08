// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import {createI18n} from 'vue-i18n';
import zh_CN from './zh_CN';import en_US from './en_US';
export default createI18n({legacy:false,globalInjection:true,locale:localStorage.getItem('lang')||'zh_CN',fallbackLocale:'zh_CN',messages:{zh_CN,en_US}});
