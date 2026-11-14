<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button @click="handleAddUrlWhite"> {{ $t('page.urlallow.button_add_url') }} </hw-button>
          <hw-button
            theme="danger"
            variant="outline"
            @click="handleBatchDelete"
            :disabled="selectedRowKeys.length === 0"
          >
            {{ $t('page.urlallow.button_batch_delete') }}
          </hw-button>
          <hw-button
            theme="danger"
            :disabled="data.length === 0"
            @click="handleClearAll"
          >
            {{ $t('page.urlallow.button_clear_all') }}
          </hw-button>
        </div>
        <div class="right-operation-container">
          <hw-form ref="form" :data="searchformData" :label-width="80" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.urlallow.label_website')" name="host_code">
              <hw-select v-model="searchformData.host_code" clearable  filterable :style="{ width: '150px' }">
                <hw-option v-for="(item, index) in host_dic" :value="index" :label="item" :key="index">
                  {{ item }}
                </hw-option>
              </hw-select>
            </hw-form-item>
            <hw-form-item :label="$t('page.urlallow.label_url')" name="url">
              <hw-input v-model="searchformData.url" class="search-input" clearable>
              </hw-input>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList('all')">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.urlallow.alert_message')" doc="guide/UrlWhite" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :selected-row-keys="selectedRowKeys" :loading="dataLoading"
          @page-change="rehandlePageChange" @change="rehandleChange" @select-change="rehandleSelectChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">

          <template #host_code="{ row }">
            <span> {{host_dic[row.host_code]}}</span>
          </template>

          <template #is_enable="{ row }">
            <hw-switch :value="row.is_enable === 1" @change="(v) => onToggle(row, v)" />
          </template>
          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleClickEdit(slotProps)">{{ $t('common.edit') }}</button>
            <button type="button" class="t-button-link" @click="handleClickDelete(slotProps)">{{ $t('common.delete') }}</button>
          </template>
        </hw-table>
      </div>
      <div>
      <router-view></router-view>
      </div>
    </hw-card>


    <hw-dialog :header="$t('common.new')" v-model:visible="addFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="100">
          <hw-form-item :label="$t('page.urlallow.label_website')" name="host_code">
            <hw-select v-model="formData.host_code" clearable filterable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in host_dic" :value="index" :label="item"
                :key="index">
                {{ item }}
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('page.urlallow.label_compare_type')" name="compare_type">
            <hw-select v-model="formData.compare_type" clearable filterable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in compare_type_options" :value="item.value" :label="item.label"
                :key="index">
                {{ item.label }}
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('page.urlallow.label_url')" name="url">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.url"  ></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.urlallow.col_enable')" name="is_enable">
            <hw-switch v-model="formData.is_enable" :custom-value="[1, 0]" />
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: 'min(100%, 480px)' }" v-model="formData.remarks"   name="remarks">
            </hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>


    <hw-dialog :header="$t('common.edit') " v-model:visible="editFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="form" :rules="rules" @submit="onSubmitEdit" :labelWidth="100">
          <hw-form-item :label="$t('page.urlallow.label_website')" name="host_code">
            <hw-select v-model="formEditData.host_code" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in host_dic" :value="index" :label="item"
                :key="index">
                {{ item }}
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('page.urlallow.label_compare_type')" name="compare_type">
            <hw-select v-model="formEditData.compare_type" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in compare_type_options" :value="item.value" :label="item.label"
                :key="index">
                {{ item.label }}
              </hw-option>
            </hw-select>
          </hw-form-item>
         <hw-form-item :label="$t('page.urlallow.label_url')" name="url">
           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.url" ></hw-input>
         </hw-form-item>
          <hw-form-item :label="$t('page.urlallow.col_enable')" name="is_enable">
            <hw-switch v-model="formEditData.is_enable" :custom-value="[1, 0]" />
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.remarks" name="remarks">
            </hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseEditBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <hw-dialog :header="$t('common.confirm_delete')" :body="confirmBody" v-model:visible="confirmVisible" @confirm="onConfirmDelete"
      :onCancel="onCancel">
    </hw-dialog>
    <hw-dialog
      :header="$t('page.urlallow.confirm_batch_delete')"
      v-model:visible="batchDeleteVisible"
      @confirm="onConfirmBatchDelete"
      @cancel="onCancelBatchDelete"
    >
      <template #body><div >
        {{ $t('page.urlallow.confirm_batch_delete') }}
      </div></template>
    </hw-dialog>

    <hw-dialog
      :header="$t('page.urlallow.confirm_clear_all')"
      v-model:visible="clearAllVisible"
      @confirm="onConfirmClearAll"
      @cancel="onCancelClearAll"
    >
      <template #body><div >
        {{ $t('page.urlallow.confirm_clear_all') }}
      </div></template>
    </hw-dialog>
  </div>
</template>
<script lang="ts">
import {defineComponent} from 'vue';


  import {
    SearchIcon
  } from '@/ui/icons';
  import Trend from '@/components/trend/index.vue';
  import {
    prefix
  } from '@/config/global';
  import {
    allhost
  } from '@/apis/host';
  import {
    wafURLWhiteListApi,wafURLWhiteDelApi,wafURLWhiteEditApi,wafURLWhiteAddApi,wafURLWhiteDetailApi,
    wafURLWhiteBatchDelApi,wafURLWhiteDelAllApi,
    wafURLWhiteToggleApi
  } from '@/apis/urlwhite';
  import {
    SSL_STATUS,
    GUARD_STATUS,
    CONTRACT_STATUS,
    CONTRACT_STATUS_OPTIONS,
    CONTRACT_TYPES,
    CONTRACT_PAYMENT_TYPES
  } from '@/constants';

  const INITIAL_DATA = {
    host_code: '',
    url: '',
    remarks: '',
    compare_type:"等于",
    is_enable: 1,
  };
  export default defineComponent({
    name: 'ListBase',
    components: {
      SearchIcon,
      Trend,
    },
    data() {
      return {
        addFormVisible: false,
        editFormVisible: false,
        guardVisible: false,
        confirmVisible: false,
        batchDeleteVisible: false,
        clearAllVisible: false,
        formData: {
          ...INITIAL_DATA
        },
        formEditData: {
          ...INITIAL_DATA
        },
        rules: {
          host_code: [{
            required: true,
            message: this.$t('common.select_placeholder')+this.$t('page.urlallow.label_website'),
            type: 'error'
          }],
          compare_type: [{
            required: true,
            message: this.$t('common.select_placeholder'),
            type: 'error'
          }],
          url: [{
            required: true,
            message: this.$t('common.select_placeholder')+this.$t('page.urlallow.label_url'),
            type: 'error'
          }],
        },
        compare_type_options: [{
            label: this.$t('page.urlallow.compare_type_option_equal'),
            value: '等于'
          },
          {
            label: this.$t('page.urlallow.compare_type_option_pre'),
            value: '前缀匹配'
          },
          {
            label: this.$t('page.urlallow.compare_type_option_end'),
            value: '后缀匹配'
          },
          {
            label: this.$t('page.urlallow.compare_type_option_contain'),
            value: '包含匹配'
          },
        ],
        textareaValue: '',
        prefix,
        dataLoading: false,
        data: [], //列表数据信息
        detail_data: [], //加载详情信息用于编辑
        selectedRowKeys: [],
        value: 'first',
        columns: [
          { colKey: 'row-select', type: 'multiple', width: 64, fixed: 'left' },
          {
            title: this.$t('page.urlallow.label_website'),
            align: 'left',
            width: 250,
            ellipsis: true,
            colKey: 'host_code',
          },{
            title: this.$t('page.urlallow.label_compare_type'),
            align: 'left',
            width: 250,
            ellipsis: true,
            colKey: 'compare_type',
          },
          {
            title: this.$t('page.urlallow.label_url'),
            width: 200,
            ellipsis: true,
            colKey: 'url',
          },
          {
            title: this.$t('common.remarks'),
            width: 200,
            ellipsis: true,
            colKey: 'remarks',
          },
          {
            title: this.$t('common.create_time'),
            width: 200,
            ellipsis: true,
            colKey: 'create_time',
          },
          {
            title: this.$t('page.urlallow.col_enable'),
            width: 90,
            colKey: 'is_enable',
          },
          {
            align: 'left',
            width: 200,
            colKey: 'op',
            title: this.$t('common.op'),
          },
        ],
        rowKey: 'id',
        tableLayout: 'auto',
        verticalAlign: 'top',
        hover: true,
        rowClassName: (rowKey: string) => `${rowKey}-class`,
        pagination: {
          total: 0,
          current: 1,
          pageSize: 10
        },
        //顶部搜索
        searchformData: {
          url:"",
          host_code:""
        },
        //索引区域
        deleteIdx: -1,
        guardStatusIdx :-1,
        //主机字典
        host_dic:{}
      };
    },
    computed: {
      confirmBody() {
        if (this.deleteIdx > -1) {
          const {
            url
          } = this.data?. [this.deleteIdx];
          return this.$t('common.confirm_delete');
        }
        return '';
      },
      offsetTop() {
        return this.$store.state.setting.isUseTabsRouter ? 48 : 0;
      },
    },
    mounted() {
      this.loadHostList().then(() => {
        this.getList("");
      });
    },

    methods: {
      loadHostList() {
        return new Promise((resolve, reject) => {
          allhost()
            .then((res) => {
              let resdata = res;
              console.log(resdata);
              if (resdata.code === 0) {
                let host_options = resdata.data;
                for (let i = 0; i < host_options.length; i++) {
                  this.host_dic[host_options[i].value] = host_options[i].label;
                }
              }
              resolve(undefined); // 调用 resolve 表示加载完成
            })
            .catch((e: Error) => {
              console.log(e);
              reject(e); // 调用 reject 表示加载失败
            });
        });
      },
      getList(keyword) {
        let that = this
        wafURLWhiteListApi({
              pageSize: that.pagination.pageSize,
              pageIndex: that.pagination.current,
              ...that.searchformData
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {

              this.data = resdata.data.list??[];
              this.data_attach = []
              console.log('getList',this.data)
              this.pagination = {
                ...this.pagination,
                total: resdata.data.total,
              };
            }
          })
          .catch((e: Error) => {
            console.log(e);
          })
          .finally(() => {
            this.dataLoading = false;
          });
        this.dataLoading = true;
      },
      onToggle(row, checked) {
        wafURLWhiteToggleApi({ id: row.id, is_enable: checked ? 1 : 0 })
          .then((res) => {
            if (res.code === 0) {
              this.$message.success(res.msg);
              this.getList("");
            } else {
              this.$message.warning(res.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
            //失败也重拉一次：服务端可能已翻转但响应丢失，让开关与库内真实状态对齐
            this.getList("");
          });
      },
      getContainer() {
        return document.querySelector('.tdesign-starter-layout');
      },
      rehandlePageChange(curr, pageInfo) {
        this.pagination.current = curr.current
        if (this.pagination.pageSize != curr.pageSize) {
          this.pagination.current = 1
          this.pagination.pageSize = curr.pageSize
        }
        this.getList("")
      },
      rehandleSelectChange(selectedRowKeys: number[]) {
        this.selectedRowKeys = selectedRowKeys;
      },
      rehandleChange(changeParams, triggerAndData) {
      },
      handleClickEdit(e) {
        console.log(e)
        const {
          id
        } = e.row
        console.log(id)
        this.editFormVisible = true
        this.getDetail(id)
      },
      handleAddUrlWhite() {
        this.addFormVisible = true
        this.formData =  {
          host_code: '',
          url: '',
          remarks: '',
        };
      },
      onSubmit({
        result,
        firstError
      }): void {
        let that = this
        if (!firstError) {

          let postdata = {
            ...that.formData
          }
          wafURLWhiteAddApi( {
              ...postdata
            })
            .then((res) => {
              let resdata = res
              console.log(resdata)
              if (resdata.code === 0) {
                that.$message.success(resdata.msg);
                that.addFormVisible = false;
                that.pagination.current = 1
                that.getList("")
              } else {
                that.$message.warning(resdata.msg);
              }
            })
            .catch((e: Error) => {
              console.log(e);
            })
            .finally(() => {});
        } else {
          console.log('Errors: ', result);
          that.$message.warning(firstError);
        }
      },
      onSubmitEdit({
        result,
        firstError
      }): void {
        let that = this
        if (!firstError) {

          let postdata = {
            ...that.formEditData
          }
          wafURLWhiteEditApi( {
              ...postdata
            })
            .then((res) => {
              let resdata = res
              console.log(resdata)
              if (resdata.code === 0) {
                that.$message.success(resdata.msg);
                that.editFormVisible = false;
                that.getList("")
              } else {
                that.$message.warning(resdata.msg);
              }
            })
            .catch((e: Error) => {
              console.log(e);
            })
            .finally(() => {});
        } else {
          console.log('Errors: ', result);
          that.$message.warning(firstError);
        }
      },
      onClickCloseBtn(): void {
        this.addFormVisible = false;
        this.formData = {...INITIAL_DATA};
      },
      onClickCloseEditBtn(): void {
        this.editFormVisible = false;
        this.formEditData = {...INITIAL_DATA};
      },
      handleClickDelete(row) {
        console.log(row)
        this.deleteIdx = row.rowIndex;
        this.confirmVisible = true;
      },
      onConfirmDelete() {
        this.confirmVisible = false;
        console.log('delete', this.data)
        console.log('delete', this.data[this.deleteIdx])
        let {
          id
        } = this.data[this.deleteIdx]
        let that = this
        wafURLWhiteDelApi({
              id: id,
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {

              that.getList("")
              that.$message.success(resdata.msg);
            } else {
              that.$message.warning(resdata.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
          })
          .finally(() => {});


        this.resetIdx();
      },
      onCancel() {
        this.resetIdx();
      },
      resetIdx() {
        this.deleteIdx = -1;
      },
      getDetail(id) {
        let that = this
        wafURLWhiteDetailApi({
              id: id
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {
              that.detail_data = resdata.data;
              that.formEditData = {
                ...that.detail_data,
                //详情未带开关字段时按启用处理
                is_enable: that.detail_data.is_enable === 0 ? 0 : 1,
              }
            }
          })
          .catch((e: Error) => {
            console.log(e);
          })
          .finally(() => {});
      },
      handleBatchDelete() {
        if (this.selectedRowKeys.length === 0) {
          this.$message.warning(this.$t('page.urlallow.no_data_selected'));
          return;
        }
        this.batchDeleteVisible = true;
      },

      handleClearAll() {
        this.clearAllVisible = true;
      },

      onConfirmBatchDelete() {
        this.batchDeleteVisible = false;
        const that = this;
        const ids = this.selectedRowKeys.map(key => {
          const item = this.data.find(d => d.id === key);
          return item ? item.id : null;
        }).filter(id => id !== null);

        wafURLWhiteBatchDelApi({ ids: ids })
          .then((res) => {
            const resdata = res;
            console.log(resdata);
            if (resdata.code === 0) {
              that.getList("");
              that.selectedRowKeys = [];
              that.$message.success(that.$t('page.urlallow.batch_delete_success'));
            } else {
              that.$message.warning(resdata.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
            that.$message.error('批量删除失败');
          })
          .finally(() => {});
      },

      onConfirmClearAll() {
        this.clearAllVisible = false;
        const that = this;

        wafURLWhiteDelAllApi({ host_code: this.searchformData.host_code })
          .then((res) => {
            const resdata = res;
            console.log(resdata);
            if (resdata.code === 0) {
              that.getList("");
              that.selectedRowKeys = [];
              that.$message.success(that.$t('page.urlallow.clear_all_success'));
            } else {
              that.$message.warning(resdata.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
            that.$message.error('清空失败');
          })
          .finally(() => {});
      },

      onCancelBatchDelete() {
        this.batchDeleteVisible = false;
      },

      onCancelClearAll() {
        this.clearAllVisible = false;
      },
    },
  });
</script>

<style lang="less" scoped>
  @import '@/style/variables';
  .left-operation-container {
    padding: 0 0 6px 0;
    margin-bottom: 16px;

    .selected-count {
      display: inline-block;
      margin-left: 8px;
      color: var(--td-text-color-secondary);
    }
  }

  .search-input {
    width: 360px;
  }

  .t-button+.t-button {
    margin-left: @spacer;
  }
</style>
