<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button @click="handleAdd">{{ $t('page.access.account.button_add') }}</hw-button>
        </div>
        <div class="right-operation-container">
          <hw-form ref="searchForm" :data="searchformData" :label-width="80" layout="inline" colon
            :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.access.account.label_name')" name="account_name">
              <hw-input v-model="searchformData.account_name" class="search-input" clearable></hw-input>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList()">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.access.account.alert_message')" doc="guide/AccessAccount" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :loading="dataLoading" @page-change="rehandlePageChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">
          <template #status="{ row }">
            <hw-tag v-if="row.status === 1" theme="success" variant="light">{{ $t('page.access.account.status_enable') }}</hw-tag>
            <hw-tag v-else theme="danger" variant="light">{{ $t('page.access.account.status_disable') }}</hw-tag>
          </template>
          <template #otp_bound="{ row }">
            <hw-tag v-if="row.otp_bound === 1" theme="primary" variant="light">{{ $t('page.access.account.otp_bound') }}</hw-tag>
            <span v-else>-</span>
          </template>
          <template #allow_host_codes="{ row }">
            <hw-tag v-if="!row.allow_host_codes" theme="primary" variant="light">{{ $t('page.access.account.all_hosts') }}</hw-tag>
            <hw-tooltip v-else :content="hostNames(row.allow_host_codes)">
              <hw-tag theme="warning" variant="light">{{ countLines(row.allow_host_codes) }}</hw-tag>
            </hw-tooltip>
          </template>
          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleEdit(slotProps.row)">{{ $t('common.edit') }}</button>
            <button type="button" class="t-button-link" @click="handleResetPwd(slotProps.row)">{{ $t('page.access.account.button_resetpwd') }}</button>
            <button type="button" class="t-button-link" @click="handleOtp(slotProps.row)">{{ $t('page.access.account.button_otp') }}</button>
            <button type="button" class="t-button-link" @click="handleKick(slotProps.row)">{{ $t('page.access.account.button_kick') }}</button>
            <button type="button" class="t-button-link" @click="handleDelete(slotProps.row)">{{ $t('common.delete') }}</button>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <!-- 新增 -->
    <hw-dialog :header="$t('common.new')" v-model:visible="addFormVisible" :width="700" :footer="false">
      <template #body><div >
        <hw-form :data="formData" ref="addForm" :rules="addRules" @submit="onSubmitAdd" :labelWidth="120"
          autocomplete="off">
          <!-- autocomplete 必须显式关掉。
               浏览器看到「文本框 + 紧跟着的密码框」就认定这是登录表单，会把保存的
               **管理端 admin 账号密码**填进来；用户不留神点了确定，就凭空多出一个
               与管理员同名同密码、而且是公网可登录的访客账号。
               Chrome 只认 new-password（off 对密码框无效），所以两个都得给。 -->
          <hw-form-item :label="$t('page.access.account.label_name')" name="account_name">
            <hw-input :style="{ width: 'min(100%, 460px)' }" v-model="formData.account_name"
              autocomplete="off"></hw-input>
            <div class="form-tips">{{ $t('page.access.account.name_immutable_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_password')" name="password">
            <hw-input type="password" :style="{ width: 'min(100%, 460px)' }" v-model="formData.password"
              autocomplete="new-password"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_nickname')" name="nick_name">
            <hw-input :style="{ width: 'min(100%, 460px)' }" v-model="formData.nick_name"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_status')" name="status">
            <hw-radio-group v-model="formData.status">
              <hw-radio :value="1">{{ $t('page.access.account.status_enable') }}</hw-radio>
              <hw-radio :value="0">{{ $t('page.access.account.status_disable') }}</hw-radio>
            </hw-radio-group>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_force_otp')" name="force_otp">
            <hw-radio-group v-model="formData.force_otp">
              <hw-radio :value="0">{{ $t('page.access.mode_inherit') }}</hw-radio>
              <hw-radio :value="1">{{ $t('page.access.account.otp_force') }}</hw-radio>
              <hw-radio :value="2">{{ $t('page.access.account.otp_exempt') }}</hw-radio>
            </hw-radio-group>
            <div class="form-tips">{{ $t('page.access.account.force_otp_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_allow_hosts')" name="allow_host_codes">
            <hw-select :style="{ width: 'min(100%, 460px)' }" v-model="addAllowHosts" multiple filterable clearable
              :placeholder="$t('page.access.account.allow_hosts_placeholder')">
              <hw-option v-for="h in hostOptions" :key="h.value" :value="h.value" :label="h.label"></hw-option>
            </hw-select>
            <div class="form-tips">{{ $t('page.access.account.allow_hosts_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_expire')" name="expire_time">
            <hw-date-picker :style="{ width: 'min(100%, 460px)' }" v-model="formData.expire_time" enable-time-picker clearable
              format="YYYY-MM-DD HH:mm:ss" value-type="YYYY-MM-DD HH:mm:ss"
              :placeholder="$t('page.access.account.expire_placeholder')"></hw-date-picker>
            <div class="form-tips">{{ $t('page.access.account.expire_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: 'min(100%, 460px)' }" v-model="formData.remarks"></hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="addFormVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- 编辑：登录名不可改 -->
    <hw-dialog :header="$t('common.edit')" v-model:visible="editFormVisible" :width="700" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="editForm" :rules="editRules" @submit="onSubmitEdit" :labelWidth="120">
          <hw-form-item :label="$t('page.access.account.label_name')">
            <hw-input :style="{ width: 'min(100%, 460px)' }" :value="formEditData.account_name" disabled></hw-input>
            <div class="form-tips">{{ $t('page.access.account.name_immutable_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_nickname')" name="nick_name">
            <hw-input :style="{ width: 'min(100%, 460px)' }" v-model="formEditData.nick_name"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_status')" name="status">
            <hw-radio-group v-model="formEditData.status">
              <hw-radio :value="1">{{ $t('page.access.account.status_enable') }}</hw-radio>
              <hw-radio :value="0">{{ $t('page.access.account.status_disable') }}</hw-radio>
            </hw-radio-group>
            <div class="form-tips">{{ $t('page.access.account.disable_kick_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_force_otp')" name="force_otp">
            <hw-radio-group v-model="formEditData.force_otp">
              <hw-radio :value="0">{{ $t('page.access.mode_inherit') }}</hw-radio>
              <hw-radio :value="1">{{ $t('page.access.account.otp_force') }}</hw-radio>
              <hw-radio :value="2">{{ $t('page.access.account.otp_exempt') }}</hw-radio>
            </hw-radio-group>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_allow_hosts')" name="allow_host_codes">
            <hw-select :style="{ width: 'min(100%, 460px)' }" v-model="editAllowHosts" multiple filterable clearable
              :placeholder="$t('page.access.account.allow_hosts_placeholder')">
              <hw-option v-for="h in hostOptions" :key="h.value" :value="h.value" :label="h.label"></hw-option>
            </hw-select>
            <div class="form-tips">{{ $t('page.access.account.allow_hosts_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('page.access.account.label_expire')" name="expire_time">
            <hw-date-picker :style="{ width: 'min(100%, 460px)' }" v-model="formEditData.expire_time" enable-time-picker clearable
              format="YYYY-MM-DD HH:mm:ss" value-type="YYYY-MM-DD HH:mm:ss"
              :placeholder="$t('page.access.account.expire_placeholder')"></hw-date-picker>
            <div class="form-tips">{{ $t('page.access.account.expire_tips') }}</div>
          </hw-form-item>
          <hw-form-item :label="$t('common.remarks')" name="remarks">
            <hw-textarea :style="{ width: 'min(100%, 460px)' }" v-model="formEditData.remarks"></hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="editFormVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- 重置密码 -->
    <hw-dialog :header="$t('page.access.account.button_resetpwd')" v-model:visible="resetPwdVisible" :width="600" :footer="false">
      <template #body><div >
        <hw-alert theme="warning" :message="$t('page.access.account.resetpwd_warning')"></hw-alert>
        <hw-form :data="resetPwdData" ref="resetForm" :rules="resetRules" @submit="onSubmitResetPwd" :labelWidth="120"
          autocomplete="off">
          <hw-form-item :label="$t('page.access.account.label_new_password')" name="password">
            <hw-input type="password" :style="{ width: 'min(100%, 380px)' }" v-model="resetPwdData.password"
              autocomplete="new-password"></hw-input>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="resetPwdVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- 二次验证绑定 -->
    <hw-dialog :header="$t('page.access.account.button_otp')" v-model:visible="otpVisible" :width="600" :footer="false">
      <template #body><div >
        <div v-if="currentAccount.otp_bound === 1">
          <hw-alert theme="success" :message="$t('page.access.account.otp_already_bound')"></hw-alert>
          <div style="text-align: right; margin-top: 16px">
            <hw-button variant="outline" @click="otpVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="danger" @click="onOtpUnbind">{{ $t('page.access.account.button_otp_unbind') }}</hw-button>
          </div>
        </div>
        <div v-else>
          <hw-alert theme="info" :message="$t('page.access.account.otp_bind_tips')"></hw-alert>
          <div style="text-align: center; margin: 20px 0">
            <qrcode-vue v-if="otpData.url" :value="otpData.url" :size="200" level="H" />
          </div>
          <hw-form :data="otpData" ref="otpForm" @submit="onOtpBind" :labelWidth="120">
            <hw-form-item :label="$t('page.access.account.label_otp_secret')">
              <hw-input :style="{ width: 'min(100%, 380px)' }" :value="otpData.secret" readonly></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.access.account.label_otp_code')" name="code">
              <hw-input :style="{ width: 'min(100%, 380px)' }" v-model="otpData.code" maxlength="6" placeholder="000000"></hw-input>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button variant="outline" @click="otpVisible = false">{{ $t('common.close') }}</hw-button>
              <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </div></template>
    </hw-dialog>

    <hw-dialog :header="$t('common.confirm_delete')" :body="$t('page.access.account.delete_warning')"
      v-model:visible="delConfirmVisible" @confirm="onConfirmDelete"></hw-dialog>

    <hw-dialog :header="$t('page.access.account.button_kick')" :body="$t('page.access.account.kick_confirm')"
      v-model:visible="kickConfirmVisible" @confirm="onConfirmKick"></hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { prefix } from '@/config/global';
import QrcodeVue from 'qrcode.vue';
import {
  wafAccessAccountListApi, wafAccessAccountAddApi, wafAccessAccountEditApi,
  wafAccessAccountDelApi, wafAccessAccountResetPwdApi,
  wafAccessAccountOtpInitApi, wafAccessAccountOtpBindApi, wafAccessAccountOtpUnbindApi,
  wafAccessSessionKickByAccountApi,
} from '@/apis/access';
import { allhost } from '@/apis/host';

// 授权站点在库里是「换行分隔的站点唯一码」，界面上必须是站点名的多选。
// 这两个函数是这层翻译的全部：拆行 → 数组，数组 → 拆行。
const splitHostCodes = (raw) => String(raw || '').split(/[\n,]/).map((x) => x.trim()).filter((x) => x !== '');
const joinHostCodes = (arr) => (arr || []).join('\n');

const INITIAL_ACCOUNT = {
  account_name: '',
  password: '',
  nick_name: '',
  status: 1,
  force_otp: 0,
  allow_host_codes: '',
  expire_time: '',
  remarks: '',
};

export default defineComponent({
  name: 'WafAccessAccount',
  components: { QrcodeVue },
  data() {
    return {
      prefix,
      dataLoading: false,
      data: [],
      addFormVisible: false,
      editFormVisible: false,
      resetPwdVisible: false,
      otpVisible: false,
      delConfirmVisible: false,
      kickConfirmVisible: false,
      formData: { ...INITIAL_ACCOUNT },
      formEditData: { id: '', ...INITIAL_ACCOUNT },
      // 多选框绑数组，提交前再拼回换行串
      addAllowHosts: [],
      editAllowHosts: [],
      hostOptions: [],
      resetPwdData: { id: '', password: '' },
      otpData: { id: '', secret: '', url: '', code: '' },
      currentAccount: { id: '', account_name: '', otp_bound: 0 },
      // 新增与编辑各用一份规则：两个弹窗共用同一份时会互相串扰
      addRules: {
        account_name: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.access.account.label_name'), type: 'error' }],
        password: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.access.account.label_password'), type: 'error' }],
      },
      editRules: {},
      resetRules: {
        password: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.access.account.label_new_password'), type: 'error' }],
      },
      columns: [
        { title: this.$t('page.access.account.label_name'), align: 'left', width: 160, ellipsis: true, colKey: 'account_name' },
        { title: this.$t('page.access.account.label_nickname'), width: 140, ellipsis: true, colKey: 'nick_name' },
        { title: this.$t('page.access.account.label_status'), width: 90, colKey: 'status' },
        { title: this.$t('page.access.account.col_otp'), width: 90, colKey: 'otp_bound' },
        { title: this.$t('page.access.account.label_allow_hosts'), width: 120, colKey: 'allow_host_codes' },
        { title: this.$t('page.access.account.col_last_login'), width: 170, ellipsis: true, colKey: 'last_login_time' },
        { title: this.$t('page.access.account.col_last_login_ip'), width: 140, ellipsis: true, colKey: 'last_login_ip' },
        { title: this.$t('common.remarks'), width: 160, ellipsis: true, colKey: 'remarks' },
        { align: 'left', width: 300, colKey: 'op', title: this.$t('common.op') },
      ],
      rowKey: 'id',
      verticalAlign: 'top',
      hover: true,
      pagination: { total: 0, current: 1, pageSize: 10 },
      searchformData: { account_name: '' },
      pendingId: '',
    };
  },
  computed: {
    offsetTop() {
      return this.$store.state.setting.isUseTabsRouter ? 48 : 0;
    },
  },
  mounted() {
    this.getHostOptions();
    this.getList();
  },
  methods: {
    getContainer() {
      return document.querySelector('.tdesign-starter-layout');
    },
    getHostOptions() {
      const that = this;
      allhost().then((res) => {
        if (res.code === 0) {
          that.hostOptions = res.data ?? [];
        }
      }).catch(() => { /* 取不到站点列表时多选框为空，不影响其它字段 */ });
    },
    countLines(raw) {
      if (!raw) return '';
      const n = splitHostCodes(raw).length;
      return this.$t('page.access.account.limited_hosts').replace('{n}', n);
    },
    // 把库里的短码翻译成站点名给人看。站点被删时后端会同步摘掉短码，
    // 这里的兜底只是为了老数据不至于显示成一片空白。
    hostNames(raw) {
      return splitHostCodes(raw).map((code) => {
        const hit = this.hostOptions.find((h) => h.value === code);
        return hit ? hit.label : this.$t('page.access.account.host_deleted') + '(' + code + ')';
      }).join('\n');
    },
    getList() {
      const that = this;
      this.dataLoading = true;
      wafAccessAccountListApi({
        pageSize: this.pagination.pageSize,
        pageIndex: this.pagination.current,
        account_name: this.searchformData.account_name,
      }).then((res) => {
        if (res.code === 0) {
          that.data = res.data.list ?? [];
          that.pagination = { ...that.pagination, total: res.data.total };
        } else {
          that.$message.warning(res.msg);
        }
      }).catch((e) => console.log(e)).finally(() => {
        that.dataLoading = false;
      });
    },
    rehandlePageChange(curr) {
      this.pagination.current = curr.current;
      this.pagination.pageSize = curr.pageSize;
      this.getList();
    },
    handleAdd() {
      this.formData = { ...INITIAL_ACCOUNT };
      this.addAllowHosts = [];
      this.addFormVisible = true;
    },
    onSubmitAdd({ firstError }) {
      if (firstError) {
        this.$message.warning(firstError);
        return;
      }
      const that = this;
      wafAccessAccountAddApi({ ...this.formData, allow_host_codes: joinHostCodes(this.addAllowHosts) }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.addFormVisible = false;
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    handleEdit(row) {
      this.formEditData = {
        id: row.id,
        account_name: row.account_name,
        password: '',
        nick_name: row.nick_name,
        status: row.status,
        force_otp: row.force_otp ?? 0,
        allow_host_codes: row.allow_host_codes ?? '',
        // 后端返回的零值时间会是 0001-01-01 之类，回填成空串表示"永不过期"
        expire_time: this.normalizeTime(row.expire_time),
        remarks: row.remarks,
      };
      this.editAllowHosts = splitHostCodes(row.allow_host_codes);
      this.editFormVisible = true;
    },
    normalizeTime(v) {
      if (!v) return '';
      const s = String(v);
      if (s.startsWith('0001-') || s.startsWith('0000-')) return '';
      return s;
    },
    onSubmitEdit({ firstError }) {
      if (firstError) {
        this.$message.warning(firstError);
        return;
      }
      const that = this;
      wafAccessAccountEditApi({ ...this.formEditData, allow_host_codes: joinHostCodes(this.editAllowHosts) }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.editFormVisible = false;
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    handleResetPwd(row) {
      this.resetPwdData = { id: row.id, password: '' };
      this.resetPwdVisible = true;
    },
    onSubmitResetPwd({ firstError }) {
      if (firstError) {
        this.$message.warning(firstError);
        return;
      }
      const that = this;
      wafAccessAccountResetPwdApi({ ...this.resetPwdData }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.resetPwdVisible = false;
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    handleOtp(row) {
      const that = this;
      this.currentAccount = { id: row.id, account_name: row.account_name, otp_bound: row.otp_bound };
      this.otpData = { id: row.id, secret: '', url: '', code: '' };
      if (row.otp_bound === 1) {
        this.otpVisible = true;
        return;
      }
      wafAccessAccountOtpInitApi({ id: row.id }).then((res) => {
        if (res.code === 0) {
          that.otpData = { id: row.id, secret: res.data.secret, url: res.data.url, code: '' };
          that.otpVisible = true;
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    onOtpBind() {
      const that = this;
      wafAccessAccountOtpBindApi({
        id: this.otpData.id, secret: this.otpData.secret, code: this.otpData.code,
      }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.otpVisible = false;
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    onOtpUnbind() {
      const that = this;
      wafAccessAccountOtpUnbindApi({ id: this.currentAccount.id }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.otpVisible = false;
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
      });
    },
    handleKick(row) {
      this.pendingId = row.id;
      this.kickConfirmVisible = true;
    },
    onConfirmKick() {
      const that = this;
      wafAccessSessionKickByAccountApi({ account_id: this.pendingId }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
        } else {
          that.$message.warning(res.msg);
        }
        that.kickConfirmVisible = false;
      });
    },
    handleDelete(row) {
      this.pendingId = row.id;
      this.delConfirmVisible = true;
    },
    onConfirmDelete() {
      const that = this;
      wafAccessAccountDelApi({ id: this.pendingId }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
        that.delConfirmVisible = false;
      });
    },
  },
});
</script>

<style lang="less" scoped>
@import '@/style/variables';

.list-card-container {
  padding: @spacer-2 @spacer-3;

  :deep(.t-card__body) {
    padding: 0;
  }
}

.left-operation-container {
  padding: 0 0 @spacer-2 0;
  margin-bottom: @spacer;

  .t-button + .t-button {
    margin-left: @spacer;
  }
}

.search-input {
  width: 200px;
}

.form-tips {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
  line-height: 20px;
  margin-top: 4px;
}
</style>
