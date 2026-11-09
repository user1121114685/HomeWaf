// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import NProgress from '@/utils/progress'; // progress bar
 // progress bar style

import store from '@/store';
import router from '@/router';

NProgress.configure({ showSpinner: false });

const whiteListRouters = store.getters['permission/whiteListRouters'];

router.beforeEach(async (to, from, next) => {
  NProgress.start();
  if (to.path === '/help' || to.name === 'DashboardShared') { next(); return; }

  const token = store.getters['user/token'];

  if (token) {
    if (to.path === '/login') {
      setTimeout(() => {
        store.dispatch('user/logout');
        store.dispatch('permission/restore');
      });
      next();
      return;
    }

    const roles = store.getters['user/roles'];

    if (roles && roles.length > 0) {
      next();
    } else {
      try {
        await store.dispatch('user/getUserInfo');

        await store.dispatch('permission/initRoutes', store.getters['user/roles']);

        next({ ...to });
      } catch (error) {
        await store.commit('user/removeToken');
        next(`/login?redirect=${to.path}`);
        NProgress.done();
      }
    }
  } else {
    /* white list router */
    if (whiteListRouters.indexOf(to.path) !== -1) {
      next();
    } else {
      next(`/login?redirect=${to.path}`);
    }
    NProgress.done();
  }
});

router.afterEach(() => {
  NProgress.done();
});
