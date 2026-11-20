// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.
import { clearAppOpPassword } from '@/utils/appOperationPassword';
import { heartbeat_api } from '@/apis/common';

const state = { token: localStorage.getItem('access_token') || '', userInfo: { name: '', roles: [] as string[] } };
const mutations = {
  setToken(state: any, token: string) {
    if (state.token !== token) clearAppOpPassword();
    state.token = token;
    localStorage.setItem('access_token', token);
  },
  removeToken(state: any) {
    clearAppOpPassword();
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
    commit('notification/setMsgData', [], { root: true });
    commit('setUserInfo', { name: '', roles: [] });
    commit('setToken', token);
  },
  async getUserInfo({ commit, state }: any) {
    if (!state.token) throw new Error('Not authenticated');
    const token = state.token;
    const result = await heartbeat_api();
    const role = result.data?.role;
    if (state.token !== token || result.code !== 0 || !['superAdmin', 'systemAdmin', 'securityAdmin', 'auditAdmin'].includes(role)) {
      throw new Error('账号角色无效，请重新登录');
    }
    commit('setUserInfo', { name: result.data.login_account, role, roles: ['ALL_ROUTERS'] });
  },
  async logout({ commit }: any) {
    commit('notification/setMsgData', [], { root: true });
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
