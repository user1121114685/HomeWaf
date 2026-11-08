// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { createPinia, defineStore } from 'pinia';
import { computed } from 'vue';
import user from './modules/user';
import notification from './modules/notification';
import attacklog from './modules/attacklog';
import version from './modules/versioninfo';
import setting from './modules/setting';
import permission from './modules/permission';
import tabRouter from './modules/tab-router';
import language from './modules/language';
import stats from './modules/stats';
import sysparams from './modules/sysparams';
// Pinia owns all state. Stable command names preserve existing page contracts.
export const pinia = createPinia();
const definitions: Record<string, any> = {
  user,
  setting,
  notification,
  attacklog,
  version,
  permission,
  tabRouter,
  language,
  stats,
  sysparams,
};
const useRoot = defineStore('homewaf', {
  state: () =>
    Object.fromEntries(
      Object.entries(definitions).map(([name, m]) => [name, typeof m.state === 'function' ? m.state() : m.state]),
    ),
});
const root = useRoot(pinia);
export type RootState = typeof root.$state;
const getters: Record<string, any> = {};
const localGetters = (ns: string) => new Proxy({}, { get: (_t, key: string) => getters[`${ns}/${key}`] });
const qualify = (ns: string, type: string, options?: any) =>
  options?.root || type.includes('/') ? type : `${ns}/${type}`;
const store = {
  state: root.$state,
  getters,
  commit(type: string, payload?: any) {
    const [ns, name] = type.split('/');
    const fn = definitions[ns]?.mutations?.[name];
    if (!fn) throw new Error(`Unknown mutation: ${type}`);
    fn(root.$state[ns], payload);
  },
  dispatch(type: string, payload?: any): Promise<any> {
    const [ns, name] = type.split('/');
    const fn = definitions[ns]?.actions?.[name];
    if (!fn) return Promise.reject(new Error(`Unknown action: ${type}`));
    try {
      return Promise.resolve(
        fn(
          {
            state: root.$state[ns],
            rootState: root.$state,
            getters: localGetters(ns),
            rootGetters: getters,
            commit: (key: string, value?: any, opts?: any) => store.commit(qualify(ns, key, opts), value),
            dispatch: (key: string, value?: any, opts?: any) => store.dispatch(qualify(ns, key, opts), value),
          },
          payload,
        ),
      );
    } catch (e) {
      return Promise.reject(e);
    }
  },
  install(app: any) {
    app.use(pinia);
    app.config.globalProperties.$store = store;
  },
};
for (const [ns, m] of Object.entries(definitions))
  for (const [name, getter] of Object.entries(m.getters || {})) {
    const value = computed(() => (getter as Function)(root.$state[ns], localGetters(ns), root.$state, getters));
    Object.defineProperty(getters, `${ns}/${name}`, { get: () => value.value, enumerable: true });
  }
export default store;
