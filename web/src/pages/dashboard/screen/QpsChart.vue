<template>
  <div
    ref="element"
    class="qps-chart"
    role="img"
    :aria-label="`实时 QPS 趋势，共 ${points.length} 个采样点，当前 ${current} 次每秒`"
  ></div>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue';
import * as echarts from '@/ui/charts';
import { LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);
const props = defineProps({
  points: { type: Array as PropType<{ T: number; V: number }[]>, required: true },
  current: { type: String, required: true },
  dark: { type: Boolean, required: true },
});
const element = ref<HTMLElement>();
let chart: echarts.ECharts | undefined;
let observer: ResizeObserver | undefined;
function render() {
  chart?.setOption(
    {
      animation: false,
      grid: { left: 42, right: 14, top: 18, bottom: 24 },
      tooltip: {
        trigger: 'axis',
        renderMode: 'richText',
        confine: true,
        backgroundColor: props.dark ? '#152238' : '#ffffff',
        textStyle: { color: props.dark ? '#e6ecfa' : '#23334b' },
      },
      xAxis: {
        type: 'time',
        axisLine: { show: false },
        axisTick: { show: false },
        splitNumber: 6,
        axisLabel: {
          color: props.dark ? '#8495af' : '#7a869b',
          fontSize: 12,
          hideOverlap: true,
          formatter: '{HH}:{mm}:{ss}',
        },
      },
      yAxis: {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitNumber: 2,
        axisLabel: { color: props.dark ? '#8495af' : '#7a869b', fontSize: 12 },
        splitLine: { lineStyle: { color: props.dark ? '#223049' : '#edf0f7', type: 'dashed' } },
      },
      series: [
        {
          name: 'QPS（次/秒）',
          type: 'line',
          showSymbol: props.points.length === 1,
          symbolSize: 6,
          connectNulls: false,
          lineStyle: { color: props.dark ? '#91a6ff' : '#1f2de6', width: 2 },
          itemStyle: { color: props.dark ? '#91a6ff' : '#1f2de6' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(136,115,244,.25)' },
              { offset: 1, color: 'rgba(136,115,244,.015)' },
            ]),
          },
          data: props.points.map((p) => [p.T * 1000, p.V]),
        },
      ],
    },
    true,
  );
}
onMounted(() => {
  chart = echarts.init(element.value);
  observer = new ResizeObserver(() => chart?.resize());
  observer.observe(element.value!);
  render();
});
watch(() => [props.points, props.dark], render);
onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>
<style scoped>
.qps-chart {
  height: 100%;
  min-height: 100px;
  width: 100%;
}
</style>
