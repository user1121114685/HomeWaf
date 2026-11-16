<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <section class="region-analysis">
    <hw-card :title="english ? 'Traffic by region' : '地区流量统计'">
      <div class="analysis-filter-row">
        <label><span>{{ english ? 'Date range' : '日期范围' }}</span><hw-date-range-picker v-model="range" :aria-label="english ? 'Date range' : '日期范围'" /></label>
        <label><span>{{ english ? 'Action' : '防护动作' }}</span><hw-select v-model="action" :options="actions" style="width:160px" :aria-label="english ? 'Action' : '防护动作'" /></label>
        <hw-button theme="primary" :loading="loading" :disabled="!range || range.length !== 2" @click="load">{{ $t('common.search') }}</hw-button>
      </div>
      <div ref="chart" class="region-chart"></div>
      <hw-table :data="rows" :columns="columns" row-key="name" :loading="loading" />
    </hw-card>
  </section>
</template>
<script>
import * as echarts from '@/ui/charts';
import {BarChart} from 'echarts/charts';
import {GridComponent,TooltipComponent} from 'echarts/components';
import {CanvasRenderer} from 'echarts/renderers';
import {wafanalysisdaycountryrange} from '@/apis/analysis';
import {LAST_7_DAYS} from '@/utils/date';
echarts.use([BarChart,GridComponent,TooltipComponent,CanvasRenderer]);
export default {
 name:'Analysis',
 data:()=>({range:[...LAST_7_DAYS],action:'all',rows:[],loading:false,chart:null,requestSequence:0,observer:null}),
 computed:{english(){return this.$store.state.language.currentLanguage==='en_US';},actions(){return [{label:this.english?'All':'全部',value:'all'},{label:this.english?'Blocked':'阻止',value:'阻止'},{label:this.english?'Allowed':'放行',value:'放行'}];},columns(){return [{colKey:'name',title:this.english?'Region':'地区'},{colKey:'value',title:this.english?'Requests':'请求数'}];}},
 mounted(){this.chart=echarts.init(this.$refs.chart);this.observer=new ResizeObserver(()=>this.chart?.resize());this.observer.observe(this.$refs.chart);this.load();},
 beforeUnmount(){this.requestSequence++;this.observer?.disconnect();this.chart?.dispose();},
 methods:{async load(){if(!this.range||this.range.length!==2)return;const sequence=++this.requestSequence;this.loading=true;try{const response=await wafanalysisdaycountryrange({start_day:this.range[0].replace(/-/g,''),end_day:this.range[1].replace(/-/g,''),attack_type:this.action});if(sequence!==this.requestSequence)return;this.rows=(response.data||[]).map(row=>({name:String(row.name||''),value:Number(row.value)||0})).sort((a,b)=>b.value-a.value);const visible=this.rows.slice(0,30).reverse();this.chart.setOption({tooltip:{trigger:'axis'},grid:{left:130,right:30,top:24,bottom:35},xAxis:{type:'value',min:0},yAxis:{type:'category',data:visible.map(row=>row.name)},series:[{type:'bar',data:visible.map(row=>row.value),itemStyle:{color:'#2563eb'}}]},true);}catch(error){this.$message.error(error.message||'Request failed');}finally{if(sequence===this.requestSequence)this.loading=false;}}}
};
</script>
<style scoped>.region-chart{height:440px;margin:24px 0}.region-analysis{min-width:0}.analysis-filter-row{display:flex;align-items:flex-end;flex-wrap:wrap;gap:16px}.analysis-filter-row label{display:grid;gap:8px;min-width:0}.analysis-filter-row label>span{font-size:13px;color:var(--hw-muted)}@media(max-width:600px){.analysis-filter-row label{width:100%}.region-chart{height:360px}}</style>
