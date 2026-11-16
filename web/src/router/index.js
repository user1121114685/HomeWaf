// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import { createRouter, createWebHashHistory } from 'vue-router';
import baseRouters from './modules/base';
import wafRouters from './modules/waf';
import otherRouters from './modules/others';
export const asyncRouterList = [...baseRouters, ...wafRouters, ...otherRouters];
const routes = [
  { path: '/dashboard/shared', name: 'DashboardShared', component: () => import('@/pages/dashboard/screen/Shared.vue'), meta: { title: '只读监控大屏' } },
  { path: '/help', name: 'help', component: () => import('@/pages/help/index.vue') },
  { path: '/login', name: 'login', component: () => import('@/pages/login/index.vue') },
  ...asyncRouterList,
  { path: '/:pathMatch(.*)*', redirect: '/dashboard/base' },
];
const router = createRouter({ history: createWebHashHistory(), routes, scrollBehavior: () => ({ left: 0, top: 0 }) });

export default router;
