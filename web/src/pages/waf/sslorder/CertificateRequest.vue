<!-- HomeWaf modification (2026-11-17): shared certificate application and credentials. -->
<template>
  <div class="certificate-page resource-panel ssl-order-page">
    <div v-if="mode === 'request'" class="certificate-toolbar">
      <div><RouterLink to="/waf-host/wafsslconfiglist">返回证书管理</RouterLink></div>
    </div>
    <section v-if="mode === 'settings'">
      <section class="certificate-section">
        <h3>DNS 验证凭据</h3>
        <p class="certificate-muted">选择服务商和分组后维护对应字段。凭据保存立即生效，取消申请不会撤销它。</p>
        <DnsProviderConfig
          v-model:provider="formData.apply_dns"
          v-model:group="formData.private_group_name"
          :revision="dnsRevision"
          @configure="handlePrivateInfo"
          @add-group="handleAddPrivateGroup"
        />
      </section>
      <section class="certificate-section">
        <h3>证书机构与账户</h3>
        <p class="certificate-muted">CA 地址与账户认证分别保存。ZeroSSL 当前为实例共用账户。</p>
        <div class="certificate-actions">
          <hw-button @click="handleCaServerMaintenance">维护 CA 服务器</hw-button
          ><hw-button variant="outline" @click="handleSslHttpCheck">HTTP 挑战异常告警</hw-button>
        </div>
        <ZeroSSLAccount :revision="zeroSSLRevision" @saved="zeroSSLRevision++" />
      </section>
    </section>
    <section v-if="mode === 'request'" class="certificate-request-page">
      <hw-form
        class="certificate-request-form"
        :disabled="addSubmitting || hostListLoading"
        :data="formData"
        ref="form"
        :rules="rules"
        @submit="onSubmit"
        :labelWidth="240"
        ><fieldset class="form-section">
          <legend>申请对象</legend>
          <hw-form-item label="关联服务" name="host_code">
            <hw-select
              v-model="formData.host_code"
              filterable
              clearable
              :loading="hostListLoading"
              :disabled="hostListLoading || !!hostListError || host_options.length === 0"
              aria-label="关联服务"
              placeholder="选择服务接入中的服务"
              :style="{ width: 'min(100%, 480px)' }"
              @change="changeHostCode"
            >
              <hw-option v-for="item in host_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
            <p v-if="hostListLoading" role="status" class="form-item-tips">正在加载服务列表…</p>
            <p v-else-if="hostListError" role="alert" class="form-item-tips">
              {{ hostListError }} <hw-button variant="text" @click="loadHostList">重试加载服务</hw-button>
            </p>
            <p v-else-if="host_options.length === 0" class="form-item-tips">
              暂无可关联服务。请先在新标签页添加服务，返回后刷新列表继续申请。<router-link
                :to="{ path: '/waf-pipeline/ports', query: { returnTo: $route.fullPath } }"
                target="_blank"
                rel="noopener noreferrer"
                >去添加服务</router-link
              >
              <hw-button variant="text" @click="loadHostList">刷新服务列表</hw-button>
            </p>
            <div v-else class="form-item-tips">
              <span>使用服务接入中的现有服务；DNS 验证不要求开放 80/443。</span>
              <hw-button variant="text" @click="loadHostList">刷新服务列表</hw-button>
            </div>
            <p
              v-if="
                selectedHostInfo(formData.host_code)?.source === 'pipeline' &&
                selectedHostInfo(formData.host_code)?.protocol === 'http'
              "
              class="form-item-tips"
            >
              签发后保存证书并关联此服务；当前 HTTP 服务不会自动切换为 HTTPS，可在服务设置中启用。
            </p>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_domain')" name="apply_domain">
            <hw-textarea
              v-model="formData.apply_domain"
              :style="{ width: 'min(100%, 480px)' }"
              rows="4"
              placeholder="example.com, *.example.com（多个域名用逗号或换行分隔）"
            ></hw-textarea>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_email')" name="apply_email">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.apply_email"></hw-input>
          </hw-form-item>
        </fieldset>
        <fieldset class="form-section">
          <legend>签发与域名验证</legend>
          <hw-form-item :label="$t('page.sslorder.label_apply_platform')" name="apply_platform">
            <div style="display: flex; align-items: center">
              <hw-select v-model="formData.apply_platform" clearable :style="{ width: 'min(100%, 250px)' }">
                <hw-option v-for="item in sslorder_platform_type" :value="item.value" :label="`${item.label}`">
                </hw-option>
              </hw-select>
              <hw-button theme="primary" variant="text" style="margin-left: 10px" @click="handleCaServerMaintenance">
                {{ $t('page.sslorder.maintenance_platform') }}
              </hw-button>
            </div>
          </hw-form-item>
          <ZeroSSLAccount
            v-if="addFormVisible && formData.apply_platform === 'zerossl'"
            :revision="zeroSSLRevision"
            @status="zeroSSLAddStatus = $event"
            @saved="zeroSSLRevision++"
          />
          <hw-form-item :label="$t('page.sslorder.label_apply_method')" name="apply_method">
            <hw-select
              v-model="formData.apply_method"
              clearable
              :style="{ width: 'min(100%, 480px)' }"
              @change="handleApplyMethodChange"
            >
              <hw-option v-for="item in sslorder_apply_method_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
            <div
              v-if="formData.apply_method === 'http01' && formData.host_code && !addHost80Ok"
              class="form-item-tips"
              style="color: #e34d59"
            >
              {{ $t('page.sslorder.http01_no_80_tips') }}
            </div>
          </hw-form-item>
          <DnsProviderConfig
            v-if="addFormVisible && formData.apply_method === 'dns01'"
            v-model:provider="formData.apply_dns"
            v-model:group="formData.private_group_name"
            :revision="dnsRevision"
            @configure="handlePrivateInfo"
            @add-group="handleAddPrivateGroup"
          />
          <hw-form-item v-if="formData.apply_method === 'dns01'" label="本地 DNS 传播检查" name="skip_dns_verify">
            <hw-radio-group v-model="formData.skip_dns_verify">
              <hw-radio :value="0">检查后验证（推荐）</hw-radio>
              <hw-radio :value="1">等待后直接验证</hw-radio>
            </hw-radio-group>
            <p class="form-item-tips">通常保留检查。跳过只影响本地预检查，证书机构仍会验证公网 TXT 记录。</p>
          </hw-form-item>
        </fieldset>
        <fieldset class="form-section">
          <legend>续期方式</legend>
          <hw-form-item label="自动续期" name="auto_renew">
            <hw-checkbox v-model="formData.auto_renew">启用自动续期</hw-checkbox>
            <p class="form-item-tips">
              签发成功后自动检查并在临近到期时续期，无需到过期检测页面另行配置。取消勾选后需手动续期。
            </p>
          </hw-form-item>
        </fieldset>
        <div class="form-actions">
          <hw-button variant="outline" @click="onClickCloseBtn">取消申请</hw-button>
          <hw-button
            theme="primary"
            type="submit"
            :loading="addSubmitting"
            :disabled="addSubmitting || (formData.apply_platform === 'zerossl' && zeroSSLAddStatus !== 'configured')"
            >提交申请</hw-button
          >
        </div></hw-form
      >
    </section>

    <hw-dialog
      :header="privateFormMode === 'add' ? $t('common.add') : $t('common.edit')"
      :visible="privateFormVisible"
      @update:visible="
        (value) => {
          if (!privateSubmitting) privateFormVisible = value;
        }
      "
      :width="680"
      :footer="false"
    >
      <template #body
        ><div>
          <p v-if="privateError" role="alert" class="certificate-error">{{ privateError }}</p>
          <hw-form
            :disabled="privateSubmitting"
            :data="privateFormData"
            ref="privateForm"
            :rules="privateRules"
            @submit="onPrivateSubmit"
            :labelWidth="200"
          >
            <hw-form-item :label="$t('page.private_info.private_group_name')" name="private_group_name">
              <hw-input
                v-model="privateFormData.private_group_name"
                :style="{ width: 'min(100%, 480px)' }"
                disabled
              ></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.private_info.private_group_belong_cloud')" name="private_group_belong_cloud">
              <hw-input
                v-model="privateFormData.private_group_belong_cloud"
                :style="{ width: 'min(100%, 480px)' }"
                disabled
              ></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.private_info.private_key')" name="private_key">
              <hw-input
                v-model="privateFormData.private_key"
                :style="{ width: 'min(100%, 480px)' }"
                disabled
              ></hw-input>
            </hw-form-item>
            <template v-if="privateFormData.private_key === 'HURRICANE_TOKENS'">
              <p>填写验证域名及对应 TXT 记录的 DDNS Key。根域名和泛域名只需一行。</p>
              <label v-if="privateFormMode === 'edit'" class="credential-replace"
                ><input v-model="hurricaneReplace" type="checkbox" /> 更新 DDNS 密钥（需重新填写本分组全部域名）</label
              >
              <div v-if="hurricaneReplace" class="hurricane-credentials">
                <div v-for="(row, index) in hurricaneRows" :key="index" class="hurricane-credential">
                  <hw-form-item :label="`验证域名 ${index + 1}`"
                    ><hw-input v-model="row.domain" :aria-label="`验证域名 ${index + 1}`" placeholder="example.com"
                  /></hw-form-item>
                  <hw-form-item :label="`DDNS Key ${index + 1}`"
                    ><hw-input
                      v-model="row.token"
                      :aria-label="`DDNS Key ${index + 1}`"
                      type="password"
                      autocomplete="new-password"
                      placeholder="从 dns.he.net 复制的 Token"
                  /></hw-form-item>
                  <hw-button v-if="hurricaneRows.length > 1" variant="text" @click="hurricaneRows.splice(index, 1)"
                    >移除此域名</hw-button
                  >
                </div>
                <hw-button variant="outline" @click="hurricaneRows.push({ domain: '', token: '' })"
                  >添加验证域名</hw-button
                >
              </div>
              <p v-else>密钥已保存；保持此选项关闭会保留原值。</p>
            </template>
            <hw-form-item v-else :label="$t('page.private_info.private_value')" name="private_value">
              <hw-textarea
                v-if="privateMultiline"
                v-model="privateFormData.private_value"
                :placeholder="privateFormMode === 'edit' ? '留空保留原值' : '粘贴完整 JSON、PEM 或多行配置'"
                rows="5"
              />
              <hw-input
                v-else
                v-model="privateFormData.private_value"
                type="password"
                autocomplete="new-password"
                :placeholder="privateFormMode === 'edit' ? '留空保留原值' : '填写配置值'"
              />
              <label class="credential-replace"
                ><input v-model="privateMultiline" type="checkbox" /> 多行输入（JSON / PEM，内容可见）</label
              >
            </hw-form-item>
            <hw-form-item :label="$t('page.private_info.remarks')" name="remarks">
              <hw-input v-model="privateFormData.remarks" :style="{ width: 'min(100%, 480px)' }"></hw-input>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button variant="outline" :disabled="privateSubmitting" @click="privateFormVisible = false">{{
                $t('common.close')
              }}</hw-button>
              <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div></template
      >
    </hw-dialog>

    <!-- 证书文件验证方式设置对话框 -->
    <hw-dialog header="HTTP 挑战异常告警" v-model:visible="sslHttpCheckDialogVisible" :width="680" :footer="false">
      <template #body
        ><div>
          <p>仅控制本地没有挑战文件、且上游返回异常响应时是否记录告警。不改变 CA 验证要求，也不改变证书签发能力。</p>
          <hw-form
            :data="sslHttpCheckFormData"
            ref="sslHttpCheckForm"
            :rules="sslHttpCheckRules"
            @submit="onSubmitSslHttpCheck"
            :labelWidth="150"
          >
            <hw-form-item :label="$t('page.systemconfig.label_configuration_item')" name="item">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="sslHttpCheckFormData.item" disabled></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.systemconfig.label_configuration_value')" name="value">
              <hw-select v-model="sslHttpCheckFormData.value" :style="{ width: 'min(100%, 480px)' }">
                <hw-option value="1" label="记录异常告警"></hw-option>
                <hw-option value="0" label="不记录此类告警"></hw-option>
              </hw-select>
              <div class="form-item-tips">{{ $t('page.sslorder.ssl_http_check_tips') }}</div>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button variant="outline" @click="sslHttpCheckDialogVisible = false">{{
                $t('common.close')
              }}</hw-button>
              <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div></template
      >
    </hw-dialog>

    <!-- 添加密钥分组对话框 -->
    <hw-dialog
      :header="$t('page.private_group.button_add_private_group')"
      :visible="privateGroupFormVisible"
      @update:visible="
        (value) => {
          if (!groupSubmitting) privateGroupFormVisible = value;
        }
      "
      :width="680"
      :footer="false"
    >
      <template #body
        ><div>
          <p v-if="groupError" role="alert" class="certificate-error">{{ groupError }}</p>
          <hw-form
            :disabled="groupSubmitting"
            :data="privateGroupFormData"
            ref="privateGroupForm"
            :rules="privateGroupRules"
            @submit="onPrivateGroupSubmit"
            :labelWidth="200"
          >
            <hw-form-item :label="$t('page.private_group.private_group_name')" name="private_group_name">
              <hw-input
                v-model="privateGroupFormData.private_group_name"
                :style="{ width: 'min(100%, 480px)' }"
              ></hw-input>
            </hw-form-item>
            <hw-form-item
              :label="$t('page.private_group.private_group_belong_cloud')"
              name="private_group_belong_cloud"
            >
              <hw-input
                v-model="privateGroupFormData.private_group_belong_cloud"
                :style="{ width: 'min(100%, 480px)' }"
                disabled
              ></hw-input>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button variant="outline" :disabled="groupSubmitting" @click="privateGroupFormVisible = false">{{
                $t('common.close')
              }}</hw-button>
              <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div></template
      >
    </hw-dialog>

    <!-- CA服务器维护对话框 -->
    <hw-dialog
      :header="$t('page.ca_server_info.title')"
      v-model:visible="caServerMaintenanceVisible"
      :width="900"
      :footer="false"
    >
      <template #body
        ><div>
          <div style="margin-bottom: 16px">
            <hw-button @click="handleAddCaServer" theme="success">{{ $t('common.new') }}</hw-button>
          </div>

          <p v-if="caError" role="alert" class="certificate-error">
            {{ caError }} <hw-button @click="loadCaServerListForMaintenance">重试</hw-button>
          </p>
          <!-- CA服务器列表表格 -->
          <hw-table :columns="caServerColumns" :data="caServerList" :rowKey="'id'" :loading="caServerLoading">
            <template #op="{ row }">
              <button type="button" class="t-button-link" @click="handleEditCaServer(row)">
                {{ $t('common.edit') }}
              </button>
              <button type="button" class="t-button-link" @click="handleDeleteCaServer(row)">
                {{ $t('common.delete') }}
              </button>
            </template>
          </hw-table>

          <div style="text-align: right; margin-top: 16px">
            <hw-button variant="outline" @click="caServerMaintenanceVisible = false">{{
              $t('common.close')
            }}</hw-button>
          </div>
        </div></template
      >
    </hw-dialog>
    <!-- 添加/编辑CA服务器对话框 -->
    <hw-dialog
      :header="caServerFormMode === 'add' ? $t('common.add') : $t('common.edit')"
      :visible="caServerFormVisible"
      @update:visible="
        (value) => {
          if (!caSubmitting) caServerFormVisible = value;
        }
      "
      :width="680"
      :footer="false"
    >
      <template #body
        ><div>
          <p v-if="caError" role="alert" class="certificate-error">{{ caError }}</p>
          <hw-form
            :disabled="caSubmitting"
            :data="caServerFormData"
            ref="caServerForm"
            :rules="caServerRules"
            @submit="onCaServerSubmit"
            :labelWidth="200"
          >
            <hw-form-item :label="$t('page.ca_server_info.ca_server_name')" name="ca_server_name">
              <hw-input
                v-model="caServerFormData.ca_server_name"
                :style="{ width: 'min(100%, 480px)' }"
                :disabled="caServerFormMode === 'edit'"
              ></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.ca_server_info.ca_server_address')" name="ca_server_address">
              <hw-input v-model="caServerFormData.ca_server_address" :style="{ width: 'min(100%, 480px)' }"></hw-input>
            </hw-form-item>
            <ZeroSSLAccount
              v-if="caServerFormVisible && caServerFormData.ca_server_name === 'zerossl'"
              :revision="zeroSSLRevision"
              @saved="zeroSSLRevision++"
            />
            <hw-form-item :label="$t('page.ca_server_info.ca_server_remarks')" name="remarks">
              <hw-input v-model="caServerFormData.remarks" :style="{ width: 'min(100%, 480px)' }"></hw-input>
            </hw-form-item>
            <hw-form-item style="float: right">
              <hw-button variant="outline" :disabled="caSubmitting" @click="caServerFormVisible = false">{{
                $t('common.close')
              }}</hw-button>
              <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div></template
      >
    </hw-dialog>
  </div>
</template>
<script lang="ts">
import { v4 as uuidv4 } from 'uuid';
import { defineComponent } from 'vue';
import { ElMessageBox } from 'element-plus';
import '../sslconfig/certificates.css';

import { SearchIcon } from '@/ui/icons';
import Trend from '@/components/trend/index.vue';

import DnsProviderConfig from './DnsProviderConfig.vue';
import ZeroSSLAccount from './ZeroSSLAccount.vue';
import OrderLogs from './OrderLogs.vue';
import OrderTable from './OrderTable.vue';
import { serializeHurricaneCredentials } from './hurricane';
import {
  sslOrderAddApi,
  sslOrderDelApi,
  sslOrderEditApi,
  sslOrderListApi,
  sslOrderRetryApi,
  sslCertificateTargetsApi,
  type SSLCertificateTarget,
} from '@/apis/sslorder';

import { wafPrivateInfoEditApi, wafPrivateInfoAddApi } from '@/apis/private_info.ts';
import { get_detail_by_item_api, edit_system_config_api } from '@/apis/systemconfig';
import { wafPrivateGroupAddApi } from '@/apis/private_group.ts';
import {
  wafCaServerInfoListApi,
  wafCaServerInfoDelApi,
  wafCaServerInfoEditApi,
  wafCaServerInfoAddApi,
} from '@/apis/ca_server_info.ts';

const INITIAL_DATA = {
  host_code: '',
  apply_platform: 'letsencrypt',
  apply_method: 'dns01',
  skip_dns_verify: 0,
  auto_renew: true,
  apply_dns: '',
  apply_email: '',
  apply_domain: '',
  apply_status: 'submitted',
  private_group_name: 'default',
  private_group_belong_cloud: '',
};

export default defineComponent({
  name: 'CertificateRequest',
  components: {
    SearchIcon,
    Trend,
    DnsProviderConfig,
    ZeroSSLAccount,
    OrderLogs,
    OrderTable,
  },
  props: {
    mode: { type: String, default: 'request' },
    //原主机码
    srcHostCode: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      draftBaseline: '',
      privateSubmitting: false,
      privateError: '',
      caSubmitting: false,
      caError: '',
      groupSubmitting: false,
      groupError: '',
      zeroSSLRevision: 0,
      zeroSSLAddStatus: 'checking',
      zeroSSLEditStatus: 'checking',
      addSubmitting: false,
      editSubmitting: false,
      addSubmission: { payload: '', id: '' },
      editSubmission: { payload: '', id: '' },
      refreshTimer: undefined as ReturnType<typeof setInterval> | undefined,
      listRequestSequence: 0,
      listRequestPending: false,
      listError: '',
      orderLogsVisible: false,
      orderLogsId: '',
      retryingOrderId: '',
      retryRequestIds: {} as Record<string, string>,
      addFormVisible: false,
      editFormVisible: false,
      confirmVisible: false,
      formData: {
        ...INITIAL_DATA,
      },
      formEditData: {
        ...INITIAL_DATA,
      },
      rules: {
        host_code: [
          {
            required: true,
            message: '请选择关联服务',
            type: 'error',
          },
        ],
        apply_email: [
          {
            required: true,
            message: this.$t('common.select_placeholder') + this.$t('page.sslorder.label_apply_email'),
            type: 'error',
          },
        ],
        apply_domain: [
          {
            required: true,
            message: this.$t('common.select_placeholder') + this.$t('page.sslorder.label_apply_domain'),
            type: 'error',
          },
        ],
      },
      dataLoading: false,
      data: [], // 列表数据信息
      dnsRevision: 0,
      hurricaneReplace: false,
      hurricaneRows: [{ domain: '', token: '' }],
      privateMultiline: false,
      // 私钥信息表单
      privateFormVisible: false,
      privateFormMode: 'add', // 'add' 或 'edit'
      privateFormData: {
        private_group_name: '',
        private_group_belong_cloud: '',
        private_key: '',
        private_value: '',
        remarks: '',
      },
      privateRules: {
        private_key: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.private_info.private_key'),
            type: 'error',
          },
        ],
      },
      //分组
      privateGroupFormVisible: false,
      privateGroupFormData: {
        private_group_name: '',
        private_group_belong_cloud: '',
      },
      privateGroupRules: {
        private_group_name: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.private_group.private_group_name'),
            type: 'error',
          },
        ],
        private_group_belong_cloud: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.private_group.private_group_belong_cloud'),
            type: 'error',
          },
        ],
      },
      selectedRowKeys: [],
      rowKey: 'id',
      tableLayout: 'auto',
      verticalAlign: 'top',
      hover: true,
      pagination: {
        total: 0,
        current: 1,
        pageSize: 10,
      },
      searchformData: {
        apply_status: '',
        apply_domain: '',
        host_code: '',
      },
      deleteIdx: -1,
      //主机字典
      host_dic: {},
      host_options: [] as SSLCertificateTarget[],
      hostListLoading: false,
      hostListError: '',
      hostListRequest: 0,
      //文件验证(http01)时，所选主机是否具备80端口（主端口或绑定更多端口）
      addHost80Ok: true,
      editHost80Ok: true,
      sslHttpCheckDialogVisible: false,
      sslHttpCheckFormData: {
        item: 'sslhttp_check',
        value: '1',
        remarks: '证书文件验证：本地挑战文件始终优先；本项仅控制本地无挑战文件且后端返回非404/301/302时是否写告警',
      },
      sslHttpCheckRules: {
        item: [{ required: true, message: this.$t('page.systemconfig.label_configuration_item'), type: 'error' }],
        value: [{ required: true, message: this.$t('page.systemconfig.label_configuration_value'), type: 'error' }],
      },
      //订单状态
      sslorder_status_type: [
        {
          label: this.$t('page.sslorder.sslorder_status_type.submitted'),
          value: 'submitted',
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.applying'),
          value: 'applying',
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.fail'),
          value: 'fail',
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.success'),
          value: 'success',
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.renewed'),
          value: 'renewed',
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.expired'),
          value: 'expired',
        },
      ],
      //平台类型
      sslorder_platform_type: [],
      //申请方式
      sslorder_apply_method_type: [
        {
          label: 'HTTP 文件验证（需要公网 80 端口）',
          value: 'http01',
        },
        {
          label: 'DNS 验证（家庭网络推荐，无需入站端口）',
          value: 'dns01',
        },
      ],
      // CA服务器维护相关
      caServerMaintenanceVisible: false,
      caServerFormVisible: false,
      caServerFormMode: 'add', // 'add' 或 'edit'
      caServerList: [],
      caServerLoading: false,
      caServerFormData: {
        ca_server_name: '',
        ca_server_address: '',
        remarks: '',
      },
      caServerRules: {
        ca_server_name: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_name'),
            type: 'error',
          },
        ],
        ca_server_address: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_address'),
            type: 'error',
          },
        ],
        remarks: [
          {
            required: true,
            message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_remarks'),
            type: 'error',
          },
        ],
      },
      caServerColumns: [
        {
          align: 'left',
          width: 120,
          colKey: 'op',
          title: this.$t('common.op'),
        },
        {
          title: this.$t('page.ca_server_info.ca_server_name'),
          align: 'left',
          width: 200,
          colKey: 'ca_server_name',
        },
        {
          title: this.$t('page.ca_server_info.ca_server_address'),
          align: 'left',
          width: 300,
          colKey: 'ca_server_address',
        },
        {
          title: this.$t('page.ca_server_info.ca_server_remarks'),
          align: 'left',
          width: 200,
          colKey: 'remarks',
        },
      ],
      //END Data
    };
  },
  computed: {
    confirmBody() {
      if (this.deleteIdx > -1) {
        return this.$t('common.confirm_delete');
      }
      return '';
    },
  },
  created() {},
  async mounted() {
    this.loadCaServerList();
    if (this.mode === 'request') {
      await this.handleAdd();
      const host = String(this.$route.query.host_code || this.srcHostCode || '');
      if (host) {
        this.formData.host_code = host;
        this.changeHostCode(host);
      }
      if (this.$route.query.domains) this.formData.apply_domain = String(this.$route.query.domains);
    } else {
      this.formData.apply_dns = String(this.$route.query.provider || '');
      this.formData.private_group_name = String(this.$route.query.group || 'default');
    }
    this.draftBaseline = JSON.stringify(this.formData);
    window.addEventListener('beforeunload', this.protectUnload);
  },
  beforeUnmount() {
    window.removeEventListener('beforeunload', this.protectUnload);
    this.privateFormData.private_value = '';
    this.formData = { ...INITIAL_DATA };
  },
  watch: {
    // 监听srcHostCode变化，重新加载数据
    srcHostCode: {
      handler(newVal, oldVal) {
        if (newVal !== oldVal) {
          console.log('srcHostCode changed:', oldVal, '->', newVal);
          // 重置分页到第一页
          this.pagination.current = 1;
          // 重新加载数据
          this.reloadInitData();
        }
      },
      immediate: false, // 不需要立即执行，因为mounted已经会执行一次
    },
  },
  beforeRouteLeave() {
    return this.confirmDiscard();
  },
  methods: {
    resourceDirty() {
      return (
        (this.privateFormVisible &&
          (!!this.privateFormData.private_value || this.hurricaneRows.some((row) => !!row.token))) ||
        (this.privateGroupFormVisible && !!this.privateGroupFormData.private_group_name) ||
        this.caServerFormVisible
      );
    },
    async confirmDiscard() {
      if (this.addSubmitting || this.privateSubmitting || this.groupSubmitting || this.caSubmitting) return false;
      if (!this.resourceDirty() && (this.mode !== 'request' || JSON.stringify(this.formData) === this.draftBaseline))
        return true;
      try {
        await ElMessageBox.confirm('离开会放弃未提交的申请。已经保存的共享凭据仍然保留。', '未保存的申请', {
          confirmButtonText: '放弃申请',
          cancelButtonText: '继续填写',
          closeOnHashChange: false,
        });
        return true;
      } catch {
        return false;
      }
    },
    protectUnload(event: BeforeUnloadEvent) {
      if (
        this.resourceDirty() ||
        (this.mode === 'request' && (this.addSubmitting || JSON.stringify(this.formData) !== this.draftBaseline))
      ) {
        event.preventDefault();
        event.returnValue = '';
      }
    },

    openOrderLogs() {},
    retryOrder() {},
    orderSubmissionPayload(form) {
      const {
        host_code,
        apply_platform,
        apply_method,
        apply_dns,
        apply_email,
        apply_domain,
        private_group_name,
        skip_dns_verify,
        auto_renew,
      } = form;
      return {
        host_code,
        apply_platform,
        apply_method,
        apply_dns,
        apply_email,
        apply_domain,
        private_group_name,
        skip_dns_verify,
        auto_renew: auto_renew !== false,
      };
    },
    //初始化数据
    reloadInitData() {
      void this.loadHostList();
      this.loadCaServerList();
    },
    // 处理CA服务器维护
    handleCaServerMaintenance() {
      this.loadCaServerListForMaintenance();
      this.caServerMaintenanceVisible = true;
    },

    // 加载CA服务器列表用于维护
    async loadCaServerListForMaintenance() {
      this.caServerLoading = true;
      this.caError = '';
      try {
        const res = await wafCaServerInfoListApi({ pageSize: 100, pageIndex: 1 });
        if (res.code !== 0) throw new Error(res.msg);
        this.caServerList = res.data.list || [];
      } catch (error) {
        this.caError = error.message || 'CA 列表读取失败，请重试';
      } finally {
        this.caServerLoading = false;
      }
    },
    // 处理添加CA服务器
    handleAddCaServer() {
      this.caServerFormMode = 'add';
      this.caServerFormData = {
        ca_server_name: '',
        ca_server_address: '',
        remarks: '',
      };
      this.caServerFormVisible = true;
    },

    // 处理编辑CA服务器
    handleEditCaServer(row) {
      this.caServerFormMode = 'edit';
      this.caServerFormData = { ...row };
      this.caServerFormVisible = true;
    },

    // 处理删除CA服务器
    handleDeleteCaServer(row) {
      // 检查是否为letsencrypt，不允许删除
      if (row.ca_server_name === 'letsencrypt') {
        this.$message.warning(this.$t('page.sslorder.ca_server_letsencrypt_cannot_op'));
        return;
      }

      const confirmDialog = this.$dialog.confirm({
        header: this.$t('common.confirm_delete'),
        body: this.$t('common.data_delete_warning'),
        confirmBtn: {
          theme: 'danger',
          content: this.$t('common.confirm'),
        },
        cancelBtn: {
          theme: 'default',
          content: this.$t('common.cancel'),
        },
        onConfirm: () => {
          this.deleteCaServer(row.id);
          confirmDialog.hide(); // 手动关闭弹窗
        },
      });
    },

    // 删除CA服务器
    deleteCaServer(id) {
      wafCaServerInfoDelApi({ id })
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg || this.$t('common.tips.delete_success'));
            this.loadCaServerListForMaintenance();
            this.loadCaServerList(); // 刷新下拉选项
          } else {
            this.$message.warning(res.msg || this.$t('common.tips.delete_failed'));
          }
        })
        .catch((e) => {
          console.error('删除CA服务器失败:', e);
          this.$message.error(this.$t('common.tips.delete_failed'));
        });
    },

    // 提交CA服务器表单
    async onCaServerSubmit({ firstError }) {
      if (this.caSubmitting || firstError) return;
      this.caSubmitting = true;
      this.caError = '';
      try {
        const res = await (this.caServerFormMode === 'add'
          ? wafCaServerInfoAddApi({ ...this.caServerFormData })
          : wafCaServerInfoEditApi({ ...this.caServerFormData }));
        if (res.code !== 0) throw new Error(res.msg);
        this.caServerFormVisible = false;
        this.loadCaServerListForMaintenance();
        this.loadCaServerList();
        this.$message.success('CA 配置已保存');
      } catch (error) {
        this.caError = error.message || '保存失败，输入已保留';
      } finally {
        this.caSubmitting = false;
      }
    },
    // 加载CA服务器列表
    loadCaServerList() {
      let that = this;
      wafCaServerInfoListApi({
        pageSize: 100,
        pageIndex: 1,
      })
        .then((res) => {
          if (res.code === 0 && res.data && res.data.list) {
            // 将CA服务器信息转换为下拉选项格式
            that.sslorder_platform_type = res.data.list.map((item) => ({
              value: item.ca_server_name, // 使用ca_server_name作为键值
              label: item.remarks || item.ca_server_name, // 使用remarks作为显示文本，如果remarks为空则使用ca_server_name
            }));

            // 如果没有CA服务器数据，保留默认的letsencrypt选项
            if (that.sslorder_platform_type.length === 0) {
              that.sslorder_platform_type = [
                {
                  label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
                  value: 'letsencrypt',
                },
              ];
            }
          } else {
            console.error('获取CA服务器列表失败:', res.msg);
            // 失败时使用默认选项
            that.sslorder_platform_type = [
              {
                label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
                value: 'letsencrypt',
              },
            ];
          }
        })
        .catch((e) => {
          console.error('获取CA服务器列表异常:', e);
          // 异常时使用默认选项
          that.sslorder_platform_type = [
            {
              label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
              value: 'letsencrypt',
            },
          ];
        });
    },
    // 处理添加密钥分组
    handleAddPrivateGroup(cloudType) {
      this.privateGroupFormData = {
        private_group_name: '',
        private_group_belong_cloud: cloudType,
      };
      this.privateGroupFormVisible = true;
    },

    // 提交密钥分组表单
    async onPrivateGroupSubmit({ firstError }) {
      if (this.groupSubmitting || firstError) return;
      this.groupSubmitting = true;
      this.groupError = '';
      try {
        const res = await wafPrivateGroupAddApi({ ...this.privateGroupFormData });
        if (res.code !== 0) throw new Error(res.msg);
        this.formData.private_group_name = this.privateGroupFormData.private_group_name;
        this.privateGroupFormVisible = false;
        this.dnsRevision++;
        this.$message.success('凭据分组已保存');
      } catch (error) {
        this.groupError = error.message || '分组保存失败';
      } finally {
        this.groupSubmitting = false;
      }
    },
    // 处理证书文件验证方式设置
    handleSslHttpCheck() {
      // 获取当前配置
      get_detail_by_item_api({ item: 'sslhttp_check' })
        .then((res) => {
          if (res.code === 0 && res.data) {
            this.sslHttpCheckFormData = res.data;
          } else {
            // 如果没有找到配置，使用默认值
            this.sslHttpCheckFormData = {
              item: 'sslhttp_check',
              value: '1',
              remarks:
                '证书文件验证：本地挑战文件始终优先；本项仅控制本地无挑战文件且后端返回非404/301/302时是否写告警',
            };
          }
          this.sslHttpCheckDialogVisible = true;
        })
        .catch((err) => {
          console.error('获取证书文件验证方式配置失败:', err);
          this.$message.error(this.$t('common.tips.api_error'));
        });
    },

    // 提交证书文件验证方式设置
    onSubmitSslHttpCheck({ validateResult }) {
      if (validateResult === true) {
        edit_system_config_api(this.sslHttpCheckFormData)
          .then((res) => {
            if (res.code === 0) {
              this.$message.success(this.$t('common.tips.save_success'));
              this.sslHttpCheckDialogVisible = false;
            } else {
              this.$message.error(res.msg || this.$t('common.tips.save_failed'));
            }
          })
          .catch((err) => {
            console.error('保存证书文件验证方式配置失败:', err);
            this.$message.error(this.$t('common.tips.save_failed'));
          });
      }
    },
    // The status endpoint intentionally never returns credential values.
    handlePrivateInfo(keyName, cloudType, groupName, action) {
      this.privateFormMode = action;
      this.privateFormData = {
        private_group_belong_cloud: cloudType,
        private_group_name: groupName,
        private_key: keyName,
        private_value: '',
        remarks: '',
      };
      this.hurricaneRows = [{ domain: '', token: '' }];
      this.hurricaneReplace = action === 'add';
      this.privateMultiline = false;
      this.privateFormVisible = true;
    },

    // 提交私钥信息表单
    async onPrivateSubmit({ firstError }) {
      if (this.privateSubmitting || firstError) return;
      const payload = { ...this.privateFormData };
      try {
        if (payload.private_key === 'HURRICANE_TOKENS')
          payload.private_value = this.hurricaneReplace ? serializeHurricaneCredentials(this.hurricaneRows) : '****';
        else if (!payload.private_value) {
          if (this.privateFormMode === 'edit') payload.private_value = '****';
          else throw new Error('请填写配置值');
        }
      } catch (error) {
        this.privateError = error.message;
        return;
      }
      this.privateSubmitting = true;
      this.privateError = '';
      try {
        const res = await (this.privateFormMode === 'add'
          ? wafPrivateInfoAddApi(payload)
          : wafPrivateInfoEditApi(payload));
        if (res.code !== 0) throw new Error(res.msg);
        this.privateFormVisible = false;
        this.privateFormData.private_value = '';
        this.hurricaneRows = [{ domain: '', token: '' }];
        this.dnsRevision++;
        this.$message.success('共享凭据已保存');
      } catch (error) {
        this.privateError = error.message || '保存失败，输入已保留';
      } finally {
        this.privateSubmitting = false;
      }
    },
    selectedHostInfo(code: string) {
      return this.host_options.find((item) => item.value === code);
    },
    evalHost80(hostCode: string) {
      return Promise.resolve(this.selectedHostInfo(hostCode)?.http01_ready ?? false);
    },
    canSubmitForHost(code: string) {
      if (this.hostListLoading || this.hostListError || !this.selectedHostInfo(code)) {
        this.$message.warning(this.hostListError || '请选择有效的关联服务；列表为空时请先添加服务。');
        return false;
      }
      return true;
    },
    // 新建：所选主机的80端口检测（仅驱动右侧静态提示，不弹窗）
    checkAddHost80() {
      this.evalHost80(this.formData.host_code).then((ok) => {
        this.addHost80Ok = ok;
      });
    },
    // 续期：所选主机的80端口检测（仅驱动右侧静态提示，不弹窗）
    checkEditHost80() {
      this.evalHost80(this.formEditData.host_code).then((ok) => {
        this.editHost80Ok = ok;
      });
    },
    // 新建：申请方式变更
    handleApplyMethodChange(value) {
      if (value === 'http01') {
        this.checkAddHost80();
      }
    },
    // 续期：申请方式变更
    handleApplyMethodChangeEdit(value) {
      if (value === 'http01') {
        this.checkEditHost80();
      }
    },
    // 续期：主机变更
    changeHostCodeEdit(hostCode) {
      if (hostCode) {
        this.checkEditHost80();
      } else {
        this.editHost80Ok = true;
      }
    },
    changeHostCode(hostCode) {
      const target = this.selectedHostInfo(hostCode);
      this.formData.apply_domain = target?.domains.join(',') || '';
      this.checkAddHost80();
    },
    async loadHostList() {
      const request = ++this.hostListRequest;
      this.hostListLoading = true;
      this.hostListError = '';
      try {
        const response = await sslCertificateTargetsApi();
        if (request !== this.hostListRequest) return;
        if (response.code !== 0 || !Array.isArray(response.data)) throw new Error('服务列表不可用');
        this.host_options = response.data;
        this.host_dic = Object.fromEntries(this.host_options.map((item) => [item.value, item.label]));
      } catch {
        if (request !== this.hostListRequest) return;
        this.host_options = [];
        this.hostListError = '服务列表加载失败，请重试。';
      } finally {
        if (request === this.hostListRequest) this.hostListLoading = false;
      }
    },
    rehandlePageChange(curr, pageInfo) {
      this.pagination.current = curr.current;
      if (this.pagination.pageSize != curr.pageSize) {
        this.pagination.current = 1;
        this.pagination.pageSize = curr.pageSize;
      }
      this.getList('');
    },
    resetFilters() {
      this.searchformData.apply_domain = '';
      this.searchformData.apply_status = '';
      this.getList('all');
    },
    getList() {},
    async handleAdd() {
      if (this.addSubmitting) return;
      this.addSubmission = { payload: '', id: '' };
      this.addFormVisible = true;
      this.formData = { ...INITIAL_DATA };
      await this.loadHostList();
      console.log('mounted sslorder,srchostCode', this.srcHostCode);
      if (this.srcHostCode != '') {
        this.formData.host_code = this.srcHostCode;
        this.changeHostCode(this.srcHostCode);
      }
    },
    onSubmit({ result, firstError }): void {
      if (this.addSubmitting || !this.addFormVisible) return;
      let that = this;
      if (!firstError) {
        if (this.formData.apply_platform === 'zerossl' && this.zeroSSLAddStatus !== 'configured') {
          this.$message.warning('请先完成 ZeroSSL 账户认证配置和状态检查');
          return;
        }
        if (!this.canSubmitForHost(this.formData.host_code)) return;

        // 如果是http01方式,域名不能包含通配符
        if (this.formData.apply_method == 'http01') {
          if (this.formData.apply_domain.indexOf('*') != -1) {
            that.$message.warning(that.$t('page.sslorder.error_domain_not_match_method'));
            return;
          }
        }
        // 如果是dns方式,apply_dns不能为空
        if (this.formData.apply_method == 'dns01') {
          if (!this.formData.apply_dns) {
            that.$message.warning(that.$t('common.select_placeholder') + that.$t('page.sslorder.label_apply_dns'));
            return;
          }
        }
        const payload = this.orderSubmissionPayload(this.formData);
        const encoded = JSON.stringify(payload);
        if (this.addSubmission.payload !== encoded) this.addSubmission = { payload: encoded, id: uuidv4() };
        this.addSubmitting = true;
        sslOrderAddApi({ ...payload, request_id: this.addSubmission.id })
          .then((res) => {
            if (res.code === 0) {
              that.pagination.current = 1;
              that.searchformData.apply_domain = '';
              that.searchformData.apply_status = '';
              that.getList('');
              that.$message.success(res.msg || '申请已受理');
              that.addSubmission = { payload: '', id: '' };
              that.draftBaseline = JSON.stringify(that.formData);
              that.addSubmitting = false;
              that.$router.push({
                path: '/waf-host/wafsslOrderlist',
                query: { order: res.data.id, returnTo: '/waf-host/wafsslconfiglist' },
              });
            } else {
              that.$message.warning(res.msg);
            }
          })
          .catch(() => {
            this.$message.error('提交结果未确认，可重试；相同申请不会重复创建');
          })
          .finally(() => {
            this.addSubmitting = false;
          });
      }
    },
    handleClickEdit() {},
    onSubmitEdit() {},
    handleClickDelete() {},
    onConfirmDelete() {},
    onCancel() {},
    async onClickCloseBtn() {
      if (await this.confirmDiscard()) {
        this.draftBaseline = JSON.stringify(this.formData);
        this.$router.push('/waf-host/wafsslconfiglist');
      }
    },
    onClickCloseEditBtn() {
      this.editFormVisible = false;
    },
  },
});
</script>
<style scoped>
.certificate-request-page :deep(.certificate-request-form) {
  max-width: 880px;
}
.certificate-request-page :deep(.form-section) {
  border: 0;
  padding: 20px 0;
  margin: 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.certificate-request-page :deep(legend) {
  font-size: 18px;
  font-weight: 600;
  padding-top: 16px;
}
.certificate-request-page :deep(.form-actions) {
  position: static;
  display: flex;
  gap: 12px;
  padding: 16px 0;
  background: var(--el-bg-color);
}

.ssl-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 20px;
}
.ssl-guide {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.ssl-guide summary {
  cursor: pointer;
  padding-block: 6px;
}
.ssl-guide[open] summary {
  margin-bottom: 12px;
}
.ssl-guide summary:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.ssl-toolbar-actions,
.ssl-filter-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.ssl-toolbar-actions :deep(.el-button),
.ssl-filter-actions :deep(.el-button) {
  margin-left: 0;
}
.ssl-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 16px;
  margin-block: 24px 20px;
}
.ssl-filters label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.ssl-domain-filter {
  flex: 1 1 240px;
  max-width: 360px;
}
.ssl-status-filter {
  flex: 0 1 180px;
  min-width: 150px;
}
.search-input {
  width: 100%;
}
.ssl-list-caption {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.ssl-list-error {
  color: var(--el-color-danger);
}
.ssl-scope {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
@media (max-width: 600px) {
  .ssl-domain-filter {
    max-width: none;
    flex-basis: 100%;
  }
  .ssl-status-filter {
    flex: 1;
  }
  .ssl-toolbar-actions {
    width: 100%;
  }
}

.credential-replace {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-block: 12px;
  line-height: 1.5;
}
.hurricane-credential {
  padding-block: 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  margin-bottom: 12px;
}

@media (max-width: 600px) {
  .ssl-order-page :deep(.hb-row) {
    flex-wrap: wrap;
    align-items: flex-start;
  }
  .ssl-order-page :deep(.hb-txt) {
    flex-basis: calc(100% - 30px);
    min-width: 0;
  }
  .ssl-order-page :deep(.hb-actions) {
    margin-left: 24px;
  }
}
</style>
