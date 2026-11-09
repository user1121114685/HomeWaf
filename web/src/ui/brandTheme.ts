import { COLOR_TOKEN, defaultDarkColor, defaultLightColor } from '../config/color';

const themes = ['default', 'cyan', 'green', 'yellow', 'orange', 'red', 'pink', 'purple'];

export function normalizeThemeColor(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const color = value.trim();
  if (/^#[\da-f]{6}$/i.test(color)) return color.toLowerCase();
  if (/^#[\da-f]{3}$/i.test(color)) return '#' + [...color.slice(1)].map((c) => c + c).join('').toLowerCase();
  const match = /^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$/i.exec(color);
  if (!match || match.slice(1).some((v) => Number(v) > 255)) return null;
  return '#' + match.slice(1).map((v) => Number(v).toString(16).padStart(2, '0')).join('');
}

export function brandThemeTokens(theme: string, mode: string): Record<string, string> {
  const custom = normalizeThemeColor(theme);
  const index = Math.max(0, themes.indexOf(theme));
  const primary = custom || (mode === 'dark' ? defaultDarkColor : defaultLightColor)[index];
  const channels = [1, 3, 5].map((offset) => parseInt(primary.slice(offset, offset + 2), 16));
  const mix = (background: number, weight: number) => '#' + channels.map((channel) =>
    Math.round(channel * (1 - weight) + background * weight).toString(16).padStart(2, '0'),
  ).join('');
  const tokens: Record<string, string> = {
    '--el-color-primary': primary,
    '--el-color-primary-rgb': channels.join(', '),
    '--el-color-primary-dark-2': mix(mode === 'dark' ? 255 : 0, 0.2),
  };
  for (let step = 1; step <= 9; step++) tokens[`--el-color-primary-light-${step}`] = mix(mode === 'dark' ? 20 : 255, step / 10);
  return tokens;
}

export function applyBrandTheme(theme: string, mode: string) {
  const custom = normalizeThemeColor(theme);
  const selected = custom || (themes.includes(theme) ? theme : 'default');
  document.documentElement.setAttribute('theme-color', selected);
  const tokens = brandThemeTokens(selected, mode);
  for (const [name, value] of Object.entries(tokens)) document.documentElement.style.setProperty(name, value);
  // Preserve named legacy palettes; restore custom colors on every load and mode change.
  document.documentElement.style.removeProperty('--td-brand-color');
  for (let step = 1; step <= 10; step++) document.documentElement.style.removeProperty(`--td-brand-color-${step}`);
  if (custom) {
    const legacy: Record<string, string> = {
      '--td-brand-color': custom,
      '--td-brand-color-7': tokens['--el-color-primary-light-3'],
      '--td-brand-color-8': custom,
      '--td-brand-color-9': tokens['--el-color-primary-dark-2'],
      '--td-brand-color-10': tokens['--el-color-primary-dark-2'],
    };
    for (let step = 1; step <= 6; step++) legacy[`--td-brand-color-${step}`] = tokens[`--el-color-primary-light-${10 - step}`];
    for (const [name, value] of Object.entries(legacy)) document.documentElement.style.setProperty(name, value);
    return { [custom]: legacy };
  }
  return { [selected.toUpperCase()]: COLOR_TOKEN[selected.toUpperCase()] };
}
