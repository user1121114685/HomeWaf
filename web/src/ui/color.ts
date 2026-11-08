// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

const clamp = (n: number, min = 0, max = 1) => Math.min(max, Math.max(min, n));
function rgb(hex: string) {
  const h = hex.replace('#', '');
  const full =
    h.length === 3
      ? h
          .split('')
          .map((c) => c + c)
          .join('')
      : h;
  return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255);
}
function hex(values: number[]) {
  return (
    '#' +
    values
      .map((v) =>
        Math.round(clamp(v) * 255)
          .toString(16)
          .padStart(2, '0'),
      )
      .join('')
  );
}
function toHsl(color: string) {
  const [r, g, b] = rgb(color);
  const max = Math.max(r, g, b),
    min = Math.min(r, g, b),
    d = max - min,
    l = (max + min) / 2;
  let h = 0,
    s = 0;
  if (d) {
    s = d / (1 - Math.abs(2 * l - 1));
    h = (max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4) * 60;
  }
  return [h, s * 100, l * 100];
}
function fromHsl([h, s, l]: number[]) {
  s = clamp(s / 100);
  l = clamp(l / 100);
  const c = (1 - Math.abs(2 * l - 1)) * s,
    x = c * (1 - Math.abs(((h / 60) % 2) - 1)),
    m = l - c / 2;
  const v =
    h < 60
      ? [c, x, 0]
      : h < 120
        ? [x, c, 0]
        : h < 180
          ? [0, c, x]
          : h < 240
            ? [0, x, c]
            : h < 300
              ? [x, 0, c]
              : [c, 0, x];
  return hex(v.map((n) => n + m));
}
export const Color = {
  colorTransform(value: any, from: string, to: string): any {
    return from === 'hex' && to === 'hsl' ? toHsl(value) : from === 'hsl' && to === 'hex' ? fromHsl(value) : value;
  },
  getPaletteByGradation({ colors, step = 10 }: any) {
    return colors.map((color: string) =>
      Array.from({ length: step }, (_, i) =>
        i === 7
          ? color
          : hex(rgb(color).map((c) => (i < 7 ? c + (1 - c) * (0.9 - i * 0.12) : c * (1 - (i - 7) * 0.17)))),
      ),
    );
  },
  getRandomPalette({ color, number = 8 }: any) {
    const [h, s, l] = toHsl(color);
    return Array.from({ length: number }, (_, i) => fromHsl([(h + (i * 360) / number) % 360, s, l]));
  },
};
