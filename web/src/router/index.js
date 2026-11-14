// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { createRouter, createWebHashHistory } from 'vue-router';
import baseRouters from './modules/base';
import wafRouters from './modules/waf';
export const asyncRouterList = [...baseRouters, ...wafRouters];
const routes = [
  { path: '/dashboard/shared', name: 'DashboardShared', component: () => import('@/pages/dashboard/screen/Shared.vue'), meta: { title: '只读监控大屏' } },
  { path: '/help', name: 'help', component: () => import('@/pages/help/index.vue') },
  { path: '/login', name: 'login', component: () => import('@/pages/login/index.vue') },
  ...asyncRouterList,
  { path: '/:pathMatch(.*)*', redirect: '/dashboard/start' },
];
const router = createRouter({ history: createWebHashHistory(), routes, scrollBehavior: () => ({ left: 0, top: 0 }) });
export function resetRouter() {
  for (const r of router.getRoutes()) if (r.name) router.removeRoute(r.name);
  for (const r of routes) router.addRoute(r);
}
export default router;
