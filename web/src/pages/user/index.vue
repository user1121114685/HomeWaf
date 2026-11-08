<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-row :gutter="[16, 16]">
    <hw-col :flex="3">
      <div class="user-left-greeting">
        <div>
          Hi，Image
          <span class="regular"> 下午好，今天是你加入鹅厂的第 100 天～</span>
        </div>
        <img src="@/assets/assets-tencent-logo.svg?url" class="logo" />
      </div>

      <hw-card class="user-info-list" title="个人信息">
        <template #option>
          <hw-button theme="default" shape="square" variant="text">
            <edit-icon size="18" />
          </hw-button>
        </template>
        <hw-row class="content" justify="space-between">
          <hw-col v-for="(item, index) in USER_INFO_LIST" :key="index" class="contract" :span="item.span || 3">
            <div class="contract-title">
              {{ item.title }}
            </div>
            <div class="contract-detail">
              {{ item.content }}
            </div>
          </hw-col>
        </hw-row>
      </hw-card>

      <hw-card class="content-container">
        <hw-tabs value="second">
          <hw-tab-panel value="first" label="内容列表">
            <p>内容列表</p>
          </hw-tab-panel>
          <hw-tab-panel value="second" label="内容列表">
            <hw-card :bordered="false" title="主页访问数据" subtitle="（次）">
              <template #options>
                <hw-date-rang-picker
                  class="card-date-picker-container"
                  :default-value="LAST_7_DAYS"
                  theme="primary"
                  mode="date"
                  @change="onLineChange"
                />
              </template>
              <div id="lineContainer" style="width: 100%; height: 330px" />
            </hw-card>
          </hw-tab-panel>
          <hw-tab-panel value="third" label="内容列表">
            <p>内容列表</p>
          </hw-tab-panel>
        </hw-tabs>
      </hw-card>
    </hw-col>

    <hw-col :flex="1">
      <hw-card class="user-intro">
        <hw-avatar size="90px">T</hw-avatar>
        <div class="name">My Account</div>
        <div class="position">XXG 港澳业务拓展组员工 直客销售</div>
      </hw-card>

      <hw-card title="团队成员" class="user-team">
        <template #option>
          <hw-button theme="default" shape="square" variant="text">
            <edit-icon size="18" />
          </hw-button>
        </template>
        <hw-list :split="false">
          <hw-list-item v-for="(item, index) in TEAM_MEMBERS" :key="index">
            <hw-list-item-meta :image="item.avatar" :title="item.title" :description="item.description" />
          </hw-list-item>
        </hw-list>
      </hw-card>

      <hw-card title="服务产品" class="product-container">
        <template #option>
          <hw-button theme="default" shape="square" variant="text">
            <edit-icon size="18" />
          </hw-button>
        </template>
        <hw-row class="content" :getters="16">
          <hw-col :span="3">
            <product-a-icon />
          </hw-col>
          <hw-col :span="3">
            <product-b-icon />
          </hw-col>
          <hw-col :span="3">
            <product-c-icon />
          </hw-col>
          <hw-col :span="3">
            <product-d-icon />
          </hw-col>
        </hw-row>
      </hw-card>
    </hw-col>
  </hw-row>
</template>
<script>
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components';
import { LineChart } from 'echarts/charts';
import { CanvasRenderer } from 'echarts/renderers';
import * as echarts from 'echarts/core';
import { EditIcon } from '@/ui/icons';

import { mapState } from '@/store/helpers';

import { getFolderLineDataSet } from './index';
import { changeChartsTheme } from '@/utils/color';
import { LAST_7_DAYS } from '@/utils/date';

import { USER_INFO_LIST, TEAM_MEMBERS, PRODUCT_LIST } from '@/service/service-user';
import ProductAIcon from '@/assets/assets-product-1.svg';
import ProductBIcon from '@/assets/assets-product-2.svg';
import ProductCIcon from '@/assets/assets-product-3.svg';
import ProductDIcon from '@/assets/assets-product-4.svg';

echarts.use([GridComponent, TooltipComponent, LineChart, CanvasRenderer, LegendComponent]);

export default {
  name: 'UserIndex',

  components: {
    ProductAIcon,
    ProductBIcon,
    ProductCIcon,
    ProductDIcon,
    EditIcon,
  },
  data() {
    return {
      dashboardBase: '',
      lineContainer: '',
      lineChart: '',
      LAST_7_DAYS,
      USER_INFO_LIST,
      TEAM_MEMBERS,
      PRODUCT_LIST,
    };
  },
  computed: {
    ...mapState('setting', ['brandTheme', 'mode']),
  },
  watch: {
    brandTheme() {
      changeChartsTheme([this.lineChart]);
    },
    mode() {
      this.renderCharts();
    },
    lineChart() {
      this.updateContainer();
    },
  },
  mounted() {
    window.addEventListener('resize', this.updateContainer, false);
    this.renderCharts();
    this.$nextTick(() => {
      this.updateContainer();
    });
  },
  methods: {
    /** 图表选择 */
    onLineChange(value) {
      this.lineChart.setOption(getFolderLineDataSet({ dateTime: value }));
    },
    updateContainer() {
      this.lineChart.resize?.({
        width: this.lineContainer.clientWidth,
        height: this.lineContainer.clientHeight,
      });
    },
    renderCharts() {
      const { chartColors } = this.$store.state.setting;
      if (!this.lineContainer) {
        this.lineContainer = document.getElementById('lineContainer');
      }
      this.lineChart = echarts.init(this.lineContainer);
      this.lineChart.setOption({
        grid: {
          x: 30, // 默认是80px
          y: 30, // 默认是60px
          x2: 10, // 默认80px
          y2: 30, // 默认60px
        },
        ...getFolderLineDataSet({ ...chartColors }),
      });
    },
    getIcon(type) {
      const typeMap = {
        a: ProductAIcon,
        b: ProductBIcon,
        c: ProductCIcon,
        d: ProductDIcon,
      };
      return typeMap[type || 'a'];
    },
  },
};
</script>
<style lang="less" scoped>
@import url('./index.less');
</style>
