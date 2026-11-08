<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="detail-advanced">
    <hw-card title="基本信息" class="advanced-card">
      <div class="info-block">
        <div v-for="(item, index) in baseInfoData" :key="index" class="info-item">
          <h1>{{ item.name }}</h1>
          <span
            :class="{
              ['inProgress']: item.type && item.type.value === 'inProgress',
              ['pdf']: item.type && item.type.value === 'pdf',
            }"
          >
            <i v-if="item.type && item.type.key === 'contractStatus'"></i>
            {{ item.value }}
          </span>
        </div>
      </div>
    </hw-card>

    <!-- 发票进度 -->
    <hw-card title="发票进度" class="container-base-margin-top">
      <hw-row :class="prefix + '-operator-row row-padding'" justify="space-between">
        <template>
          <hw-steps :current="updateCurrent">
            <hw-step-item title="申请提交" content="已于12月21日提交"></hw-step-item>
            <hw-step-item title="电子发票" content="预计1～3个工作日"></hw-step-item>
            <hw-step-item title="发票已邮寄" content="电子发票开出后7个工作日内联系"></hw-step-item>
            <hw-step-item title="完成" content></hw-step-item>
          </hw-steps>
        </template>
      </hw-row>
    </hw-card>

    <!-- 产品目录 -->
    <hw-card title="产品目录" class="container-base-margin-top">
      <template #option  >
        <hw-radio-group default-value="dateVal" @change="onAlertChange">
          <hw-radio-button value="dateVal">季度</hw-radio-button>
          <hw-radio-button value="monthVal">月份</hw-radio-button>
        </hw-radio-group>
      </template>

      <hw-row :gutter="16" class="product-block-container">
        <hw-col :xl="4">
          <div class="product-add">
            <div class="product-sub">
              <add-icon class="product-sub-icon" />
              <span>新增产品</span>
            </div>
          </div>
        </hw-col>
        <hw-col v-for="(item, index) in PRODUCT_LIST" :key="index" :xl="4">
          <product :data="item" />
        </hw-col>
      </hw-row>
    </hw-card>

    <!-- 产品采购明细 -->
    <hw-card title="产品采购明细" class="container-base-margin-top">
      <hw-table
        :columns="columns"
        :data="data"
        :pagination="pagination"
        :hover="hover"
        size="large"
        row-key="index"
        @sort-change="sortChange"
        @change="rehandleChange"
      >
        <template #pdName="{ row }">
          <span>
            {{ row.pdName }}
            <hw-tag v-if="row.pdType" size="small">{{ row.pdType }}</hw-tag>
          </span>
        </template>

        <template #purchaseNum="{ row }">
          <span>
            {{ row.purchaseNum }}
            <hw-tag v-if="row.purchaseNum > 50" theme="danger" variant="light" size="small">超预算</hw-tag>
          </span>
        </template>

        <template #op="slotProps">
          <a class="t-button-link" @click="listClick(slotProps)">管理</a>
          <a class="t-button-link" @click="deleteClickOp(slotProps)">删除</a>
        </template>
        <template #op-column><order-descending-icon  /></template>
      </hw-table>
    </hw-card>

    <hw-dialog header="基本信息" v-model:visible="visible" @confirm="onConfirm">
      <template #body><div >
        <div class="dialog-info-block">
          <div v-for="(item, index) in baseInfoData" :key="index" class="info-item">
            <h1>{{ item.name }}</h1>
            <span
              :class="{
                ['inProgress']: item.type && item.type.value === 'inProgress',
                ['pdf']: item.type && item.type.value === 'pdf',
              }"
            >
              <i v-if="item.type && item.type.key === 'contractStatus'"></i>
              {{ item.value }}
            </span>
          </div>
        </div>
      </div></template>
    </hw-dialog>
  </div>
</template>
<script lang="ts">
import { TableChangeContext, TableChangeData, TableSort } from '@/ui';
import { OrderDescendingIcon, AddIcon } from '@/ui/icons';
import { prefix } from '@/config/global';
import model from '@/service/service-advance';
import Product from './components/Product.vue';

const PRODUCT_LIST = [
  {
    name: 'MacBook Pro 2021',
    subTitle: 'MacBook Pro 2021',
    size: '13.3 英寸',
    cpu: 'Apple M1',
    memory: 'RAM 16GB',
    info: '最高可选配 16GB 内存 · 最高可选配 2TB 存储设备 电池续航最长达 18 小时',
    use: 1420,
    stock: 1500,
  },
  {
    name: 'Surface Laptop Go',
    subTitle: '微软（Microsoft Corporation）',
    size: '12.4 英寸',
    cpu: 'Core i7',
    memory: 'RAM 16GB',
    info: '常规使用 Surface，续航时间最长可达13小时 随时伴您工作',
    use: 120,
    stock: 2000,
  },
];

/** 高级详情 */
export default {
  name: 'DetailAdvanced',
  components: { Product, OrderDescendingIcon, AddIcon },
  data() {
    return {
      data: [],
      prefix,
      updateCurrent: 0,
      columns: model.getTableColumns(),
      baseInfoData: model.getBaseInfoData(),
      bordered: true,
      hover: true,
      visible: false,
      pagination: {
        defaultPageSize: 10,
        total: 100,
        defaultCurrent: 1,
        pageSizeOptions: [],
      },
      PRODUCT_LIST,
    };
  },
  async mounted() {
    // 模拟进度
    setInterval(() => {
      if (this.updateCurrent > 5) {
        this.updateCurrent = -1;
      }
      this.updateCurrent += 1;
    }, 2000);

    this.$request
      .get('/api/get-purchase-list')
      .then((res: { code: number; data: Record<string, any> }) => {
        if (res.code === 0) {
          const { list = [] } = res.data;
          this.data = list;
          this.pagination = {
            ...this.pagination,
            total: list.length,
          };
        }
      })
      .catch((e: Error) => {
        console.log(e);
      });
  },
  methods: {
    onAlertChange(val: string): void {
      console.log(val);
    },
    sortChange(val: TableSort): void {
      console.log(val);
    },
    rehandleChange(
      changeParams: TableChangeData,
      triggerAndData: TableChangeContext,
    ): void {
      console.log('统一Change', changeParams, triggerAndData);
    },
    listClick(e: any): void {
      console.log(e);
      this.visible = true;
    },
    deleteClickOp(e: any): void {
      console.log(e, 'e');
      this.data.splice(e.rowIndex, 1);
    },
    onConfirm(): void {
      this.visible = false;
    },
  },
};
</script>
<style lang="less" scoped>
@import './index';
</style>
