<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button @click="handleAddSensitive"> {{ $t('page.sensitive.button_add_sensitive') }} </hw-button>
          <hw-button
            theme="danger"
            variant="outline"
            @click="handleBatchDelete"
            :disabled="selectedRowKeys.length === 0"
          >
            {{ $t('page.sensitive.button_batch_delete') }}
          </hw-button>
          <hw-button
            theme="danger"
            :disabled="data.length === 0"
            @click="handleClearAll"
          >
            {{ $t('page.sensitive.button_clear_all') }}
          </hw-button>
          <span v-if="selectedRowKeys.length > 0" class="selected-count">
            {{ $t('common.selected_count', { count: selectedRowKeys.length }) }}
          </span>
        </div>
        <div class="right-operation-container">
          <hw-form ref="form" :data="searchformData" :label-width="80" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.sensitive.label_content')" name="content">
              <hw-input v-model="searchformData.content" class="search-input" clearable>
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
      <help-block :summary="$t('page.sensitive.alert_message')" doc="guide/Sensitive" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :selected-row-keys="selectedRowKeys" :loading="dataLoading"
          @page-change="rehandlePageChange" @change="rehandleChange" @select-change="rehandleSelectChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">

          <template #host_code="{ row }">
            <span> {{host_dic[row.host_code]}}</span>
          </template>
          <template #check_direction="{ row }">
            <p>
              {{
                check_direction_type.find(option => option.value === row.check_direction)?.label || row.check_direction
              }}
            </p>
          </template>
          <template #action="{ row }">
            <p>
              {{
                action_type.find(option => option.value === row.action)?.label || row.action
              }}
            </p>
          </template>
          <template #op="slotProps">
            <a class="t-button-link" @click="handleClickEdit(slotProps)">{{ $t('common.edit') }}</a>
            <a class="t-button-link" @click="handleClickDelete(slotProps)">{{ $t('common.delete') }}</a>
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

          <hw-form-item :label="$t('page.sensitive.label_content')" name="content">
            <hw-input :style="{ width: '480px' }" v-model="formData.content"  ></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.sensitive.label_check_direction')" name="check_direction">
            <hw-select v-model="formData.check_direction" clearable :style="{ width: '480px' }">
              <hw-option v-for="item in check_direction_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('page.sensitive.label_action')" name="action">
            <hw-select v-model="formData.action" clearable :style="{ width: '480px' }">
              <hw-option v-for="item in action_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: '480px' }" v-model="formData.remarks"   name="remarks">
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
         <hw-form-item :label="$t('page.sensitive.label_content')" name="content">
           <hw-input :style="{ width: '480px' }" v-model="formEditData.content" ></hw-input>
         </hw-form-item>
          <hw-form-item :label="$t('page.sensitive.label_check_direction')" name="check_direction">
            <hw-select v-model="formEditData.check_direction" clearable :style="{ width: '480px' }">
              <hw-option v-for="item in check_direction_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('page.sensitive.label_action')" name="action">
            <hw-select v-model="formEditData.action" clearable :style="{ width: '480px' }">
              <hw-option v-for="item in action_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: '480px' }" v-model="formEditData.remarks" name="remarks">
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

    <!-- 批量删除确认对话框 -->
    <hw-dialog
      :header="$t('page.sensitive.button_batch_delete')"
      :body="$t('page.sensitive.confirm_batch_delete')"
      v-model:visible="batchDeleteVisible"
      @confirm="onConfirmBatchDelete"
      :onCancel="onCancelBatchDelete">
    </hw-dialog>

    <!-- 清空所有确认对话框 -->
    <hw-dialog
      :header="$t('page.sensitive.button_clear_all')"
      :body="$t('page.sensitive.confirm_clear_all')"
      v-model:visible="clearAllVisible"
      @confirm="onConfirmClearAll"
      :onCancel="onCancelClearAll">
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
    wafSensitiveListApi,  wafSensitiveDelApi,wafSensitiveEditApi,wafSensitiveAddApi,wafSensitiveDetailApi,
    wafSensitiveBatchDelApi, wafSensitiveDelAllApi
  } from '@/apis/sensitive';

  const INITIAL_DATA = {
    content: '',
    check_direction: 'in',
    action: 'deny',
    remarks: '',
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
          type: [{
            required: true,
            message: this.$t('common.select_placeholder')+this.$t('page.sensitive.label_type'),
            type: 'error'
          }],
          content: [{
            required: true,
            message: this.$t('common.select_placeholder')+this.$t('page.sensitive.label_content'),
            type: 'error'
          }],
        },
        type_options: [{
            label: this.$t('page.sensitive.type_option_0'),
            value: '0'
          },
          {
            label: this.$t('page.sensitive.type_option_1'),
            value: '1'
          },
          {
            label: this.$t('page.sensitive.type_option_2'),
            value: '2'
          },
          {
            label: this.$t('page.sensitive.type_option_3'),
            value: '3'
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
            title: this.$t('page.sensitive.label_content'),
            align: 'left',
            width: 250,
            ellipsis: true,
            colKey: 'content',
          },
          {
            title: this.$t('page.sensitive.label_check_direction'),
            align: 'left',
            width: 100,
            ellipsis: true,
            colKey: 'check_direction',
          },
          {
            title: this.$t('page.sensitive.label_action'),
            align: 'left',
            width: 100,
            ellipsis: true,
            colKey: 'action',
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
          type:"",
          content:"",
          remarks: "",
        },
        //索引区域
        deleteIdx: -1,
        guardStatusIdx :-1,

        //检测方向
        check_direction_type: [{
          label: this.$t('page.sensitive.check_direction_type.in'),
          value: 'in'
        },
          {
            label: this.$t('page.sensitive.check_direction_type.out'),
            value: 'out'
          },
          {
            label: this.$t('page.sensitive.check_direction_type.all'),
            value: 'all'
          },
        ],
        //执行类型
        action_type: [{
          label: this.$t('page.sensitive.action_type.deny'),
          value: 'deny'
        },
          {
            label: this.$t('page.sensitive.action_type.replace'),
            value: 'replace'
          },
        ],
        //end option
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
      this.getList("")
    },

    methods: {
      getList(keyword) {
        let that = this
        that.searchformData['type'] = Number(that.searchformData['type'])
        console.log(that.searchformData)
        wafSensitiveListApi({
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
      handleAddSensitive() {
        this.addFormVisible = true
        this.formData =  { ...INITIAL_DATA };
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
          postdata['type'] = Number(postdata['type'])
          wafSensitiveAddApi( {
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
          postdata['type'] = Number(postdata['type'])
          wafSensitiveEditApi( {
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
        wafSensitiveDelApi({
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
        wafSensitiveDetailApi({
              id: id
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {
              that.detail_data = resdata.data;
              that.formEditData = {
                ...that.detail_data
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
          this.$message.warning(this.$t('page.sensitive.no_data_selected'));
          return;
        }
        this.batchDeleteVisible = true;
      },

      handleClearAll() {
        this.clearAllVisible = true;
      },

      onConfirmBatchDelete() {
        this.batchDeleteVisible = false;
        const ids = this.selectedRowKeys.map(key => {
          const item = this.data.find(d => d[this.rowKey] === key);
          return item ? item.id : null;
        }).filter(id => id !== null);

        if (ids.length === 0) {
          this.$message.warning(this.$t('page.sensitive.no_data_selected'));
          return;
        }

        wafSensitiveBatchDelApi({ ids })
          .then((res) => {
            if (res.code === 0) {
              this.$message.success(this.$t('page.sensitive.batch_delete_success'));
              this.selectedRowKeys = [];
              this.getList("");
            } else {
              this.$message.warning(res.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
            this.$message.error(this.$t('common.operation_failed'));
          });
      },

      onConfirmClearAll() {
        this.clearAllVisible = false;

        wafSensitiveDelAllApi({})
          .then((res) => {
            if (res.code === 0) {
              this.$message.success(this.$t('page.sensitive.clear_all_success'));
              this.selectedRowKeys = [];
              this.getList("");
            } else {
              this.$message.warning(res.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
            this.$message.error(this.$t('common.operation_failed'));
          });
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
