// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import NProgress from '@/utils/progress'; // progress bar
 // progress bar style

import store from '@/store';
import router from '@/router';

NProgress.configure({ showSpinner: false });

const whiteListRouters = store.getters['permission/whiteListRouters'];

router.beforeEach(async (to) => {
  NProgress.start();
  if (to.path === '/help' || to.name === 'DashboardShared') return;

  const token = store.getters['user/token'];

  if (token) {
    if (to.path === '/login') {
      await store.dispatch('user/logout');
      await store.dispatch('permission/restore');
      return;
    }

    const roles = store.getters['user/roles'];

    if (roles && roles.length > 0) {
      return;
    } else {
      try {
        await store.dispatch('user/getUserInfo');

        await store.dispatch('permission/initRoutes', store.getters['user/roles']);

        return;
      } catch (error) {
        await store.commit('user/removeToken');
        NProgress.done();
        return { path: '/login', query: { redirect: to.fullPath } };
      }
    }
  } else {
    /* white list router */
    if (whiteListRouters.indexOf(to.path) !== -1) {
      return;
    } else {
      return { path: '/login', query: { redirect: to.fullPath } };
    }
    NProgress.done();
  }
});

router.afterEach(() => {
  NProgress.done();
});
