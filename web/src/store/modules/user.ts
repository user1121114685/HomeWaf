// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

const state = { token: localStorage.getItem('access_token') || '', userInfo: { name: '', roles: [] as string[] } };
const mutations = {
  setToken(state: any, token: string) {
    state.token = token;
    localStorage.setItem('access_token', token);
  },
  removeToken(state: any) {
    state.token = '';
    localStorage.removeItem('access_token');
    localStorage.removeItem('tdesign-starter');
  },
  setUserInfo(state: any, value: any) {
    state.userInfo = value;
  },
};
const actions = {
  async login({ commit }: any) {
    const token = localStorage.getItem('access_token');
    if (!token) throw new Error('A verified backend login is required');
    commit('setToken', token);
  },
  async getUserInfo({ commit, state }: any) {
    if (!state.token) throw new Error('Not authenticated');
    commit('setUserInfo', { name: localStorage.getItem('current_account') || '', roles: ['ALL_ROUTERS'] });
  },
  async logout({ commit }: any) {
    commit('removeToken');
    commit('setUserInfo', { name: '', roles: [] });
  },
};
// ALL_ROUTERS controls navigation only. Every API remains protected by backend RBAC.
export default {
  namespaced: true,
  state,
  mutations,
  actions,
  getters: { token: (s: any) => s.token, roles: (s: any) => s.userInfo.roles },
};
