import * as core from 'echarts/core';
import { watch } from 'vue';
import store from '@/store';
import { defaultDarkColor, defaultLightColor, DARK_CHART_COLORS, LIGHT_CHART_COLORS } from '@/config/color';

export * from 'echarts/core';

const instances = new Set<WeakRef<core.ECharts>>();
const colorsFor = (dark: boolean) => (dark ? DARK_CHART_COLORS : LIGHT_CHART_COLORS);
const typography = { fontFamily: 'Geist, Microsoft YaHei, sans-serif', fontSize: 13 };
function axisTheme(dark: boolean) {
  const colors = colorsFor(dark);
  return {
    axisLabel: { ...typography, color: colors.placeholderColor },
    nameTextStyle: { ...typography, color: colors.placeholderColor },
    axisLine: { lineStyle: { color: colors.borderColor } },
    splitLine: { lineStyle: { color: colors.borderColor } },
  };
}
function theme(dark: boolean) {
  const colors = colorsFor(dark);
  return {
    backgroundColor: 'transparent',
    color: dark ? defaultDarkColor : defaultLightColor,
    textStyle: { ...typography, color: colors.textColor },
    title: { textStyle: { ...typography, color: colors.textColor } },
    legend: { textStyle: { ...typography, color: colors.textColor } },
    tooltip: {
      backgroundColor: colors.containerColor,
      borderColor: colors.borderColor,
      textStyle: { ...typography, color: colors.textColor },
    },
    categoryAxis: axisTheme(dark),
    valueAxis: axisTheme(dark),
    timeAxis: axisTheme(dark),
  };
}
core.registerTheme('homewaf-light', theme(false));
core.registerTheme('homewaf-dark', theme(true));

function isDark(dom: HTMLElement) {
  const screen = dom.closest('.security-screen');
  return screen ? screen.classList.contains('is-dark') : store.getters['setting/mode'] === 'dark';
}
function aliveCharts(callback?: (chart: core.ECharts) => void) {
  for (const reference of instances) {
    const chart = reference.deref();
    if (!chart || chart.isDisposed()) instances.delete(reference);
    else callback?.(chart);
  }
}
export function init(dom: HTMLElement, _theme?: string | object, options?: Parameters<typeof core.init>[2]) {
  aliveCharts();
  const chart = core.init(dom, isDark(dom) ? 'homewaf-dark' : 'homewaf-light', options);
  instances.add(new WeakRef(chart));
  return chart;
}

// Update presentation only. Series data, zoom, filters and selections remain intact.
const stop = watch(
  () => store.getters['setting/mode'],
  () =>
    aliveCharts((chart) => {
      const dark = isDark(chart.getDom()),
        palette = theme(dark),
        current = chart.getOption();
      const patch: Record<string, unknown> = { textStyle: palette.textStyle };
      for (const key of ['xAxis', 'yAxis']) {
        if (Array.isArray(current[key])) patch[key] = current[key].map(() => axisTheme(dark));
      }
      if (current.legend) patch.legend = palette.legend;
      if (current.tooltip) patch.tooltip = palette.tooltip;
      chart.setOption(patch);
    }),
  { flush: 'post' },
);
if (import.meta.hot) import.meta.hot.dispose(stop);
