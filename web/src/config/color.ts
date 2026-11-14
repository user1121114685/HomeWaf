export type ColorToken = Record<string, string>;
export type ColorSeries = Record<string, ColorToken>;
export const defaultLightColor = [
  '#1f2de6',
  '#177d8c',
  '#14765b',
  '#926019',
  '#c13d45',
  '#526981',
  '#4385bd',
  '#81745e',
];
export const defaultDarkColor = [
  '#91a6ff',
  '#75c1cf',
  '#79cbb0',
  '#e2bd79',
  '#f19ba0',
  '#a4b9d4',
  '#79b3e3',
  '#c1b59f',
];
export const COLOR_TOKEN: ColorSeries = { DEFAULT: { '--td-brand-color': '#1f2de6' } };
export const LIGHT_CHART_COLORS: ColorToken = {
  textColor: '#161a23',
  placeholderColor: '#5d6470',
  borderColor: '#dfe3e9',
  containerColor: '#ffffff',
};
export const DARK_CHART_COLORS: ColorToken = {
  textColor: '#e9edf4',
  placeholderColor: '#a4adbc',
  borderColor: '#303947',
  containerColor: '#191f29',
};
export function getBrandColor(_type: string, _colors: ColorSeries): ColorToken {
  return COLOR_TOKEN.DEFAULT;
}
