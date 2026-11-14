<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button @click="handleAdd"> {{ $t('page.cache_rule.button_add_cache_rule') }} </hw-button>
        </div>
        <div class="right-operation-container">
            <hw-form ref="form" :data="searchformData" :label-width="300" layout="inline" colon :style="{ marginBottom: '8px' }">
                   <hw-form-item>
                  <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList('all')">
                            {{ $t('common.search') }}
                          </hw-button>
                   </hw-form-item>
             </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.cache_rule.alert_message')" doc="guide/CacheRule" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :selected-row-keys="selectedRowKeys" :loading="dataLoading"
          @page-change="rehandlePageChange" @change="rehandleChange" @select-change="rehandleSelectChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">
          <template #host_code="{ row }">
            <span> {{host_dic[row.host_code]}}</span>
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
                     <hw-form-item :label="$t('page.cache_rule.host_code')" name="host_code">
                       <hw-select v-model="formData.host_code" clearable :style="{ width: 'min(100%, 480px)' }">
                         <hw-option v-for="(item, index) in host_dic" :value="index" :label="item"
                                   :key="index">
                           {{ item }}
                         </hw-option>
                       </hw-select>


                    </hw-form-item>

                      <hw-form-item :label="$t('page.cache_rule.rule_name')" name="rule_name">
                         <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.rule_name" ></hw-input>
                    </hw-form-item>

                     <hw-form-item :label="$t('page.cache_rule.rule_type')" name="rule_type">
                      <hw-select v-model="formData.rule_type" clearable :style="{ width: 'min(100%, 480px)' }">
                        <hw-option v-for="item in rule_type_options" :value="item.value" :label="item.label" :key="item.value">
                          {{ item.label }}
                        </hw-option>
                      </hw-select>
                      <div class="form-item-tips">{{ $t('page.cache_rule.rule_type_tips') }}</div>
                     </hw-form-item>

                      <hw-form-item :label="$t('page.cache_rule.rule_content')" name="rule_content">
                         <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.rule_content" :placeholder="$t('page.cache_rule.rule_content_placeholder')"></hw-input>
                         <div class="form-item-tips">{{ $t('page.cache_rule.rule_content_tips') }}</div>
                    </hw-form-item>

                     <hw-form-item :label="$t('page.cache_rule.param_type')" name="param_type">
                        <hw-select v-model="formData.param_type" clearable :style="{ width: 'min(100%, 480px)' }">
                          <hw-option v-for="item in param_type_options" :value="item.value" :label="item.label" :key="item.value">
                            {{ item.label }}
                          </hw-option>
                        </hw-select>
                        <div class="form-item-tips">{{ $t('page.cache_rule.param_type_tips') }}</div>
                     </hw-form-item>

                     <hw-form-item :label="$t('page.cache_rule.cache_time')" name="cache_time">
                         <hw-input-number :style="{ width: '150px' }" v-model="formData.cache_time" :placeholder="$t('common.placeholder')">
                          </hw-input-number>
                          <div class="form-item-tips">{{ $t('page.cache_rule.cache_time_tips') }}</div>
                     </hw-form-item>

                     <hw-form-item :label="$t('page.cache_rule.priority')" name="priority">
                         <hw-input-number :style="{ width: '150px' }" v-model="formData.priority" :placeholder="$t('common.placeholder')">
                          </hw-input-number>
                          <div class="form-item-tips">{{ $t('page.cache_rule.priority_tips') }}</div>
                     </hw-form-item>

                      <hw-form-item :label="$t('page.cache_rule.request_method')" name="request_method">
                         <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.request_method" :placeholder="$t('page.cache_rule.request_method_placeholder')"></hw-input>
                         <div class="form-item-tips">{{ $t('page.cache_rule.request_method_tips') }}</div>
                    </hw-form-item>

                      <hw-form-item :label="$t('page.cache_rule.remarks')" name="remarks">
                         <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remarks" ></hw-input>
                    </hw-form-item>

          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <hw-dialog :header="$t('common.edit')" v-model:visible="editFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="form" :rules="rules" @submit="onSubmitEdit" :labelWidth="100">

                        <hw-form-item :label="$t('page.cache_rule.host_code')" name="host_code">
                          <hw-select v-model="formEditData.host_code" clearable :style="{ width: 'min(100%, 480px)' }">
                            <hw-option v-for="(item, index) in host_dic" :value="index" :label="item"
                                      :key="index">
                              {{ item }}
                            </hw-option>
                          </hw-select>
                      </hw-form-item>

                        <hw-form-item :label="$t('page.cache_rule.rule_name')" name="rule_name">
                           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.rule_name" ></hw-input>
                      </hw-form-item>

                       <hw-form-item :label="$t('page.cache_rule.rule_type')" name="rule_type">
                        <hw-select v-model="formEditData.rule_type" clearable :style="{ width: 'min(100%, 480px)' }">
                          <hw-option v-for="item in rule_type_options" :value="item.value" :label="item.label" :key="item.value">
                            {{ item.label }}
                          </hw-option>
                        </hw-select>
                        <div class="form-item-tips">{{ $t('page.cache_rule.rule_type_tips') }}</div>
                       </hw-form-item>

                        <hw-form-item :label="$t('page.cache_rule.rule_content')" name="rule_content">
                           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.rule_content" :placeholder="$t('page.cache_rule.rule_content_placeholder')"></hw-input>
                           <div class="form-item-tips">{{ $t('page.cache_rule.rule_content_tips') }}</div>
                      </hw-form-item>

                       <hw-form-item :label="$t('page.cache_rule.param_type')" name="param_type">
                        <hw-select v-model="formEditData.param_type" clearable :style="{ width: 'min(100%, 480px)' }">
                          <hw-option v-for="item in param_type_options" :value="item.value" :label="item.label" :key="item.value">
                            {{ item.label }}
                          </hw-option>
                        </hw-select>
                        <div class="form-item-tips">{{ $t('page.cache_rule.param_type_tips') }}</div>
                       </hw-form-item>

                       <hw-form-item :label="$t('page.cache_rule.cache_time')" name="cache_time">
                           <hw-input-number :style="{ width: '150px' }" v-model="formEditData.cache_time" :placeholder="$t('common.placeholder')">
                            </hw-input-number>
                            <div class="form-item-tips">{{ $t('page.cache_rule.cache_time_tips') }}</div>
                       </hw-form-item>

                       <hw-form-item :label="$t('page.cache_rule.priority')" name="priority">
                           <hw-input-number :style="{ width: '150px' }" v-model="formEditData.priority" :placeholder="$t('common.placeholder')">
                            </hw-input-number>
                            <div class="form-item-tips">{{ $t('page.cache_rule.priority_tips') }}</div>
                       </hw-form-item>

                        <hw-form-item :label="$t('page.cache_rule.request_method')" name="request_method">
                           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.request_method" :placeholder="$t('page.cache_rule.request_method_placeholder')"></hw-input>
                           <div class="form-item-tips">{{ $t('page.cache_rule.request_method_tips') }}</div>
                      </hw-form-item>

                        <hw-form-item :label="$t('page.cache_rule.remarks')" name="remarks">
                           <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.remarks" ></hw-input>
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
    wafCacheRuleListApi,wafCacheRuleDelApi,wafCacheRuleEditApi,wafCacheRuleAddApi,wafCacheRuleDetailApi
  } from '@/apis/cache_rule.ts';

  const INITIAL_DATA = {



    host_code:'',
    rule_name:'',
    rule_type:'',
    rule_content:'',
    param_type:'',
    cache_time:'3600',
    priority:'1',
    request_method:'GET',
    remarks:'',

  };
  export default defineComponent({
    name: 'CacheRuleBase',
    props:{
      propHostCode: String,
    },
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
        formData: {
          ...INITIAL_DATA
        },
        formEditData: {
          ...INITIAL_DATA
        },
        rules: {

           host_code: [{
                  required: true,
                  message: this.$t('common.placeholder')+this.$t('page.cache_rule.host_code'),
                  type: 'error'
            }],

           rule_name: [{
                  required: true,
                  message: this.$t('common.placeholder')+this.$t('page.cache_rule.rule_name'),
                  type: 'error'
            }],
            rule_type: [{
                  required: true,
                  message: this.$t('common.placeholder')+this.$t('page.cache_rule.rule_type'),
                  type: 'error'
            }],
           param_type: [{
                  required: true,
                  message: this.$t('common.placeholder')+this.$t('page.cache_rule.param_type'),
                  type: 'error'
            }],
        },
        textareaValue: '',
        prefix,
        dataLoading: false,
        data: [], //列表数据信息
        detail_data: [], //加载详情信息用于编辑
        selectedRowKeys: [],
        value: 'first',
        columns: [


            { title: this.$t('page.cache_rule.rule_name'),
                    width: 200,
                    ellipsis: true,
                    colKey: 'rule_name',
             },

            { title: this.$t('page.cache_rule.rule_content'),
                    width: 200,
                    ellipsis: true,
                    colKey: 'rule_content',
             },
            { title: this.$t('page.cache_rule.cache_time'),
                    width: 100,
                    ellipsis: true,
                    colKey: 'cache_time',
             },

          {
            align: 'left',
            fixed: 'right',
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
          host_code:"",
        },
        //索引区域
        deleteIdx: -1,
        guardStatusIdx :-1,
        //主机字典
        host_dic:{},
        // 规则类型选项
        rule_type_options: [
          {
            label: this.$t('page.cache_rule.rule_type_options.suffix_match'),
            value: '1'
          },
          {
            label: this.$t('page.cache_rule.rule_type_options.specific_directory'),
            value: '2'
          },
          {
            label: this.$t('page.cache_rule.rule_type_options.specific_file'),
            value: '3'
          },
        ],
        // 参数处理选项
        param_type_options: [
          {
            label: this.$t('page.cache_rule.param_type_options.ignore_params'),
            value: '1'
          },
          {
            label: this.$t('page.cache_rule.param_type_options.full_params'),
            value: '2'
          },
        ],

      };//end data
    },
    watch:{
      propHostCode(newVal) {
        // 当 propHostCode 更新时，更新相应的数据
        this.searchformData.host_code = newVal;
        this.getList("")
      }
    },
    computed: {
      confirmBody() {
        if (this.deleteIdx > -1) {
          const {
            host
          } = this.data?. [this.deleteIdx];
          return this.$t('common.data_delete_warning');
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
    created() {
      this.searchformData.host_code = this.propHostCode;
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
        wafCacheRuleListApi( {
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
      handleAdd() {
        this.addFormVisible = true
        this.formData = { ...INITIAL_DATA };
        this.formData.host_code = this.propHostCode
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

          postdata['rule_type'] = Number(postdata['rule_type'])
          postdata['param_type'] = Number(postdata['param_type'])
          postdata['cache_time'] = Number(postdata['cache_time'])
          postdata['priority'] = Number(postdata['priority'])

          wafCacheRuleAddApi({...postdata})
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
          postdata['rule_type'] = Number(postdata['rule_type'])
          postdata['param_type'] = Number(postdata['param_type'])
          postdata['cache_time'] = Number(postdata['cache_time'])
          postdata['priority'] = Number(postdata['priority'])
          wafCacheRuleEditApi({...postdata})
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
        wafCacheRuleDelApi({
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
        wafCacheRuleDetailApi({
              id: id
          })
          .then((res) => {
            let resdata = res
            console.log(resdata)
            if (resdata.code === 0) {
              that.detail_data = resdata.data;
              that.detail_data.rule_type = that.detail_data.rule_type.toString()
              that.detail_data.param_type = that.detail_data.param_type.toString()
              that.detail_data.cache_time = that.detail_data.cache_time.toString()
              that.detail_data.priority = that.detail_data.priority.toString()
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
  .form-item-tips {
  color: var(--hw-muted);
  font-size: 13px;
  margin-top: 4px;
  line-height: 1.5;
}
</style>
