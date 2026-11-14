<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-tabs v-model="activeTab">
        <hw-tab-panel value="port" :label="$t('page.one_key_mod.tab_port_modify')">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-form :label-width="300" colon layout="inline"  :style="{ marginBottom: '8px' }">
            <hw-form-item>
              {{ $t('page.one_key_mod.one_key_placeholder')}}
            </hw-form-item>
            <hw-form-item  :label="$t('page.one_key_mod.baota_placeholder')" >
              <hw-input v-model="defaultFilePath" class="search-input" :placeholder="$t('page.one_key_mod.baota_placeholder')" clearable>
              </hw-input>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="danger" @click="handleOneKeyModify"> {{ $t('page.one_key_mod.button_one_key_modify') }} </hw-button>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList('all')"> {{ $t('common.search') }} </hw-button>

            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>

      <help-block :summary="$t('page.one_key_mod.modify_logs')" doc="guide/OneKeyMod" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :selected-row-keys="selectedRowKeys" :loading="dataLoading"
          @page-change="rehandlePageChange" @change="rehandleChange" @select-change="rehandleSelectChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">

          <template #is_restore="slotProps">
            <span>{{ slotProps.row.is_restore === 0 ? $t('page.one_key_mod.label_is_restore_0') : $t('page.one_key_mod.label_is_restore_1') }}</span>
          </template>
          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleClickEdit(slotProps)">{{$t('common.details')}}</button>
            <button type="button" class="t-button-link" @click="handleClickDelete(slotProps)">{{$t('common.delete')}}</button>
            <button type="button" class="t-button-link" v-if="slotProps.row.is_restore === 0" @click="handleOneKeyRestore(slotProps.row.id)">{{$t('common.restore')}}</button>
          </template>
        </hw-table>
      </div>
      <div>
      <router-view></router-view>
      </div>
        </hw-tab-panel>
        <hw-tab-panel value="import" :label="$t('page.one_key_mod.tab_import_nginx')">
          <import-nginx />
        </hw-tab-panel>
      </hw-tabs>
    </hw-card>


    <hw-dialog :header="$t('common.details')" v-model:visible="editFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="form" :labelWidth="100">
          <hw-form-item :label="$t('page.one_key_mod.label_op_system')" name="op_system">
           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.op_system" placeholder=""></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.one_key_mod.label_file_path')" name="file_path">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.file_path" placeholder=""></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.one_key_mod.label_before_content')" name="before_content">
            <hw-textarea :style="{ width: 'min(100%, 480px)' }"  :autosize="{ minRows: 5, maxRows: 10 }"  v-model="formEditData.before_content" name="before_content">
            </hw-textarea>
          </hw-form-item>
          <hw-form-item :label="$t('page.one_key_mod.label_after_content')" name="after_content">
            <hw-textarea :style="{ width: 'min(100%, 480px)' }" :autosize="{ minRows: 5, maxRows: 10 }"  v-model="formEditData.after_content"  name="after_content">
            </hw-textarea>
          </hw-form-item>
          <hw-form-item :label="$t('page.one_key_mod.label_is_restore')" name="is_restore">
            <hw-select :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.is_restore" placeholder="">
              <hw-option value="0">{{ $t('page.one_key_mod.label_is_restore_0') }}</hw-option>
              <hw-option value="1">{{ $t('page.one_key_mod.label_is_restore_1') }}</hw-option>
            </hw-select>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseEditBtn">{{ $t('common.close') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <hw-dialog :header="$t('common.confirm_delete')" :body="confirmBody" v-model:visible="confirmVisible" @confirm="onConfirmDelete"
      :onCancel="onCancel">
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
    wafOneKeyModDelApi,wafOneKeyModDetailApi,wafOneKeyModListApi,wafDoOneKeyModApi,wafRestoreOneKeyModApi
  } from '@/apis/onekeymod';
  import ImportNginx from './components/ImportNginx.vue';
  import {
    SSL_STATUS,
    GUARD_STATUS,
    CONTRACT_STATUS,
    CONTRACT_STATUS_OPTIONS,
    CONTRACT_TYPES,
    CONTRACT_PAYMENT_TYPES
  } from '@/constants';

  const INITIAL_DATA = {

  };
  export default defineComponent({
    name: 'ListBase',
    components: {
      SearchIcon,
      Trend,
      ImportNginx,
    },
    data() {
      return {
        activeTab: 'port',
        defaultFilePath:"/www/server/panel/vhost/nginx",
        addFormVisible: false,
        editFormVisible: false,
        guardVisible: false,
        confirmVisible: false,
        formData: {
          ...INITIAL_DATA
        },
        formEditData: {
          ...INITIAL_DATA
        },
        rules: {
        },
        textareaValue: '',
        prefix,
        dataLoading: false,
        data: [], //列表数据信息
        detail_data: [], //加载详情信息用于编辑
        selectedRowKeys: [],
        value: 'first',
        columns: [
          {
            title: this.$t('page.one_key_mod.label_op_system'),
            align: 'left',
            width: 60,
            ellipsis: true,
            colKey: 'op_system',
          },
          {
            title: this.$t('page.one_key_mod.label_file_path'),
            width: 450,
            ellipsis: true,
            colKey: 'file_path',
          },
          {
            title: this.$t('page.one_key_mod.label_is_restore'),
            width: 60,
            ellipsis: true,
            colKey: 'is_restore',
          },
          {
            title: this.$t('common.create_time'),
            width: 130,
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
        rowKey: 'code',
        tableLayout: 'auto',
        verticalAlign: 'top',
        hover: true,
        rowClassName: (rowKey: string) => `${rowKey}-class`,
        // 与pagination对齐
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
      if (this.$route.query.tab === 'import') {
        this.activeTab = 'import'
      }
      this.getList("")
    },

    methods: {
      getList(keyword) {
        let that = this
        wafOneKeyModListApi( {
              pageSize: that.pagination.pageSize,
              pageIndex: that.pagination.current,
              ... that.searchformData
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
      onClickCloseBtn(): void {
        this.formVisible = false;
        this.formData = {};
      },
      onClickCloseEditBtn(): void {
        this.editFormVisible = false;
        this.formEditData = {};
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
        wafOneKeyModDelApi( {
              id: id
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {

              that.pagination.current = 1
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
        wafOneKeyModDetailApi( {
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
      //跳转界面
      //一键修改
      handleOneKeyModify(){
        let that = this
        wafDoOneKeyModApi( {
          file_path:that.defaultFilePath
        })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {
              //1秒后刷新列表
              setTimeout(() => {
                that.getList("")
              }, 1000);
              that.$message.success(resdata.msg);
            }else{
              that.$message.warning(resdata.msg);
            }
          })
          .catch((e: Error) => {
            console.log(e);
          })
          .finally(() => {});
      },
      //还原
      handleOneKeyRestore(id) {
        const confirmDia = this.$dialog.confirm({
          header: this.$t('page.one_key_mod.restore_confirm_title') || '确认执行还原',
          body: this.$t('page.one_key_mod.restore_confirm_content') || '该操作将回滚修改，是否继续？',
          confirmBtn: {
            theme: 'danger',
            content: this.$t('common.confirm'),
          },
          cancelBtn: {
            theme: 'default',
            content: this.$t('common.cancel'),
          },
          onConfirm: () => {
            this.doOneKeyRestore(id);
            confirmDia.destroy();
          },
        });
      },
      //新增：实际执行还原
      doOneKeyRestore(id) {
        let that = this
        wafRestoreOneKeyModApi({
          id: id
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
