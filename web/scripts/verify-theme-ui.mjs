import { expect } from '@playwright/test';

// Test preferences and monitor behavior; visual geometry is reviewed separately.
export async function verifyThemeSettings(page, root, _output, report) {
  const drawer = page.locator('.console-preferences');
  const open = async () => {
    await page.getByRole('button', { name: '系统设置', exact: true }).click();
    await expect(drawer).toBeVisible();
  };
  const close = async () => {
    await drawer.locator('.el-drawer__close-btn').click();
    await expect(drawer).toBeHidden();
  };
  const choose = async (value) => drawer.locator(`label:has(input[value="${value}"])`).click();
  await open();
  await choose('light');
  await expect(root).toHaveAttribute('data-theme', 'light');
  await close();
  await page.reload();
  await expect(root).toHaveAttribute('data-theme', 'light');
  await open();
  await choose('dark');
  await expect(root).toHaveAttribute('data-theme', 'dark');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(root).toHaveAttribute('data-theme', 'dark');
  await choose('auto');
  await expect(root).toHaveAttribute('data-theme', 'light');
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(root).toHaveAttribute('data-theme', 'dark');
  await close();
  report.checks.push('Appearance persists across reload; explicit mode ignores OS changes and auto follows them');
}

export async function verifySystemMonitor(page, _output, report) {
  const popup = page.locator('.system-monitor-popup');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole('button', { name: '系统监控', exact: true }).click();
  await expect(popup).toBeVisible();
  const panel = popup.locator('.system-monitor-panel');
  await expect(panel.locator('.disk-item')).toHaveCount(12);
  await panel.locator('.monitor-content').evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await expect(panel.locator('.disk-item').last()).toBeInViewport();
  await expect(panel.locator('.monitor-footer')).toBeInViewport();
  await panel.locator('button').first().press('Escape');
  await page.mouse.move(1435, 995);
  await expect(popup).toBeHidden();
  await page.setViewportSize({ width: 1920, height: 1200 });
  report.checks.push('System monitor keeps long disk lists scrollable and closes with Escape');
}
