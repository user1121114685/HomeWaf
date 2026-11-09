// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { DashboardIcon } from '@/ui/icons';
import Layout from '@/layouts/index.vue';

export default [
  {
    path: '/dashboard',
    component: Layout,
    redirect: '/dashboard/base',
    name: 'dashboard',
    meta: {
      title: 'menu.dashboard.parent_title',
      icon: DashboardIcon,
    },
    children: [
      {
        path: 'base',
        name: 'DashboardBase',
        component: () => import('@/pages/dashboard/base/index.vue'),
        meta: { title: 'menu.dashboard.dashboard_title'},
      },
      {
        path: 'screen',
        name: 'DashboardScreen',
        component: () => import('@/pages/dashboard/screen/index.vue'),
        meta: { title: 'menu.dashboard.screen_title', fullscreen: true, openInNewTab: true, roleCode: 'DashboardBase' },
      },
      {
        path: 'stats',
        name: 'DashboardStats',
        component: () => import('@/pages/dashboard/stats/index.vue'),
        meta: { title: 'menu.dashboard.stats_title'},
      },
      {
        path: 'overview',
        name: 'DashboardOverview',
        component: () => import('@/pages/dashboard/overview/index.vue'),
        meta: { title: 'menu.dashboard.overview_title'},
      },
    ],
  },
];
