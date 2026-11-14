<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="ssl-order-page">
    <hw-card class="list-card-container">
      <div class="ssl-toolbar">
        <div class="ssl-toolbar-actions">
          <hw-button @click="handleAdd" theme="primary">申请证书</hw-button>
          <hw-button variant="outline" @click="handleCaServerMaintenance">CA 服务器</hw-button>
        </div>
      </div>
      <details class="ssl-guide">
        <summary>自动续期默认开启 · DNS 验证无需开放 80/443</summary>
      <help-block summary="签发成功后按计划检查续期，无需添加过期检测记录。请保留 DNS 凭据和验证记录；家庭网络推荐 DNS 验证。" doc="guide/SSL">
        <template #actions>
          <hw-button variant="text" @click="handleSslHttpCheck">HTTP 验证设置</hw-button>
        </template>
      </help-block>
      </details>
      <form class="ssl-filters" @submit.prevent="getList('all')">
        <label class="ssl-domain-filter"><span>申请域名</span><hw-input v-model="searchformData.apply_domain" class="search-input" aria-label="筛选申请域名" placeholder="输入域名搜索" clearable /></label>
        <label class="ssl-status-filter"><span>申请状态</span><hw-select v-model="searchformData.apply_status" placeholder="全部状态" aria-label="筛选申请状态" @change="getList('all')"><hw-option value="" label="全部状态" /><hw-option v-for="item in sslorder_status_type" :key="item.value" :value="item.value" :label="item.label" /></hw-select></label>
        <div class="ssl-filter-actions"><hw-button type="submit" theme="primary" variant="outline">查询</hw-button><hw-button variant="text" @click="resetFilters">重置</hw-button><hw-button variant="text" :loading="dataLoading" @click="getList('')">刷新</hw-button></div>
      </form>
      <p v-if="srcHostCode" class="ssl-scope">当前服务：{{ host_dic[srcHostCode] || srcHostCode }}</p>
      <p v-if="listError" role="alert" class="ssl-list-error">{{ listError }} <hw-button variant="text" @click="getList('')">刷新状态</hw-button></p>
      <div class="ssl-list-caption"><span>共 {{ pagination.total }} 条申请</span><span>每 3 秒更新申请状态</span></div>
      <OrderTable :rows="data" :host-names="host_dic" :loading="dataLoading" :filtered="!!(searchformData.apply_domain || searchformData.apply_status)" :retrying-order-id="retryingOrderId" :pagination="pagination"
        @logs="openOrderLogs" @retry="retryOrder" @renew="handleClickEdit({ row: $event })" @delete="handleClickDelete({ row: $event })" @page="rehandlePageChange" />
      <div>
        <router-view></router-view>
      </div>
    </hw-card>

    <OrderLogs v-model:visible="orderLogsVisible" :order-id="orderLogsId" :retrying="!!retryingOrderId" @retry="retryOrder({ id: $event, apply_status: 'fail' })" />

    <hw-dialog header="申请证书" v-model:visible="addFormVisible" :width="770" :footer="false">
      <template #body><div >
        <hw-form :data="formData" ref="form" :rules="rules" @submit="onSubmit" :labelWidth="240">
          <hw-form-item label="关联服务" name="host_code">
            <hw-select v-model="formData.host_code" filterable clearable :loading="hostListLoading"
              :disabled="hostListLoading || !!hostListError || host_options.length === 0" aria-label="关联服务"
              placeholder="选择服务接入中的服务" :style="{ width: 'min(100%, 480px)' }" @change="changeHostCode">
              <hw-option v-for="item in host_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
            <p v-if="hostListLoading" role="status" class="form-item-tips">正在加载服务列表…</p>
            <p v-else-if="hostListError" role="alert" class="form-item-tips">{{ hostListError }} <hw-button variant="text" @click="loadHostList">重试加载服务</hw-button></p>
            <p v-else-if="host_options.length === 0" class="form-item-tips">暂无可关联服务。请先在“服务接入”添加服务，再返回申请证书。<router-link to="/waf-pipeline/ports">去添加服务</router-link></p>
            <div v-else class="form-item-tips">
              <span>使用服务接入中的现有服务；DNS 验证不要求开放 80/443。</span>
              <hw-button variant="text" @click="loadHostList">刷新服务列表</hw-button>
            </div>
            <p v-if="selectedHostInfo(formData.host_code)?.source === 'pipeline' && selectedHostInfo(formData.host_code)?.protocol === 'http'" class="form-item-tips">签发后保存证书并关联此服务；当前 HTTP 服务不会自动切换为 HTTPS，可在服务设置中启用。</p>
          </hw-form-item>
          <hw-form-item label="自动续期" name="auto_renew">
            <hw-checkbox v-model="formData.auto_renew">启用自动续期</hw-checkbox>
            <p class="form-item-tips">签发成功后自动检查并在临近到期时续期，无需到过期检测页面另行配置。取消勾选后需手动续期。</p>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_platform')" name="apply_platform">
            <div style="display: flex; align-items: center;">
              <hw-select v-model="formData.apply_platform" clearable :style="{ width: 'min(100%, 250px)' }">
                <hw-option v-for="item in sslorder_platform_type" :value="item.value" :label="`${item.label}`">
                </hw-option>
              </hw-select>
              <hw-button theme="primary" variant="text" style="margin-left: 10px;"
                @click="handleCaServerMaintenance">
                {{ $t('page.sslorder.maintenance_platform') }}
              </hw-button>
            </div>
          </hw-form-item>
          <ZeroSSLAccount v-if="addFormVisible && formData.apply_platform === 'zerossl'" :revision="zeroSSLRevision"
            @status="zeroSSLAddStatus = $event" @saved="zeroSSLRevision++" />
          <hw-form-item :label="$t('page.sslorder.label_apply_method')" name="apply_method">
            <hw-select v-model="formData.apply_method" clearable :style="{ width: 'min(100%, 480px)' }" @change="handleApplyMethodChange">
              <hw-option v-for="item in sslorder_apply_method_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
            <div v-if="formData.apply_method === 'http01' && formData.host_code && !addHost80Ok"
              class="form-item-tips" style="color: #e34d59;">
              {{ $t('page.sslorder.http01_no_80_tips') }}
            </div>
          </hw-form-item>
          <DnsProviderConfig v-if="addFormVisible && formData.apply_method === 'dns01'"
            v-model:provider="formData.apply_dns" v-model:group="formData.private_group_name"
            :revision="dnsRevision" @configure="handlePrivateInfo" @add-group="handleAddPrivateGroup" />
          <hw-form-item v-if="formData.apply_method === 'dns01'" label="本地 DNS 传播检查" name="skip_dns_verify">
            <hw-radio-group v-model="formData.skip_dns_verify">
              <hw-radio :value="0">检查后验证（推荐）</hw-radio>
              <hw-radio :value="1">等待后直接验证</hw-radio>
            </hw-radio-group>
            <p class="form-item-tips">通常保留检查。跳过只影响本地预检查，证书机构仍会验证公网 TXT 记录。</p>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_email')" name="apply_email">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.apply_email"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_domain')" name="apply_domain">
            <hw-textarea v-model="formData.apply_domain" :style="{ width: 'min(100%, 480px)' }" rows="4" placeholder="example.com, *.example.com（多个域名用逗号或换行分隔）"></hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit" :loading="addSubmitting" :disabled="addSubmitting || (formData.apply_platform === 'zerossl' && zeroSSLAddStatus !== 'configured')">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>
    <hw-dialog header="手动续期" v-model:visible="editFormVisible" :width="770" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="form" :rules="rules" @submit="onSubmitEdit" :labelWidth="240">
          <hw-form-item label="关联服务" name="host_code">
            <hw-select v-model="formEditData.host_code" filterable clearable :loading="hostListLoading"
              :disabled="hostListLoading || !!hostListError || host_options.length === 0" aria-label="关联服务"
              placeholder="选择服务接入中的服务" :style="{ width: 'min(100%, 480px)' }" @change="changeHostCodeEdit">
              <hw-option v-for="item in host_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
            <p v-if="hostListLoading" role="status" class="form-item-tips">正在加载服务列表…</p>
            <p v-else-if="hostListError" role="alert" class="form-item-tips">{{ hostListError }} <hw-button variant="text" @click="loadHostList">重试加载服务</hw-button></p>
            <p v-else-if="host_options.length === 0" class="form-item-tips">暂无可关联服务。请先在“服务接入”添加服务，再返回申请证书。<router-link to="/waf-pipeline/ports">去添加服务</router-link></p>
            <div v-else class="form-item-tips">
              <span>使用服务接入中的现有服务；DNS 验证不要求开放 80/443。</span>
              <hw-button variant="text" @click="loadHostList">刷新服务列表</hw-button>
            </div>
            <p v-if="selectedHostInfo(formEditData.host_code)?.source === 'pipeline' && selectedHostInfo(formEditData.host_code)?.protocol === 'http'" class="form-item-tips">签发后保存证书并关联此服务；当前 HTTP 服务不会自动切换为 HTTPS，可在服务设置中启用。</p>
          </hw-form-item>
          <hw-form-item label="自动续期" name="auto_renew">
            <hw-checkbox v-model="formEditData.auto_renew">启用自动续期</hw-checkbox>
            <p class="form-item-tips">签发成功后自动检查并在临近到期时续期，无需到过期检测页面另行配置。取消勾选后需手动续期。</p>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_platform')" name="apply_platform">
            <div style="display: flex; align-items: center;">
              <hw-select v-model="formEditData.apply_platform" clearable :style="{ width: 'min(100%, 250px)' }">
                <hw-option v-for="item in sslorder_platform_type" :value="item.value" :label="`${item.label}`">
                </hw-option>
              </hw-select>
              <hw-button theme="primary" variant="text" style="margin-left: 10px;"
                @click="handleCaServerMaintenance">
                {{ $t('page.sslorder.maintenance_platform') }}
              </hw-button>
            </div>
          </hw-form-item>
          <ZeroSSLAccount v-if="editFormVisible && formEditData.apply_platform === 'zerossl'" :revision="zeroSSLRevision"
            @status="zeroSSLEditStatus = $event" @saved="zeroSSLRevision++" />
          <hw-form-item :label="$t('page.sslorder.label_apply_method')" name="apply_method">
            <hw-select v-model="formEditData.apply_method" clearable :style="{ width: 'min(100%, 480px)' }" @change="handleApplyMethodChangeEdit">
              <hw-option v-for="item in sslorder_apply_method_type" :value="item.value" :label="`${item.label}`">
              </hw-option>
            </hw-select>
            <div v-if="formEditData.apply_method === 'http01' && formEditData.host_code && !editHost80Ok"
              class="form-item-tips" style="color: #e34d59;">
              {{ $t('page.sslorder.http01_no_80_tips') }}
            </div>
          </hw-form-item>
          <DnsProviderConfig v-if="editFormVisible && formEditData.apply_method === 'dns01'"
            v-model:provider="formEditData.apply_dns" v-model:group="formEditData.private_group_name"
            :revision="dnsRevision" @configure="handlePrivateInfo" @add-group="handleAddPrivateGroup" />
          <hw-form-item v-if="formEditData.apply_method === 'dns01'" label="本地 DNS 传播检查" name="skip_dns_verify">
            <hw-radio-group v-model="formEditData.skip_dns_verify">
              <hw-radio :value="0">检查后验证（推荐）</hw-radio>
              <hw-radio :value="1">等待后直接验证</hw-radio>
            </hw-radio-group>
            <p class="form-item-tips">通常保留检查。跳过只影响本地预检查，证书机构仍会验证公网 TXT 记录。</p>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_email')" name="apply_email">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.apply_email"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.sslorder.label_apply_domain')" name="apply_domain">
            <hw-textarea v-model="formEditData.apply_domain" :style="{ width: 'min(100%, 480px)' }" rows="4" placeholder="example.com, *.example.com（多个域名用逗号或换行分隔）"></hw-textarea>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseEditBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit" :loading="editSubmitting" :disabled="editSubmitting || (formEditData.apply_platform === 'zerossl' && zeroSSLEditStatus !== 'configured')">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>


    <hw-dialog :header="$t('common.confirm_delete')" :body="confirmBody" v-model:visible="confirmVisible"
      @confirm="onConfirmDelete" :onCancel="onCancel">
    </hw-dialog>

    <!-- 添加/编辑私钥信息对话框 -->
    <hw-dialog :header="privateFormMode === 'add' ? $t('common.add') : $t('common.edit')"
      v-model:visible="privateFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="privateFormData" ref="privateForm" :rules="privateRules" @submit="onPrivateSubmit"
          :labelWidth="200">
          <hw-form-item :label="$t('page.private_info.private_group_name')" name="private_group_name">
            <hw-input v-model="privateFormData.private_group_name" :style="{ width: 'min(100%, 480px)' }"
              disabled></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.private_info.private_group_belong_cloud')" name="private_group_belong_cloud">
            <hw-input v-model="privateFormData.private_group_belong_cloud" :style="{ width: 'min(100%, 480px)' }"
              disabled></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.private_info.private_key')" name="private_key">
            <hw-input v-model="privateFormData.private_key" :style="{ width: 'min(100%, 480px)' }"
              disabled></hw-input>
          </hw-form-item>
          <template v-if="privateFormData.private_key === 'HURRICANE_TOKENS'">
            <p>填写验证域名及对应 TXT 记录的 DDNS Key。根域名和泛域名只需一行。</p>
            <label v-if="privateFormMode === 'edit'" class="credential-replace"><input v-model="hurricaneReplace" type="checkbox" /> 更新 DDNS 密钥（需重新填写本分组全部域名）</label>
            <div v-if="hurricaneReplace" class="hurricane-credentials">
              <div v-for="(row, index) in hurricaneRows" :key="index" class="hurricane-credential">
                <hw-form-item :label="`验证域名 ${index + 1}`"><hw-input v-model="row.domain" :aria-label="`验证域名 ${index + 1}`" placeholder="example.com" /></hw-form-item>
                <hw-form-item :label="`DDNS Key ${index + 1}`"><hw-input v-model="row.token" :aria-label="`DDNS Key ${index + 1}`" type="password" autocomplete="new-password" placeholder="从 dns.he.net 复制的 Token" /></hw-form-item>
                <hw-button v-if="hurricaneRows.length > 1" variant="text" @click="hurricaneRows.splice(index, 1)">移除此域名</hw-button>
              </div>
              <hw-button variant="outline" @click="hurricaneRows.push({ domain: '', token: '' })">添加验证域名</hw-button>
            </div>
            <p v-else>密钥已保存；保持此选项关闭会保留原值。</p>
          </template>
          <hw-form-item v-else :label="$t('page.private_info.private_value')" name="private_value">
            <hw-textarea v-if="privateMultiline" v-model="privateFormData.private_value" :placeholder="privateFormMode === 'edit' ? '留空保留原值' : '粘贴完整 JSON、PEM 或多行配置'" rows="5" />
            <hw-input v-else v-model="privateFormData.private_value" type="password" autocomplete="new-password" :placeholder="privateFormMode === 'edit' ? '留空保留原值' : '填写配置值'" />
            <label class="credential-replace"><input v-model="privateMultiline" type="checkbox" /> 多行输入（JSON / PEM，内容可见）</label>
          </hw-form-item>
          <hw-form-item :label="$t('page.private_info.remarks')" name="remarks">
            <hw-input v-model="privateFormData.remarks" :style="{ width: 'min(100%, 480px)' }"></hw-input>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="privateFormVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- 证书文件验证方式设置对话框 -->
    <hw-dialog :header="$t('page.sslorder.ssl_http_check_setting')" v-model:visible="sslHttpCheckDialogVisible"
      :width="680" :footer="false">
      <template #body><div >
        <p>{{ $t('page.sslorder.ssl_http_check_desc') }}</p>
        <hw-form :data="sslHttpCheckFormData" ref="sslHttpCheckForm" :rules="sslHttpCheckRules"
          @submit="onSubmitSslHttpCheck" :labelWidth="150">
          <hw-form-item :label="$t('page.systemconfig.label_configuration_item')" name="item">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="sslHttpCheckFormData.item" disabled></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.systemconfig.label_configuration_value')" name="value">
            <hw-select v-model="sslHttpCheckFormData.value" :style="{ width: 'min(100%, 480px)' }">
              <hw-option value="1" :label="$t('page.sslorder.ssl_http_check_strict')"></hw-option>
              <hw-option value="0" :label="$t('page.sslorder.ssl_http_check_loose')"></hw-option>
            </hw-select>
            <div class="form-item-tips">{{ $t('page.sslorder.ssl_http_check_tips') }}</div>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="sslHttpCheckDialogVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>


    <!-- 添加密钥分组对话框 -->
    <hw-dialog :header="$t('page.private_group.button_add_private_group')" v-model:visible="privateGroupFormVisible"
      :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="privateGroupFormData" ref="privateGroupForm" :rules="privateGroupRules"
          @submit="onPrivateGroupSubmit" :labelWidth="200">
          <hw-form-item :label="$t('page.private_group.private_group_name')" name="private_group_name">
            <hw-input v-model="privateGroupFormData.private_group_name" :style="{ width: 'min(100%, 480px)' }"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.private_group.private_group_belong_cloud')" name="private_group_belong_cloud">
            <hw-input v-model="privateGroupFormData.private_group_belong_cloud" :style="{ width: 'min(100%, 480px)' }"
              disabled></hw-input>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="privateGroupFormVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- CA服务器维护对话框 -->
    <hw-dialog :header="$t('page.ca_server_info.title')" v-model:visible="caServerMaintenanceVisible"
      :width="900" :footer="false">
      <template #body><div >
        <div style="margin-bottom: 16px;">
          <hw-button @click="handleAddCaServer" theme="success">{{ $t('common.new') }}</hw-button>
        </div>

        <!-- CA服务器列表表格 -->
        <hw-table :columns="caServerColumns" :data="caServerList" :rowKey="'id'"
          :loading="caServerLoading">
          <template #op="{ row }">
            <button type="button" class="t-button-link" @click="handleEditCaServer(row)">{{ $t('common.edit') }}</button>
            <button type="button" class="t-button-link" @click="handleDeleteCaServer(row)">{{ $t('common.delete') }}</button>
          </template>
        </hw-table>

        <div style="text-align: right; margin-top: 16px;">
          <hw-button variant="outline" @click="caServerMaintenanceVisible = false">{{ $t('common.close') }}</hw-button>
        </div>
      </div></template>
    </hw-dialog>
    <!-- 添加/编辑CA服务器对话框 -->
    <hw-dialog :header="caServerFormMode === 'add' ? $t('common.add') : $t('common.edit')"
      v-model:visible="caServerFormVisible" :width="680" :footer="false">
      <template #body><div >
        <hw-form :data="caServerFormData" ref="caServerForm" :rules="caServerRules"
          @submit="onCaServerSubmit" :labelWidth="200">
          <hw-form-item :label="$t('page.ca_server_info.ca_server_name')" name="ca_server_name">
            <hw-input v-model="caServerFormData.ca_server_name" :style="{ width: 'min(100%, 480px)' }"
              :disabled="caServerFormMode === 'edit'"></hw-input>
          </hw-form-item>
          <hw-form-item :label="$t('page.ca_server_info.ca_server_address')" name="ca_server_address">
            <hw-input v-model="caServerFormData.ca_server_address" :style="{ width: 'min(100%, 480px)' }"></hw-input>
          </hw-form-item>
          <ZeroSSLAccount v-if="caServerFormVisible && caServerFormData.ca_server_name === 'zerossl'" :revision="zeroSSLRevision" @saved="zeroSSLRevision++" />
          <hw-form-item :label="$t('page.ca_server_info.ca_server_remarks')" name="remarks">
            <hw-input v-model="caServerFormData.remarks" :style="{ width: 'min(100%, 480px)' }"></hw-input>
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="caServerFormVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import { v4 as uuidv4 } from 'uuid';
import {defineComponent} from 'vue';


import {
  SearchIcon
} from '@/ui/icons';
import Trend from '@/components/trend/index.vue';
import {
  prefix
} from '@/config/global';
import DnsProviderConfig from './DnsProviderConfig.vue';
import ZeroSSLAccount from './ZeroSSLAccount.vue';
import OrderLogs from './OrderLogs.vue';
import OrderTable from './OrderTable.vue';
import { serializeHurricaneCredentials } from './hurricane';
import {
  sslOrderDetailApi, sslOrderAddApi, sslOrderDelApi, sslOrderEditApi, sslOrderListApi, sslOrderRetryApi, sslCertificateTargetsApi, type SSLCertificateTarget
} from '@/apis/sslorder';

import {
  wafPrivateInfoEditApi, wafPrivateInfoAddApi
} from '@/apis/private_info.ts';
import { get_detail_by_item_api, edit_system_config_api } from '@/apis/systemconfig';
import {
  wafPrivateGroupAddApi
} from '@/apis/private_group.ts';
import {
   wafCaServerInfoListApi, wafCaServerInfoDelApi, wafCaServerInfoEditApi, wafCaServerInfoAddApi, wafCaServerInfoDetailApi
} from '@/apis/ca_server_info.ts';

const INITIAL_DATA = {
  host_code: "",
  apply_platform: "letsencrypt",
  apply_method: "dns01",
  skip_dns_verify: 0,
  auto_renew: true,
  apply_dns: "",
  apply_email: "",
  apply_domain: "",
  apply_status: "submitted",
  private_group_name: "default",
  private_group_belong_cloud: "",
};

export default defineComponent({
  name: 'SslOrderList',
  components: {
    SearchIcon,
    Trend,
    DnsProviderConfig,
    ZeroSSLAccount,
    OrderLogs,
    OrderTable,
  },
  props: {
    //原主机码
    srcHostCode: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
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
        ...INITIAL_DATA
      },
      formEditData: {
        ...INITIAL_DATA
      },
      rules: {
        host_code: [{
          required: true,
          message: '请选择关联服务',
          type: 'error'
        }],
        apply_email: [
          {
            required: true,
            message: this.$t('common.select_placeholder') + this.$t('page.sslorder.label_apply_email'),
            type: 'error'
          }
        ],
        apply_domain: [
          {
            required: true,
            message: this.$t('common.select_placeholder') + this.$t('page.sslorder.label_apply_domain'),
            type: 'error'
          }
        ]
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
        private_group_belong_cloud: "",
        private_key: '',
        private_value: '',
        remarks: ''
      },
      privateRules: {
        private_key: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.private_info.private_key'),
          type: 'error'
        }],

      },
      //分组
      privateGroupFormVisible: false,
      privateGroupFormData: {
        private_group_name: '',
        private_group_belong_cloud: ''
      },
      privateGroupRules: {
        private_group_name: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.private_group.private_group_name'),
          type: 'error'
        }],
        private_group_belong_cloud: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.private_group.private_group_belong_cloud'),
          type: 'error'
        }]
      },
      selectedRowKeys: [],
      rowKey: 'id',
      tableLayout: 'auto',
      verticalAlign: 'top',
      hover: true,
      pagination: {
        total: 0,
        current: 1,
        pageSize: 10
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
        remarks: '证书文件验证：本地挑战文件始终优先；本项仅控制本地无挑战文件且后端返回非404/301/302时是否写告警'
      },
      sslHttpCheckRules: {
        item: [{ required: true, message: this.$t('page.systemconfig.label_configuration_item'), type: 'error' }],
        value: [{ required: true, message: this.$t('page.systemconfig.label_configuration_value'), type: 'error' }]
      },
      //订单状态
      sslorder_status_type: [
        {
          label: this.$t('page.sslorder.sslorder_status_type.submitted'),
          value: 'submitted'
        }, {
          label: this.$t('page.sslorder.sslorder_status_type.applying'),
          value: 'applying'
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.fail'),
          value: 'fail'
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.success'),
          value: 'success'
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.renewed'),
          value: 'renewed'
        },
        {
          label: this.$t('page.sslorder.sslorder_status_type.expired'),
          value: 'expired'
        },
      ],
      //平台类型
      sslorder_platform_type: [ ],
      //申请方式
      sslorder_apply_method_type: [
        {
          label: 'HTTP 文件验证（需要公网 80 端口）',
          value: 'http01'
        },
        {
          label: 'DNS 验证（家庭网络推荐，无需入站端口）',
          value: 'dns01'
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
        remarks: ''
      },
      caServerRules: {
        ca_server_name: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_name'),
          type: 'error'
        }],
        ca_server_address: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_address'),
          type: 'error'
        }],
        remarks: [{
          required: true,
          message: this.$t('common.placeholder') + this.$t('page.ca_server_info.ca_server_remarks'),
          type: 'error'
        }]
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
        }
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
  created() {
  },
  mounted() {
    this.reloadInitData();
    this.refreshTimer = setInterval(() => {
      if (this.data.some((row) => ['submitted', 'applying'].includes(row.apply_status))) this.getList('', true);
    }, 3000);
  },
  beforeUnmount() {
    clearInterval(this.refreshTimer);
    this.listRequestSequence++;
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
      immediate: false // 不需要立即执行，因为mounted已经会执行一次
    }
  },
  methods: {
    openOrderLogs(id: string) {
      this.orderLogsId = id;
      this.orderLogsVisible = true;
    },
    async retryOrder(row) {
      if (this.retryingOrderId || row.apply_status !== 'fail') return;
      this.retryingOrderId = row.id;
      this.retryRequestIds[row.id] ||= uuidv4();
      try {
        const result = await sslOrderRetryApi({ id: row.id, request_id: this.retryRequestIds[row.id] });
        if (result.code !== 0) { this.$message.warning(result.msg || '重试未受理，请查看日志'); return; }
        delete this.retryRequestIds[row.id];
        this.pagination.current = 1;
        this.searchformData.apply_domain = '';
          this.searchformData.apply_status = '';
        this.getList('');
        this.$message.success(result.msg || '重试申请已受理');
        this.openOrderLogs(result.data.id);
      } catch {
        this.$message.error('重试结果未确认，请再次点击重试；相同提交不会重复创建订单');
      } finally {
        this.retryingOrderId = '';
      }
    },
    orderSubmissionPayload(form) {
      const { host_code, apply_platform, apply_method, apply_dns, apply_email, apply_domain, private_group_name, skip_dns_verify, auto_renew } = form;
      return { host_code, apply_platform, apply_method, apply_dns, apply_email, apply_domain, private_group_name, skip_dns_verify, auto_renew: auto_renew !== false };
    },
    //初始化数据
    reloadInitData() {
      void this.loadHostList();
      this.getList('');
      this.loadCaServerList();
    },
    // 处理CA服务器维护
    handleCaServerMaintenance() {
      this.loadCaServerListForMaintenance();
      this.caServerMaintenanceVisible = true;
    },

    // 加载CA服务器列表用于维护
    loadCaServerListForMaintenance() {
      this.caServerLoading = true;
      wafCaServerInfoListApi({
        pageSize: 100,
        pageIndex: 1
      })
      .then((res) => {
        if (res.code === 0 && res.data && res.data.list) {
          this.caServerList = res.data.list;
        } else {
          this.caServerList = [];
          this.$message.warning(res.msg || this.$t('common.tips.get_failed'));
        }
      })
      .catch((e) => {
        console.error('获取CA服务器列表失败:', e);
        this.caServerList = [];
        this.$message.error(this.$t('common.tips.api_error'));
      })
      .finally(() => {
        this.caServerLoading = false;
      });
    },
    // 处理添加CA服务器
    handleAddCaServer() {
      this.caServerFormMode = 'add';
      this.caServerFormData = {
        ca_server_name: '',
        ca_server_address: '',
        remarks: ''
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
          content: this.$t('common.confirm')
        },
        cancelBtn: {
          theme: 'default',
          content: this.$t('common.cancel')
        },
        onConfirm: () => {
          this.deleteCaServer(row.id);
          confirmDialog.hide(); // 手动关闭弹窗
        }
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
    onCaServerSubmit({ result, firstError }) {
      if (!firstError) {
        if (this.caServerFormMode === 'add') {
          // 添加新CA服务器
          wafCaServerInfoAddApi({ ...this.caServerFormData })
            .then((res) => {
              if (res.code === 0) {
                this.$message.success(res.msg || this.$t('common.tips.save_success'));
                this.caServerFormVisible = false;
                this.loadCaServerListForMaintenance();
                this.loadCaServerList(); // 刷新下拉选项
              } else {
                this.$message.warning(res.msg || this.$t('common.tips.save_failed'));
              }
            })
            .catch((e) => {
              console.error('添加CA服务器失败:', e);
              this.$message.error(this.$t('common.tips.save_failed'));
            });
        } else {
          // 编辑CA服务器
          wafCaServerInfoEditApi({ ...this.caServerFormData })
            .then((res) => {
              if (res.code === 0) {
                this.$message.success(res.msg || this.$t('common.tips.save_success'));
                this.caServerFormVisible = false;
                this.loadCaServerListForMaintenance();
                this.loadCaServerList(); // 刷新下拉选项
              } else {
                this.$message.warning(res.msg || this.$t('common.tips.save_failed'));
              }
            })
            .catch((e) => {
              console.error('编辑CA服务器失败:', e);
              this.$message.error(this.$t('common.tips.save_failed'));
            });
        }
      } else {
        console.log('Errors: ', result);
        this.$message.warning(firstError);
      }
    },
    // 加载CA服务器列表
    loadCaServerList() {
      let that = this;
      wafCaServerInfoListApi({
        pageSize: 100,
        pageIndex: 1
      })
      .then((res) => {
        if (res.code === 0 && res.data && res.data.list) {
          // 将CA服务器信息转换为下拉选项格式
          that.sslorder_platform_type = res.data.list.map(item => ({
            value: item.ca_server_name, // 使用ca_server_name作为键值
            label: item.remarks || item.ca_server_name // 使用remarks作为显示文本，如果remarks为空则使用ca_server_name
          }));

          // 如果没有CA服务器数据，保留默认的letsencrypt选项
          if (that.sslorder_platform_type.length === 0) {
            that.sslorder_platform_type = [
              {
                label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
                value: 'letsencrypt'
              }
            ];
          }
        } else {
          console.error('获取CA服务器列表失败:', res.msg);
          // 失败时使用默认选项
          that.sslorder_platform_type = [
            {
              label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
              value: 'letsencrypt'
            }
          ];
        }
      })
      .catch((e) => {
        console.error('获取CA服务器列表异常:', e);
        // 异常时使用默认选项
        that.sslorder_platform_type = [
          {
            label: that.$t('page.sslorder.sslorder_platform_type.letsencrypt'),
            value: 'letsencrypt'
          }
        ];
      });
    },
    // 处理添加密钥分组
    handleAddPrivateGroup(cloudType) {
      this.privateGroupFormData = {
        private_group_name: '',
        private_group_belong_cloud: cloudType
      };
      this.privateGroupFormVisible = true;
    },

    // 提交密钥分组表单
    onPrivateGroupSubmit({ result, firstError }) {
      let that = this;
      if (!firstError) {
        wafPrivateGroupAddApi({ ...this.privateGroupFormData })
          .then((res) => {
            if (res.code === 0) {
              that.$message.success(res.msg || that.$t('common.tips.save_success'));
              that.privateGroupFormVisible = false;
              // 刷新分组列表
              const active = that.editFormVisible ? that.formEditData : that.formData;
              active.private_group_name = that.privateGroupFormData.private_group_name;
              that.dnsRevision++;
            } else {
              that.$message.warning(res.msg || that.$t('common.tips.save_failed'));
            }
          })
          .catch((e) => {
            console.error('添加密钥分组失败:', e);
            that.$message.error(that.$t('common.tips.save_failed'));
          });
      } else {
        console.log('Errors: ', result);
        that.$message.warning(firstError);
      }
    },
    // 处理证书文件验证方式设置
    handleSslHttpCheck() {
      // 获取当前配置
      get_detail_by_item_api({ item: 'sslhttp_check' }).then(res => {
        if (res.code === 0 && res.data) {
          this.sslHttpCheckFormData = res.data;
        } else {
          // 如果没有找到配置，使用默认值
          this.sslHttpCheckFormData = {
            item: 'sslhttp_check',
            value: '1',
            remarks: '证书文件验证：本地挑战文件始终优先；本项仅控制本地无挑战文件且后端返回非404/301/302时是否写告警'
          };
        }
        this.sslHttpCheckDialogVisible = true;
      }).catch(err => {
        console.error('获取证书文件验证方式配置失败:', err);
        this.$message.error(this.$t('common.tips.api_error'));
      });
    },

    // 提交证书文件验证方式设置
    onSubmitSslHttpCheck({ validateResult }) {
      if (validateResult === true) {
        edit_system_config_api(this.sslHttpCheckFormData).then(res => {
          if (res.code === 0) {
            this.$message.success(this.$t('common.tips.save_success'));
            this.sslHttpCheckDialogVisible = false;
          } else {
            this.$message.error(res.msg || this.$t('common.tips.save_failed'));
          }
        }).catch(err => {
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
    onPrivateSubmit({ result, firstError }) {
      let that = this;
      if (!firstError) {
        const payload = { ...this.privateFormData };
        if (payload.private_key === 'HURRICANE_TOKENS') {
          try {
            payload.private_value = this.hurricaneReplace
              ? serializeHurricaneCredentials(this.hurricaneRows)
              : '****';
          } catch (error) {
            this.$message.warning(error.message);
            return;
          }
        } else if (!payload.private_value) {
          if (this.privateFormMode === 'edit') payload.private_value = '****';
          else { this.$message.warning('请填写配置值'); return; }
        }

        if (this.privateFormMode === 'add') {
          // 添加新私钥
          wafPrivateInfoAddApi(payload)
            .then((res) => {
              if (res.code === 0) {
                that.$message.success(res.msg);
                that.privateFormVisible = false;
                // 刷新私钥列表
                that.dnsRevision++;
              } else {
                  that.$message.warning(res.msg);
              }
            })
            .catch((e) => {
              console.log(e);
            });
        } else {
          // 编辑私钥
          wafPrivateInfoEditApi(payload)
            .then((res) => {
              if (res.code === 0) {
                that.$message.success(res.msg);
                that.privateFormVisible = false;
                // 刷新私钥列表
                that.dnsRevision++;
              } else {
                that.$message.warning(res.msg);
              }
            })
            .catch((e) => {
              console.log(e);
            });
        }
      } else {
        console.log('Errors: ', result);
        that.$message.warning(firstError);
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
      this.pagination.current = curr.current
      if (this.pagination.pageSize != curr.pageSize) {
        this.pagination.current = 1
        this.pagination.pageSize = curr.pageSize
      }
      this.getList("")
    },
    resetFilters() {
      this.searchformData.apply_domain = '';
      this.searchformData.apply_status = '';
      this.getList('all');
    },
    getList(keyword, quiet = false) {
      if (keyword === 'all') this.pagination.current = 1;
      if (quiet && this.listRequestPending) return;
      this.listRequestPending = true;
      const sequence = ++this.listRequestSequence;
      let that = this;
      that.searchformData.host_code = this.srcHostCode
      sslOrderListApi({
        pageSize: that.pagination.pageSize,
        pageIndex: that.pagination.current,
        ...that.searchformData
      })
        .then((res) => {
          if (sequence !== this.listRequestSequence) return;
          let resdata = res;
          if (resdata.code === 0) {
            this.listError = '';
            this.data = resdata.data.list ?? [];
            this.pagination = {
              ...this.pagination,
              total: resdata.data.total,
            };
          } else {
            this.listError = resdata.msg || '订单状态刷新失败，请重试';
          }
        })
        .catch((e: Error) => {
          if (sequence === this.listRequestSequence) this.listError = '订单状态刷新失败，当前显示的是上次结果，请检查网络后重试';
        })
        .finally(() => {
          if (sequence === this.listRequestSequence) { this.dataLoading = false; this.listRequestPending = false; }
        });
      if (!quiet) this.dataLoading = true;
    },
    async handleAdd() {
      if (this.addSubmitting) return;
      this.addSubmission = { payload: '', id: '' };
      this.addFormVisible = true;
      this.formData = { ...INITIAL_DATA };
      await this.loadHostList();
      console.log("mounted sslorder,srchostCode", this.srcHostCode)
      if (this.srcHostCode != "") {
        this.formData.host_code = this.srcHostCode;
        this.changeHostCode(this.srcHostCode)
      }
    },
    onSubmit({
      result,
      firstError
    }): void {
      if (this.addSubmitting || !this.addFormVisible) return;
      let that = this;
      if (!firstError) {
        if (this.formData.apply_platform === 'zerossl' && this.zeroSSLAddStatus !== 'configured') {
          this.$message.warning('请先完成 ZeroSSL 账户认证配置和状态检查');
          return;
        }
        if (!this.canSubmitForHost(this.formData.host_code)) return;

        // 如果是http01方式,域名不能包含通配符
        if (this.formData.apply_method == "http01") {
          if (this.formData.apply_domain.indexOf("*") != -1) {
            that.$message.warning(that.$t('page.sslorder.error_domain_not_match_method'));
            return
          }
        }
        // 如果是dns方式,apply_dns不能为空
        if (this.formData.apply_method == "dns01") {
          if (!this.formData.apply_dns) {
            that.$message.warning(that.$t('common.select_placeholder') + that.$t('page.sslorder.label_apply_dns'));
            return
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
              that.addFormVisible = false;
            } else {
              that.$message.warning(res.msg);
            }
          })
          .catch(() => { this.$message.error('提交结果未确认，可重试；相同申请不会重复创建'); })
          .finally(() => { this.addSubmitting = false; });
      }
    },
    handleClickEdit(slotProps) {
      if (this.editSubmitting) return;
      this.editSubmission = { payload: '', id: '' };
      void this.loadHostList();
      const {
        row
      } = slotProps;
      this.formEditData = {
        ...row,
        auto_renew: row.auto_renew !== false,
        skip_dns_verify: row.skip_dns_verify ?? 0
      };
      // 文件验证方式：检测该主机是否具备80端口
      this.editHost80Ok = true;
      if (row.apply_method === 'http01' && row.host_code) {
        this.checkEditHost80();
      }
      this.editFormVisible = true;
    },
    onSubmitEdit({
      result,
      firstError
    }): void {
      if (this.editSubmitting || !this.editFormVisible) return;
      let that = this;
      if (!firstError) {
        if (this.formEditData.apply_platform === 'zerossl' && this.zeroSSLEditStatus !== 'configured') {
          this.$message.warning('请先完成 ZeroSSL 账户认证配置和状态检查');
          return;
        }
        if (!this.canSubmitForHost(this.formEditData.host_code)) return;
        const payload = { ...this.orderSubmissionPayload(this.formEditData), id: this.formEditData.id };
        const encoded = JSON.stringify(payload);
        if (this.editSubmission.payload !== encoded) this.editSubmission = { payload: encoded, id: uuidv4() };
        this.editSubmitting = true;
        sslOrderEditApi({ ...payload, request_id: this.editSubmission.id })
          .then((res) => {
            if (res.code === 0) {
              that.pagination.current = 1;
              that.searchformData.apply_domain = '';
              that.searchformData.apply_status = '';
              that.getList('');
              that.$message.success(res.msg || '续期申请已受理');
              that.editSubmission = { payload: '', id: '' };
              that.editFormVisible = false;
            } else {
              that.$message.warning(res.msg);
            }
          })
          .catch(() => { this.$message.error('续期结果未确认，可重试；相同申请不会重复创建'); })
          .finally(() => { this.editSubmitting = false; });
      }
    },
    handleClickDelete(slotProps) {
      const {
        row
      } = slotProps;
      this.deleteIdx = row.id;
      this.confirmVisible = true;
    },
    onConfirmDelete() {
      let that = this;
      sslOrderDelApi({
        id: this.deleteIdx
      })
        .then((res) => {
          if (res.code === 0) {
            that.getList('');
            that.$message.success('删除成功');
            that.confirmVisible = false;
            that.deleteIdx = -1;
          }
        });
    },
    onCancel() {
      this.confirmVisible = false;
      this.deleteIdx = -1;
    },
    onClickCloseBtn() {
      this.addFormVisible = false;
    },
    onClickCloseEditBtn() {
      this.editFormVisible = false;
    },
  },
});
</script>

<style scoped>
.ssl-toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px; margin-bottom: 20px; }
.ssl-guide { font-size: 13px; color: var(--el-text-color-secondary); }
.ssl-guide summary { cursor: pointer; padding-block: 6px; }
.ssl-guide[open] summary { margin-bottom: 12px; }
.ssl-guide summary:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: 2px; }
.ssl-toolbar-actions, .ssl-filter-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.ssl-toolbar-actions :deep(.el-button), .ssl-filter-actions :deep(.el-button) { margin-left: 0; }
.ssl-filters { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 16px; margin-block: 24px 20px; }
.ssl-filters label { display: flex; flex-direction: column; gap: 8px; font-size: 13px; color: var(--el-text-color-regular); }
.ssl-domain-filter { flex: 1 1 240px; max-width: 360px; }
.ssl-status-filter { flex: 0 1 180px; min-width: 150px; }
.search-input { width: 100%; }
.ssl-list-caption { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; color: var(--el-text-color-secondary); font-size: 12px; }
.ssl-list-error { color: var(--el-color-danger); }
.ssl-scope { font-size: 13px; color: var(--el-text-color-secondary); }
@media (max-width: 600px) { .ssl-domain-filter { max-width: none; flex-basis: 100%; } .ssl-status-filter { flex: 1; } .ssl-toolbar-actions { width: 100%; } }

.credential-replace { display: flex; align-items: center; gap: 8px; margin-block: 12px; line-height: 1.5; }
.hurricane-credential { padding-block: 12px; border-bottom: 1px solid var(--el-border-color-lighter); margin-bottom: 12px; }

@media (max-width: 600px) {
  .ssl-order-page :deep(.hb-row) { flex-wrap: wrap; align-items: flex-start; }
  .ssl-order-page :deep(.hb-txt) { flex-basis: calc(100% - 30px); min-width: 0; }
  .ssl-order-page :deep(.hb-actions) { margin-left: 24px; }
}
</style>
