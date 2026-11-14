// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import * as echarts from 'echarts/core';

import { defaultLightColor, defaultDarkColor } from '@/config/color';
import store from '@/store';

const { state } = store;

/**
 * 依据主题类型获取颜色
 *
 * @export
 * @param {string} theme
 * @returns {string[]}
 */
export function getColorFromTheme(_theme: string): Array<string> {
  return store.getters['setting/mode'] === 'dark' ? [...defaultDarkColor] : [...defaultLightColor];
}

/** 图表颜色 */
export function getChartListColor(): Array<string> {
  const { setting } = state;
  const res = getColorFromTheme(setting.brandTheme);

  return res;
}

/**
 * 更改图表主题颜色
 *
 * @export
 * @param {Array<any>} chartsList
 * @param {string} theme
 */
export function changeChartsTheme(chartsList: echarts.EChartsType[]): void {
  if (chartsList && chartsList.length) {
    const chartChangeColor = getChartListColor();

    for (let index = 0; index < chartsList.length; index++) {
      const elementChart = chartsList[index];

      if (elementChart) {
        const optionVal = elementChart.getOption();

        // 更改主题颜色
        optionVal.color = chartChangeColor;
        elementChart.setOption(optionVal, true);
      }
    }
  }
}
