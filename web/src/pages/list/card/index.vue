<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="list-card">
    <!-- 搜索区域 -->
    <div class="list-card-operation">
      <hw-button @click="formVisible = true">新建产品</hw-button>
      <hw-input v-model="searchValue" class="search-input" placeholder="请输入你需要搜索的内容" clearable>
        <template #suffix-icon>
          <search-icon v-if="searchValue === ''" size="20px" />
        </template>
      </hw-input>
    </div>
    <!-- 卡片列表 -->
    <template v-if="pagination.total > 0 && !dataLoading">
      <div class="list-card-items">
        <hw-row :gutter="[16, 16]">
          <hw-col
            :lg="4"
            :xs="6"
            :xl="3"
            v-for="product in productList.slice(
              pagination.pageSize * (pagination.current - 1),
              pagination.pageSize * pagination.current,
            )"
            :key="product.index"
          >
            <product-card :product="product" @delete-item="handleDeleteItem" @manage-product="handleManageProduct" />
          </hw-col>
        </hw-row>
      </div>
      <div class="list-card-pagination">
        <hw-pagination
          v-model="pagination.current"
          :total="pagination.total"
          :pageSizeOptions="[12, 24, 36]"
          v-model:page-size="pagination.pageSize"
          @page-size-change="onPageSizeChange"
          @current-change="onCurrentChange"
        />
      </div>
    </template>
    <div v-else-if="dataLoading" class="list-card-loading">
      <hw-loading text="加载中..."></hw-loading>
    </div>
    <!-- 产品管理弹窗 -->
    <hw-dialog header="新建产品" v-model:visible="formVisible" :width="680" :footer="false">
      <template #body><div >
        <!-- 表单内容 -->
        <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="100">
          <hw-form-item label="产品名称" name="name">
            <hw-input :style="{ width: '480px' }" v-model="formData.name" placeholder="请输入产品名称"></hw-input>
          </hw-form-item>
          <hw-form-item label="产品状态" name="status">
            <hw-radio-group v-model="formData.status">
              <hw-radio value="0">已停用</hw-radio>
              <hw-radio value="1">已启用</hw-radio>
            </hw-radio-group>
          </hw-form-item>
          <hw-form-item label="产品描述" name="description">
            <hw-input :style="{ width: '480px' }" v-model="formData.description" placeholder="请输入产品描述"></hw-input>
          </hw-form-item>
          <hw-form-item label="产品类型" name="type">
            <hw-select v-model="formData.type" clearable :style="{ width: '480px' }">
              <hw-option v-for="(item, index) in options" :value="item.value" :label="item.label" :key="index">
                {{ item.label }}
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item label="备注" name="mark">
            <hw-textarea :style="{ width: '480px' }" v-model="textareaValue" placeholder="请输入内容" name="description">
            </hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseBtn">取消</hw-button>
            <hw-button theme="primary" type="submit">确定</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>
    <!-- 删除操作弹窗 -->
    <hw-dialog
      header="确认删除所选产品？"
      :body="confirmBody"
      v-model:visible="confirmVisible"
      @confirm="onConfirmDelete"
      :onCancel="onCancel"
    >
    </hw-dialog>
  </div>
</template>
<script lang="ts">
import { prefix } from '@/config/global';
import { SearchIcon } from '@/ui/icons';
import ProductCard from '@/components/product-card/index.vue';

const INITIAL_DATA = {
  name: '',
  status: '',
  description: '',
  type: '',
  mark: '',
  amount: 0,
};

export default {
  name: 'ListCard',
  components: {
    SearchIcon,
    ProductCard,
  },
  data() {
    return {
      pagination: { current: 1, pageSize: 12, total: 0 },
      prefix,
      productList: [],
      value: 'first',
      rowKey: 'index',
      tableLayout: 'auto',
      verticalAlign: 'top',
      bordered: true,
      hover: true,
      rowClassName: (rowKey) => `${rowKey}-class`,
      formData: { ...INITIAL_DATA },
      options: [
        { label: '网关', value: '1' },
        { label: '人工智能', value: '2' },
        { label: 'CVM', value: '3' },
      ],
      formVisible: false,
      textareaValue: '',
      rules: {
        name: [{ required: true, message: '请输入产品名称', type: 'error' }],
      },
      searchValue: '',
      confirmVisible: false, // 控制确认弹窗
      deleteProduct: undefined,
      dataLoading: false,
    };
  },
  computed: {
    confirmBody(): string {
      const { deleteProduct } = this;
      return deleteProduct ? `删除后，${deleteProduct.name}的所有产品信息将被清空, 且无法恢复` : '';
    },
  },
  mounted() {
    this.dataLoading = true;
    this.$request
      .get('/api/get-card-list')
      .then((res) => {
        if (res.code === 0) {
          const { list = [] } = res.data;
          this.productList = list;
          this.pagination = {
            ...this.pagination,
            total: list.length,
          };
        }
      })
      .catch((e: Error) => {
        console.log(e);
      })
      .finally(() => {
        this.dataLoading = false;
      });
  },
  methods: {
    onPageSizeChange(size: number): void {
      this.pagination.pageSize = size;
      this.pagination.current = 1;
    },
    onCurrentChange(current: number): void {
      this.pagination.current = current;
    },
    onSubmit({ result, firstError }): void {
      if (!firstError) {
        this.$message.success('提交成功');
        this.formVisible = false;
      } else {
        console.log('Errors: ', result);
        this.$message.warning(firstError);
      }
    },
    onClickCloseBtn(): void {
      this.formVisible = false;
      this.formData = {};
    },
    handleDeleteItem(product): void {
      this.confirmVisible = true;
      this.deleteProduct = product;
    },
    onConfirmDelete(): void {
      const { index } = this.deleteProduct;
      this.productList.splice(index - 1, 1);
      this.confirmVisible = false;
      this.$message.success('删除成功');
    },
    onCancel(): void {
      this.deleteProduct = undefined;
      this.formData = {};
    },
    handleManageProduct(product): void {
      this.formVisible = true;
      this.formData = { ...product, status: product?.isSetup ? '1' : '0' };
    },
  },
};
</script>
<style scoped lang="less">
.list-card-operation {
  display: flex;
  justify-content: space-between;

  .search-input {
    width: 360px;
  }
}

.list-card-items {
  margin: 14px 0 24px 0;
}
</style>
