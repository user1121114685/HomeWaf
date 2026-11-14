import STYLE_CONFIG from '@/config/style';
import { COLOR_TOKEN, LIGHT_CHART_COLORS, DARK_CHART_COLORS } from '@/config/color';

export const PREFERENCE_KEY = 'homewaf.console.preferences';
function savedMode() {
  try {
    const mode = JSON.parse(localStorage.getItem(PREFERENCE_KEY) || '{}').mode;
    return ['light', 'dark', 'auto'].includes(mode) ? mode : 'auto';
  } catch {
    return 'auto';
  }
}
const state = {
  ...STYLE_CONFIG,
  mode: savedMode(),
  systemMode: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
  showSettingPanel: false,
  colorList: COLOR_TOKEN,
  chartColors: LIGHT_CHART_COLORS,
};
export type IStateType = typeof state;
const mutations = {
  setSystemMode(state: IStateType, mode: string) {
    state.systemMode = mode;
  },
  update(state: IStateType, payload: Partial<IStateType>) {
    if (['light', 'dark', 'auto'].includes(payload.mode)) state.mode = payload.mode;
  },
  toggleSidebarCompact(state: IStateType) {
    state.isSidebarCompact = !state.isSidebarCompact;
  },
  showSidebarCompact(state: IStateType, value: boolean) {
    state.isSidebarCompact = value;
  },
  toggleSettingPanel(state: IStateType, value: boolean) {
    state.showSettingPanel = value;
  },
  changeChartColor(state: IStateType, value: Record<string, string>) {
    state.chartColors = value;
  },
};
const getters = {
  showHeader: () => true,
  showSidebar: () => true,
  showSidebarLogo: () => true,
  showHeaderLogo: () => false,
  showFooter: () => true,
  isUseTabsRouter: () => false,
  mode: (state: IStateType) => (state.mode === 'auto' ? state.systemMode : state.mode),
};
const actions = {
  changeTheme({ commit, dispatch }, payload: Partial<IStateType>) {
    commit('update', payload);
    try {
      localStorage.setItem(PREFERENCE_KEY, JSON.stringify({ mode: payload.mode }));
    } catch {
      /* Session preference still works without storage. */
    }
    dispatch('changeMode');
  },
  changeMode({ state, commit }) {
    const dark = (state.mode === 'auto' ? state.systemMode : state.mode) === 'dark';
    document.documentElement.setAttribute('theme-mode', dark ? 'dark' : 'light');
    document.documentElement.classList.toggle('dark', dark);
    commit('changeChartColor', dark ? DARK_CHART_COLORS : LIGHT_CHART_COLORS);
  },
};
export default { namespaced: true, state, mutations, actions, getters };
