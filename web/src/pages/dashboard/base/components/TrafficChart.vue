<template>
  <div ref="container" class="traffic-chart" role="img" :aria-label="label" />
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch, type PropType } from 'vue';
import * as echarts from '@/ui/charts';
import { BarChart, LineChart, PieChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
echarts.use([BarChart, LineChart, PieChart, GridComponent, TooltipComponent, CanvasRenderer]);
const props = defineProps({
  option: { type: Object as PropType<echarts.EChartsCoreOption>, required: true },
  label: { type: String, required: true },
});
const container = ref<HTMLDivElement>();
let chart: echarts.ECharts | undefined;
let observer: ResizeObserver | undefined;
onMounted(() => {
  chart = echarts.init(container.value);
  chart.setOption(props.option);
  observer = new ResizeObserver(() => chart?.resize());
  observer.observe(container.value!);
});
watch(
  () => props.option,
  (option) => chart?.setOption(option, true),
);
onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>
<style scoped>
.traffic-chart {
  width: 100%;
  height: 100%;
  min-width: 0;
}
</style>
