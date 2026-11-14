import test from 'node:test';
import assert from 'node:assert/strict';
import { navigationItems, filterNavigation } from '../src/ui/navigation.ts';

test('navigation and search cannot expose hidden or unpermitted routes', () => {
  const permitted = [
    {
      path: '/services',
      meta: { title: '服务' },
      children: [
        { path: 'ports', name: 'Ports', meta: { title: '服务接入' } },
        { path: 'internal', name: 'Internal', meta: { title: '内部配置', hidden: true } },
        { path: 'old', name: 'Redirect', redirect: '/services/ports', meta: { title: '旧入口' } },
      ],
    },
    {
      path: '/admin',
      meta: { title: '隐藏分组', hidden: true },
      children: [{ path: 'keys', name: 'Keys', meta: { title: '签名密钥' } }],
    },
  ];
  const items = navigationItems(permitted, (value) => value);
  assert.deepEqual(
    items.map((item) => item.path),
    ['/services/ports'],
  );
  assert.equal(filterNavigation(items, '签名密钥').length, 0);
  assert.equal(filterNavigation(items, '服务 ports')[0].name, 'Ports');
});

test('nested and absolute routes resolve correctly and retain new-tab isolation', () => {
  const items = navigationItems(
    [
      {
        path: '/dashboard',
        meta: { title: '仪表盘' },
        children: [
          { path: 'screen', name: 'Screen', meta: { title: '监控大屏', openInNewTab: true } },
          { path: '/profile', name: 'Profile', meta: { title: '个人中心' } },
        ],
      },
    ],
    (value) => value,
  );
  assert.deepEqual(
    items.map(({ path, newTab }) => ({ path, newTab })),
    [
      { path: '/dashboard/screen', newTab: true },
      { path: '/profile', newTab: false },
    ],
  );
});
