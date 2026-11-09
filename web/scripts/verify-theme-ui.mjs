import { expect } from '@playwright/test';
import { fileURLToPath } from 'node:url';

export async function verifySystemMonitor(page, output, report) {
  const popup = page.locator('.system-monitor-popup');
  for (const [theme, width] of [
    ['light', 1440],
    ['dark', 1440],
    ['light', 390],
  ]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.getByRole('button', { name: '系统设置', exact: true }).click();
    await page.locator(`.setting-theme-modes label:has(input[value="${theme}"])`).click();
    await page.locator('.setting-drawer-container .el-drawer__close-btn').click();
    await expect(page.locator('.setting-drawer-container')).toBeHidden();
    await page.getByRole('button', { name: '系统监控', exact: true }).click();
    await expect(popup).toBeVisible();
    await expect(popup).toHaveCSS('opacity', '1');
    const panel = popup.locator('.system-monitor-panel');
    await expect(panel.locator('.disk-item')).toHaveCount(12);
    const frame = await popup.boundingBox(),
      content = await panel.boundingBox();
    expect(content.x).toBeGreaterThanOrEqual(frame.x);
    expect(content.x + content.width).toBeLessThanOrEqual(frame.x + frame.width + 1);
    expect(content.y + content.height).toBeLessThanOrEqual(frame.y + frame.height + 1);
    expect(frame.x).toBeGreaterThanOrEqual(0);
    expect(frame.x + frame.width).toBeLessThanOrEqual(width);
    await expect
      .poll(() => panel.evaluate((el) => getComputedStyle(el).backgroundColor))
      .toBe(theme === 'light' ? 'rgb(255, 255, 255)' : 'rgb(29, 30, 31)');
    expect(await panel.locator('.monitor-content').evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
    await panel.locator('.monitor-content').evaluate((el) => (el.scrollTop = el.scrollHeight));
    await expect(panel.locator('.disk-item').last()).toBeInViewport();
    await expect(panel.locator('.monitor-footer')).toBeInViewport();
    await popup.screenshot({ path: fileURLToPath(new URL(`system-monitor-${theme}-${width}.png`, output)) });
    await panel.locator('button').first().press('Escape');
    await page.mouse.move(width - 5, 995);
    await expect(popup).toBeHidden();
  }
  await page.setViewportSize({ width: 1920, height: 1200 });
  report.checks.push(
    'System monitor background encloses all content in light/dark and on mobile; long disk lists scroll above a visible footer',
  );
}

export async function verifyThemeSettings(page, root, output, report) {
  const drawer = page.locator('.setting-drawer-container');
  const open = async () => {
    await page.getByRole('button', { name: '系统设置', exact: true }).click();
    await expect(drawer).toBeVisible();
  };
  const close = async () => {
    await drawer.locator('.el-drawer__close-btn').click();
    await expect(drawer).toBeHidden();
  };
  const choose = async (value) => {
    await drawer.locator(`.setting-theme-modes label:has(input[value="${value}"])`).click();
  };
  const chartColor = () =>
    page
      .locator('.trend-chart .traffic-chart')
      .first()
      .evaluate((el) => el.__vueParentComponent.props.option.series[0].lineStyle.color);
  await open();
  await expect(drawer.locator('.setting-theme-modes .setting-choice-label')).toHaveText(['明亮', '暗黑', '跟随系统']);
  const positions = await drawer
    .locator('.setting-theme-modes .el-radio-button')
    .evaluateAll((items) => items.map((el) => el.getBoundingClientRect().toJSON()));
  expect(positions.every((p) => p.width >= 90 && p.height >= 58)).toBe(true);
  expect(positions[1].left - positions[0].right).toBeGreaterThanOrEqual(8);
  expect(positions[2].left - positions[1].right).toBeGreaterThanOrEqual(8);
  expect(Math.abs((await drawer.boundingBox()).width - 408)).toBeLessThan(2);
  expect(
    await drawer
      .locator('.setting-navigation-layout img')
      .evaluateAll((images) => images.every((image) => image.complete && image.naturalWidth > 0)),
  ).toBe(true);
  const switchEdges = await drawer
    .locator('.el-form-item__content .el-switch')
    .evaluateAll((items) =>
      items.filter((el) => el.getBoundingClientRect().width).map((el) => el.getBoundingClientRect().right),
    );
  expect(Math.max(...switchEdges) - Math.min(...switchEdges)).toBeLessThan(2);
  await choose('light');
  await expect(drawer.locator('.setting-theme-modes input[value="light"]')).toBeChecked();
  await expect(root).toHaveAttribute('data-theme', 'light');
  await expect(page.locator('html')).not.toHaveClass(/dark/);
  expect(await root.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(242, 244, 247)');
  expect(await chartColor()).toBe('#3485b4');
  await expect.poll(() => drawer.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(255, 255, 255)');
  await drawer.screenshot({ path: fileURLToPath(new URL('settings-light.png', output)) });
  await close();
  await page.screenshot({ path: fileURLToPath(new URL('overview-light-theme.png', output)) });
  await page.reload();
  await expect(root).toHaveAttribute('data-theme', 'light');
  await open();
  await expect(drawer.locator('.setting-theme-modes input[value="light"]')).toBeChecked();
  await choose('dark');
  await expect(drawer.locator('.setting-theme-modes input[value="dark"]')).toBeChecked();
  await expect(root).toHaveAttribute('data-theme', 'dark');
  expect(await chartColor()).toBe('#85c7eb');
  await expect.poll(() => drawer.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(20, 20, 20)');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(root).toHaveAttribute('data-theme', 'dark');
  await drawer.screenshot({ path: fileURLToPath(new URL('settings-dark.png', output)) });
  await choose('auto');
  await expect(root).toHaveAttribute('data-theme', 'light');
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(root).toHaveAttribute('data-theme', 'dark');
  await expect(page.locator('html')).toHaveClass(/dark/);
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(root).toHaveAttribute('data-theme', 'light');
  await expect(page.locator('html')).not.toHaveClass(/dark/);
  expect(await chartColor()).toBe('#3485b4');
  await page.setViewportSize({ width: 390, height: 1000 });
  expect((await drawer.boundingBox()).width).toBeLessThanOrEqual(390);
  expect(await drawer.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
  await drawer.screenshot({ path: fileURLToPath(new URL('settings-mobile.png', output)) });
  await page.setViewportSize({ width: 1920, height: 1200 });
  await choose('dark');
  await close();
  report.checks.push('Theme choices have labels and spacing; drawer width, icons and switches align');
  report.checks.push(
    'Light/dark updates cards and chart palettes; persisted mode survives reload; auto follows OS changes',
  );
}

export async function verifySharedComponents(page, output, report) {
  await page.evaluate(async () => {
    const { h, render, resolveComponent } = await import('/node_modules/.vite/deps/vue.js');
    const { icons } = await import('/src/ui/icons.ts');
    const mount = document.createElement('div');
    mount.id = 'ui-compat-verification';
    document.body.append(mount);
    const component = {
      setup() {
        return () =>
          h(
            'section',
            {
              class: 'ui-compat-fixture',
              style: 'position:fixed;inset:0;z-index:10000;padding:24px;background:var(--el-bg-color);overflow:auto',
            },
            [
              h(
                resolveComponent('hw-card'),
                { title: '组件对齐检查', subtitle: '标题、操作、表单、栅格与图标', bordered: false },
                {
                  actions: () => h(resolveComponent('hw-button'), { 'data-testid': 'card-action' }, () => '刷新'),
                  default: () => [
                    h(resolveComponent('hw-row'), { gutter: [16, 24], class: 'fixture-grid' }, () =>
                      [1, 2, 3, 4].map((n) =>
                        h(resolveComponent('hw-col'), { xs: 12, sm: 6, xl: 4, class: 'fixture-col' }, () =>
                          h('div', { style: 'height:40px;background:var(--el-fill-color)' }, String(n)),
                        ),
                      ),
                    ),
                    h(
                      resolveComponent('hw-form'),
                      { data: {}, style: 'margin-top:24px', class: 'fixture-form' },
                      () => [
                        h(
                          resolveComponent('hw-form-item'),
                          { label: '名称' },
                          {
                            default: () => h(resolveComponent('hw-input'), { modelValue: '本地测试' }),
                            help: () => '说明文字应显示在输入框下方',
                          },
                        ),
                        h(resolveComponent('hw-form-item'), { label: '较长的字段标题' }, () =>
                          h(resolveComponent('hw-input'), { modelValue: '对齐检查' }),
                        ),
                      ],
                    ),
                    h(
                      'div',
                      { style: 'display:flex;gap:12px;flex-wrap:wrap;margin-top:20px', class: 'fixture-icons' },
                      Object.entries(icons).map(([name, icon]) => h(icon, { size: 24, title: name })),
                    ),
                    h(
                      'div',
                      { class: 'fixture-named-icons', style: 'display:flex;gap:12px;margin-top:16px' },
                      [
                        'menu-fold',
                        'menu-unfold',
                        'internet',
                        'hard-disk-storage',
                        'view-list',
                        'user-safety',
                        'arrow-down-circle',
                        'fullscreen',
                      ].map((name) => h(resolveComponent('hw-icon'), { name, size: 24 })),
                    ),
                  ],
                },
              ),
            ],
          );
      },
    };
    const vnode = h(component);
    vnode.appContext = window.vm.$.appContext;
    render(vnode, mount);
    window.__cleanupUiVerification = () => {
      render(null, mount);
      mount.remove();
    };
  });
  const fixture = page.locator('.ui-compat-fixture');
  await expect(fixture.getByText('组件对齐检查', { exact: true })).toBeVisible();
  await expect(fixture.getByText('标题、操作、表单、栅格与图标', { exact: true })).toBeVisible();
  await expect(fixture.getByTestId('card-action')).toBeVisible();
  await expect(fixture.getByText('说明文字应显示在输入框下方', { exact: true })).toBeVisible();
  const fields = await fixture
    .locator('.fixture-form .el-input')
    .evaluateAll((items) => items.map((el) => el.getBoundingClientRect().left));
  expect(Math.abs(fields[0] - fields[1])).toBeLessThan(1);
  const help = await fixture.locator('.hw-form-help').boundingBox();
  const input = await fixture.locator('.fixture-form .el-input').first().boundingBox();
  expect(help.y).toBeGreaterThanOrEqual(input.y + input.height);
  expect(await fixture.locator('.fixture-named-icons [data-icon="QuestionFilled"]').count()).toBe(0);
  expect(await fixture.locator('.fixture-icons svg').count()).toBeGreaterThan(80);
  const sizes = await fixture
    .locator('.fixture-icons .hw-icon')
    .evaluateAll((items) => items.map((el) => el.getBoundingClientRect().width));
  expect(sizes.every((n) => Math.abs(n - 24) < 1)).toBe(true);
  for (const [width, fraction] of [
    [1440, 1 / 3],
    [1024, 1 / 2],
    [390, 1],
  ]) {
    await page.setViewportSize({ width, height: 1000 });
    await expect
      .poll(async () => {
        const bounds = await fixture.locator('.fixture-grid').boundingBox();
        const col = await fixture.locator('.fixture-col').first().boundingBox();
        return Math.abs(col.width / bounds.width - fraction);
      })
      .toBeLessThan(0.01);
  }
  await page.setViewportSize({ width: 1440, height: 1100 });
  expect(await fixture.locator('.fixture-grid').evaluate((el) => getComputedStyle(el).rowGap)).toBe('24px');
  await page.screenshot({ path: fileURLToPath(new URL('shared-ui-alignment.png', output)) });
  await page.evaluate(() => window.__cleanupUiVerification());
  await page.setViewportSize({ width: 1920, height: 1200 });
  report.checks.push(
    'Shared cards preserve headers/actions, form labels/help align, 12-column responsive gutters render correctly',
  );
  report.checks.push(
    'All exported icons render at correct size; common named icons resolve without fallback placeholders',
  );
}
