<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">

        </div>
        <div class="right-operation-container">

        </div>
      </hw-row>
      <help-block :summary="$t('page.otp.alert_message')" doc="guide/Otp" />
      <div class="table-container">
        <hw-form v-if="!isBind" :data="formData" ref="form" :rules="rules" @submit="onBindSubmit" :labelWidth="200">
          <hw-form-item style="text-align: center">
            <qrcode-vue :value="formData.url" :size="qrSize" level="H" />
            <hw-popup :content="formData.secret">
              <hw-button variant="outline">{{ $t('page.otp.cannot_scan') }}</hw-button>
            </hw-popup>
          </hw-form-item>
          <hw-form-item :label="$t('page.otp.issuer')" name="issuer">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.issuer" :placeholder="$t('page.otp.issuer_placeholder')"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.otp.secret_code')" name="secret_code">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.secret_code" ></hw-input>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button theme="primary" type="submit">{{ $t('page.otp.bind') }}</hw-button>
          </hw-form-item>
        </hw-form>

        <hw-alert  v-if="isBind"  theme="success" close :max-line="1">
          <span>{{ $t('page.otp.bind_success_tip') }}</span>
          <hw-form :data="formBindData" ref="unBindForm" :rules="rules" @submit="onUnBindSubmit" :labelWidth="200">
            <hw-form-item :label="$t('page.otp.issuer')" name="issuer">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formBindData.issuer" :readonly="true" :placeholder="$t('page.otp.current_issuer')"></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.otp.secret_code')" name="secret_code">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formBindData.secret_code" ></hw-input>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button theme="primary" type="submit">{{ $t('page.otp.unbind') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </hw-alert>


      </div>
      <div>
      <router-view></router-view>
      </div>
    </hw-card>
  </div>
</template>
<script lang="ts">
import {defineComponent} from 'vue';


  import QrcodeVue from 'qrcode.vue'

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
    wafOtpInitApi,wafOtpBindApi,wafOtpUnBindApi
  } from '@/apis/otp.ts';

  const INITIAL_DATA = {
    user_name:'',
    is_enable_otp:'',
    url:'',
    secret:'',
    remarks:'',

  };
  export default defineComponent({
    name: 'OtpBase',
    components: {
      SearchIcon,
      Trend,
      QrcodeVue
    },
    data() {
      return {
        addFormVisible: false,
        editFormVisible: false,
        guardVisible: false,
        confirmVisible: false,
        formData: {
          user_name: "",
          url: "",
          secret: "",
          issuer: "",
          remarks: "",
          secret_code: ""
        },
        formBindData: {
          id: "",
          issuer: "",
          secret_code: ""
        },
        rules: {
          secret_code: [{
              required: true,
              message: this.$t('common.placeholder')+this.$t('page.otp.secret_code'),
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

        },
        //索引区域
        deleteIdx: -1,
        guardStatusIdx :-1,
        //主机字典
        host_dic:{},

        //是否是绑定
        isBind:false,
        qrSize: 300,
      };
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
      this.loadInitData().then(() => {

      });
    },

    watch: {
      'formData.issuer'(newIssuer) {
        // 当用户修改 Issuer 时，重新生成 OTP URL
        if (newIssuer && this.formData.secret && this.formData.user_name) {
          const encodedIssuer = encodeURIComponent(newIssuer);
          const encodedAccount = encodeURIComponent(this.formData.user_name);
          const encodedSecret = encodeURIComponent(this.formData.secret);
          this.formData.url = `otpauth://totp/${encodedIssuer}:${encodedAccount}?secret=${encodedSecret}&issuer=${encodedIssuer}`;
        }
      }
    },
    methods: {
      loadInitData() {
        return new Promise((resolve, reject) => {
          wafOtpInitApi()
            .then((res) => {
              let resdata = res;
              console.log('loadInitData',resdata);
              if (resdata.code === 0) {
                if(resdata.data.id===undefined){
                  this.isBind = false
                  this.formData.secret_code = ""
                  this.formData = resdata.data;
                }else{
                  this.isBind = true
                  this.formBindData.secret_code = ""
                  this.formBindData.id= resdata.data.id
                  this.formBindData.issuer= resdata.data.issuer || ""
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
      onBindSubmit({
        result,
        firstError
      }): void {
        let that = this
        if (!firstError) {

          let postdata = {
            ...that.formData
          }
          wafOtpBindApi({...postdata})
            .then((res) => {
              let resdata = res
              console.log(resdata)
              if (resdata.code === 0) {
                that.$message.success(resdata.msg);
                that.isBind = true;
                that.loadInitData().then(() => {

                });
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
      onUnBindSubmit({
        result,
        firstError
      }): void {
        let that = this
        if (!firstError) {

          let postdata = {
            ...that.formBindData
          }
          console.log('unbind',postdata)
          wafOtpUnBindApi({...postdata})
            .then((res) => {
              let resdata = res
              console.log(resdata)
              if (resdata.code === 0) {
                that.$message.success(resdata.msg);
                that.isBind = false;
                that.loadInitData().then(() => {

                });
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
      onCancel() {
        this.resetIdx();
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
