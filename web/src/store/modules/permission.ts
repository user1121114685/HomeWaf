// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { asyncRouterList } from '@/router';

function filterPermissionsRouters(routes, roles) {
  const res = [];
  routes.forEach((route) => {
    const children = [];
    route.children?.forEach((childRouter) => {
      const roleCode = childRouter.meta?.roleCode || childRouter.name;
      if (roles.indexOf(roleCode) !== -1) {
        children.push(childRouter);
      }
    });
    if (children.length > 0) {
      res.push({ ...route, children });
    }
  });
  return res;
}

const state = {
  whiteListRouters: ['/login'],
  routers: [],
};

const mutations = {
  setRouters: (state, routers) => {
    state.routers = routers;
  },
};

const getters = {
  routers: (state) => state.routers,
  whiteListRouters: (state) => state.whiteListRouters,
};

const actions = {
  async initRoutes({ commit }, roles) {
    let accessedRouters;

    // special token
    if (roles.includes('ALL_ROUTERS')) {
      accessedRouters = asyncRouterList;
    } else {
      accessedRouters = filterPermissionsRouters(asyncRouterList, roles);
    }

    commit('setRouters', accessedRouters);

    // register routers
    // router.addRoutes(state.routers);
  },
  async restore({ commit }) {
    // Routes are registered statically. Removing them during login invalidates
    // the active RouterView and can leave subsequent navigation on a stale view.
    commit('setRouters', []);
  },
};

export default {
  namespaced: true,
  state,
  mutations,
  actions,
  getters,
};
