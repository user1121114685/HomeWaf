<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button @click="handleAdd">{{ $t('page.path_rule.button_add') }}</hw-button>
        </div>
        <div class="right-operation-container">
          <hw-form ref="searchForm" :data="searchformData" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.path_rule.alert_message')" />
      <div class="table-container">
        <hw-table
          :columns="columns"
          :data="data"
          :rowKey="rowKey"
          :verticalAlign="verticalAlign"
          :hover="hover"
          :pagination="pagination"
          :loading="dataLoading"
          @page-change="rehandlePageChange"
          :headerAffixedTop="true"
          :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }"
        >
          <template #match_type="{ row }">
            <span>{{ matchTypeLabel(row.match_type) }}</span>
          </template>
          <template #target_type="{ row }">
            <hw-tag :theme="targetTypeTheme(row.target_type)" variant="light">
              {{ targetTypeLabel(row.target_type) }}
            </hw-tag>
          </template>
          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleClickEdit(slotProps)">{{ $t('common.edit') }}</button>
            <button type="button" class="t-button-link" @click="handleClickDelete(slotProps)">{{ $t('common.delete') }}</button>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <!-- Add Dialog -->
    <hw-dialog :header="$t('common.new')" v-model:visible="addFormVisible" :width="720" :footer="false">
      <template #body><div >
        <hw-form :data="formData" ref="addForm" :rules="rules" @submit="onSubmit" :labelWidth="120">
          <hw-form-item :label="$t('page.path_rule.host_code')" name="host_code">
            <hw-select v-model="formData.host_code" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in host_dic" :value="index" :label="item" :key="index">{{ item }}</hw-option>
            </hw-select>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.rule_name')" name="rule_name">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.rule_name" />
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.path')" name="path">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.path" :placeholder="$t('page.path_rule.path_placeholder')" />
            <div class="form-item-tips">{{ $t('page.path_rule.path_tips') }}</div>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.match_type')" name="match_type">
            <hw-select v-model="formData.match_type" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="item in match_type_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.priority')" name="priority">
            <hw-input-number :style="{ width: '150px' }" v-model="formData.priority" />
            <div class="form-item-tips">{{ $t('page.path_rule.priority_tips') }}</div>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.target_type')" name="target_type">
            <hw-select v-model="formData.target_type" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="item in target_type_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
          </hw-form-item>

          <!-- strip_prefix: 代理 or 静态文件时显示 -->
          <hw-form-item :label="$t('page.path_rule.strip_prefix')" name="strip_prefix" v-if="formData.target_type === '1' || formData.target_type === '2'">
            <hw-radio-group v-model="formData.strip_prefix">
              <hw-radio value="0">{{ $t('common.no') }}</hw-radio>
              <hw-radio value="1">{{ $t('common.yes') }}</hw-radio>
            </hw-radio-group>
            <div class="form-item-tips">{{ $t('page.path_rule.strip_prefix_tips') }}</div>
          </hw-form-item>

          <!-- target_type=1: 后端代理 -->
          <template v-if="formData.target_type === '1'">
            <hw-form-item :label="$t('page.path_rule.remote_scheme')" name="remote_scheme">
              <hw-select v-model="formData.remote_scheme" :style="{ width: 'min(100%, 200px)' }">
                <hw-option v-for="item in scheme_options" :value="item.value" :label="item.label" :key="item.value" />
              </hw-select>
              <div class="form-item-tips">{{ $t('page.path_rule.remote_scheme_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_host')" name="remote_host">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remote_host" :placeholder="$t('page.path_rule.remote_host_placeholder')" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_port')" name="remote_port">
              <hw-input-number :style="{ width: '150px' }" v-model="formData.remote_port" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_ip')" name="remote_ip">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remote_ip" :placeholder="$t('page.path_rule.remote_ip_placeholder')" />
              <div class="form-item-tips">{{ $t('page.path_rule.remote_ip_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.response_time_out')" name="response_time_out">
              <hw-input-number :style="{ width: '150px' }" v-model="formData.response_time_out" :min="0" />
              <div class="form-item-tips">{{ $t('page.path_rule.response_time_out_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.record_access_log')" name="record_access_log">
              <hw-radio-group v-model="formData.record_access_log">
                <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
                <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
              </hw-radio-group>
              <div class="form-item-tips">{{ $t('page.path_rule.record_access_log_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.preview')">
              <hw-alert theme="info">
                <template #message>
                  <div>{{ $t('page.path_rule.preview_client') }}：{{ addPreview.clientUrl }}</div>
                  <div>{{ $t('page.path_rule.preview_backend') }}：{{ addPreview.backendUrl }}{{ addPreview.stripNote }}</div>
                  <div v-if="addPreview.ipLine">{{ addPreview.ipLine }}</div>
                </template>
              </hw-alert>
            </hw-form-item>
          </template>

          <!-- target_type=2: 静态文件 -->
          <template v-if="formData.target_type === '2'">
            <hw-form-item :label="$t('page.path_rule.static_root')" name="static_root">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.static_root" :placeholder="$t('page.path_rule.static_root_placeholder')" />
              <div class="form-item-tips">{{ $t('page.path_rule.static_root_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.spa_fallback')" name="spa_fallback">
              <hw-radio-group v-model="formData.spa_fallback">
                <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
                <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
              </hw-radio-group>
              <div class="form-item-tips">{{ $t('page.path_rule.spa_fallback_tips') }}</div>
            </hw-form-item>
          </template>

          <!-- target_type=3: 重定向 -->
          <template v-if="formData.target_type === '3'">
            <hw-form-item :label="$t('page.path_rule.redirect_url')" name="redirect_url">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.redirect_url" :placeholder="$t('page.path_rule.redirect_url_placeholder')" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.redirect_code')" name="redirect_code">
              <hw-select v-model="formData.redirect_code" :style="{ width: 'min(100%, 200px)' }">
                <hw-option value="302" label="302 Found" />
                <hw-option value="301" label="301 Moved Permanently" />
              </hw-select>
            </hw-form-item>
          </template>

          <hw-form-item :label="$t('page.path_rule.remarks')" name="remarks">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formData.remarks" />
          </hw-form-item>

          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- Edit Dialog -->
    <hw-dialog :header="$t('common.edit')" v-model:visible="editFormVisible" :width="720" :footer="false">
      <template #body><div >
        <hw-form :data="formEditData" ref="editForm" :rules="rules" @submit="onSubmitEdit" :labelWidth="120">
          <hw-form-item :label="$t('page.path_rule.host_code')" name="host_code">
            <hw-select v-model="formEditData.host_code" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="(item, index) in host_dic" :value="index" :label="item" :key="index">{{ item }}</hw-option>
            </hw-select>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.rule_name')" name="rule_name">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.rule_name" />
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.path')" name="path">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.path" :placeholder="$t('page.path_rule.path_placeholder')" />
            <div class="form-item-tips">{{ $t('page.path_rule.path_tips') }}</div>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.match_type')" name="match_type">
            <hw-select v-model="formEditData.match_type" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="item in match_type_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.priority')" name="priority">
            <hw-input-number :style="{ width: '150px' }" v-model="formEditData.priority" />
            <div class="form-item-tips">{{ $t('page.path_rule.priority_tips') }}</div>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.target_type')" name="target_type">
            <hw-select v-model="formEditData.target_type" clearable :style="{ width: 'min(100%, 480px)' }">
              <hw-option v-for="item in target_type_options" :value="item.value" :label="item.label" :key="item.value" />
            </hw-select>
          </hw-form-item>

          <hw-form-item :label="$t('page.path_rule.strip_prefix')" name="strip_prefix" v-if="formEditData.target_type === '1' || formEditData.target_type === '2'">
            <hw-radio-group v-model="formEditData.strip_prefix">
              <hw-radio value="0">{{ $t('common.no') }}</hw-radio>
              <hw-radio value="1">{{ $t('common.yes') }}</hw-radio>
            </hw-radio-group>
            <div class="form-item-tips">{{ $t('page.path_rule.strip_prefix_tips') }}</div>
          </hw-form-item>

          <template v-if="formEditData.target_type === '1'">
            <hw-form-item :label="$t('page.path_rule.remote_scheme')" name="remote_scheme">
              <hw-select v-model="formEditData.remote_scheme" :style="{ width: 'min(100%, 200px)' }">
                <hw-option v-for="item in scheme_options" :value="item.value" :label="item.label" :key="item.value" />
              </hw-select>
              <div class="form-item-tips">{{ $t('page.path_rule.remote_scheme_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_host')" name="remote_host">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.remote_host" :placeholder="$t('page.path_rule.remote_host_placeholder')" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_port')" name="remote_port">
              <hw-input-number :style="{ width: '150px' }" v-model="formEditData.remote_port" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.remote_ip')" name="remote_ip">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.remote_ip" :placeholder="$t('page.path_rule.remote_ip_placeholder')" />
              <div class="form-item-tips">{{ $t('page.path_rule.remote_ip_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.response_time_out')" name="response_time_out">
              <hw-input-number :style="{ width: '150px' }" v-model="formEditData.response_time_out" :min="0" />
              <div class="form-item-tips">{{ $t('page.path_rule.response_time_out_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.record_access_log')" name="record_access_log">
              <hw-radio-group v-model="formEditData.record_access_log">
                <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
                <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
              </hw-radio-group>
              <div class="form-item-tips">{{ $t('page.path_rule.record_access_log_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.preview')">
              <hw-alert theme="info">
                <template #message>
                  <div>{{ $t('page.path_rule.preview_client') }}：{{ editPreview.clientUrl }}</div>
                  <div>{{ $t('page.path_rule.preview_backend') }}：{{ editPreview.backendUrl }}{{ editPreview.stripNote }}</div>
                  <div v-if="editPreview.ipLine">{{ editPreview.ipLine }}</div>
                </template>
              </hw-alert>
            </hw-form-item>
          </template>

          <template v-if="formEditData.target_type === '2'">
            <hw-form-item :label="$t('page.path_rule.static_root')" name="static_root">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.static_root" :placeholder="$t('page.path_rule.static_root_placeholder')" />
              <div class="form-item-tips">{{ $t('page.path_rule.static_root_tips') }}</div>
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.spa_fallback')" name="spa_fallback">
              <hw-radio-group v-model="formEditData.spa_fallback">
                <hw-radio value="0">{{ $t('common.off') }}</hw-radio>
                <hw-radio value="1">{{ $t('common.on') }}</hw-radio>
              </hw-radio-group>
              <div class="form-item-tips">{{ $t('page.path_rule.spa_fallback_tips') }}</div>
            </hw-form-item>
          </template>

          <template v-if="formEditData.target_type === '3'">
            <hw-form-item :label="$t('page.path_rule.redirect_url')" name="redirect_url">
              <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.redirect_url" :placeholder="$t('page.path_rule.redirect_url_placeholder')" />
            </hw-form-item>
            <hw-form-item :label="$t('page.path_rule.redirect_code')" name="redirect_code">
              <hw-select v-model="formEditData.redirect_code" :style="{ width: 'min(100%, 200px)' }">
                <hw-option value="302" label="302 Found" />
                <hw-option value="301" label="301 Moved Permanently" />
              </hw-select>
            </hw-form-item>
          </template>

          <hw-form-item :label="$t('page.path_rule.remarks')" name="remarks">
            <hw-input :style="{ width: 'min(100%, 480px)' }" v-model="formEditData.remarks" />
          </hw-form-item>

          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="onClickCloseEditBtn">{{ $t('common.close') }}</hw-button>
            <hw-button theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- Delete Confirm -->
    <hw-dialog
      :header="$t('common.confirm_delete')"
      :body="$t('common.data_delete_warning')"
      v-model:visible="confirmVisible"
      @confirm="onConfirmDelete"
      :onCancel="onCancel"
    />
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { prefix } from '@/config/global';
import { allhost } from '@/apis/host';
import {
  wafPathRuleListApi,
  wafPathRuleDelApi,
  wafPathRuleEditApi,
  wafPathRuleAddApi,
  wafPathRuleDetailApi,
} from '@/apis/path_rule.ts';

const INITIAL_DATA = {
  host_code: '',
  rule_name: '',
  path: '',
  match_type: '1',
  priority: '100',
  target_type: '1',
  strip_prefix: '0',
  remote_host: '',
  remote_port: '80',
  remote_ip: '',
  remote_scheme: 'auto',
  record_access_log: '0',
  response_time_out: '0',
  static_root: '',
  spa_fallback: '0',
  redirect_url: '',
  redirect_code: '302',
  remarks: '',
};

export default defineComponent({
  name: 'PathRuleBase',
  props: {
    propHostCode: String,
  },
  data() {
    return {
      addFormVisible: false,
      editFormVisible: false,
      confirmVisible: false,
      formData: { ...INITIAL_DATA },
      formEditData: { ...INITIAL_DATA },
      rules: {
        host_code: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.path_rule.host_code'), type: 'error' }],
        rule_name: [{ required: true, message: this.$t('common.placeholder') + this.$t('page.path_rule.rule_name'), type: 'error' }],
        path:      [{ required: true, message: this.$t('common.placeholder') + this.$t('page.path_rule.path'), type: 'error' }],
        match_type:[{ required: true, message: this.$t('common.placeholder') + this.$t('page.path_rule.match_type'), type: 'error' }],
        target_type:[{ required: true, message: this.$t('common.placeholder') + this.$t('page.path_rule.target_type'), type: 'error' }],
      },
      prefix,
      dataLoading: false,
      data: [],
      deleteIdx: -1,
      host_dic: {},
      searchformData: { host_code: '' },
      pagination: { total: 0, current: 1, pageSize: 10 },
      rowKey: 'id',
      verticalAlign: 'top',
      hover: true,
      columns: [
        { title: this.$t('page.path_rule.rule_name'), colKey: 'rule_name', width: 160, ellipsis: true },
        { title: this.$t('page.path_rule.path'),      colKey: 'path',      width: 200, ellipsis: true },
        { title: this.$t('page.path_rule.match_type'),colKey: 'match_type',width: 110 },
        { title: this.$t('page.path_rule.target_type'),colKey: 'target_type',width: 110 },
        { title: this.$t('page.path_rule.priority'),  colKey: 'priority',  width: 80 },
        { align: 'left', fixed: 'right', width: 160,  colKey: 'op', title: this.$t('common.op') },
      ],
      match_type_options: [
        { label: this.$t('page.path_rule.match_type_prefix'), value: '1' },
        { label: this.$t('page.path_rule.match_type_exact'),  value: '2' },
        { label: this.$t('page.path_rule.match_type_regex'),  value: '3' },
      ],
      target_type_options: [
        { label: this.$t('page.path_rule.target_type_proxy'),  value: '1' },
        { label: this.$t('page.path_rule.target_type_static'), value: '2' },
        { label: this.$t('page.path_rule.target_type_redirect'),value: '3' },
      ],
      scheme_options: [
        { label: this.$t('page.path_rule.scheme_auto'), value: 'auto' },
        { label: 'HTTP',  value: 'http' },
        { label: 'HTTPS', value: 'https' },
      ],
    };
  },
  watch: {
    propHostCode(newVal) {
      this.searchformData.host_code = newVal;
      this.getList();
    },
  },
  computed: {
    offsetTop() {
      return this.$store.state.setting.isUseTabsRouter ? 48 : 0;
    },
    addPreview() {
      return this.buildPreview(this.formData);
    },
    editPreview() {
      return this.buildPreview(this.formEditData);
    },
  },
  created() {
    this.searchformData.host_code = this.propHostCode;
  },
  mounted() {
    this.loadHostList().then(() => this.getList());
  },
  methods: {
    matchTypeLabel(val) {
      const map = { 1: this.$t('page.path_rule.match_type_prefix'), 2: this.$t('page.path_rule.match_type_exact'), 3: this.$t('page.path_rule.match_type_regex') };
      return map[val] || val;
    },
    targetTypeLabel(val) {
      const map = { 1: this.$t('page.path_rule.target_type_proxy'), 2: this.$t('page.path_rule.target_type_static'), 3: this.$t('page.path_rule.target_type_redirect') };
      return map[val] || val;
    },
    targetTypeTheme(val) {
      const map = { 1: 'primary', 2: 'success', 3: 'warning' };
      return map[val] || 'default';
    },
    loadHostList() {
      return new Promise((resolve, reject) => {
        allhost()
          .then((res) => {
            if (res.code === 0) {
              res.data.forEach((item) => { this.host_dic[item.value] = item.label; });
            }
            resolve(null);
          })
          .catch((e) => { console.log(e); reject(e); });
      });
    },
    getList() {
      this.dataLoading = true;
      wafPathRuleListApi({
        pageSize: this.pagination.pageSize,
        pageIndex: this.pagination.current,
        ...this.searchformData,
      })
        .then((res) => {
          if (res.code === 0) {
            this.data = res.data.list ?? [];
            this.pagination = { ...this.pagination, total: res.data.total };
          }
        })
        .catch((e) => console.log(e))
        .finally(() => { this.dataLoading = false; });
    },
    getContainer() {
      return document.querySelector('.tdesign-starter-layout');
    },
    rehandlePageChange(curr) {
      this.pagination.current = curr.current;
      if (this.pagination.pageSize !== curr.pageSize) {
        this.pagination.current = 1;
        this.pagination.pageSize = curr.pageSize;
      }
      this.getList();
    },
    handleAdd() {
      this.formData = { ...INITIAL_DATA };
      this.formData.host_code = this.propHostCode || '';
      this.addFormVisible = true;
    },
    handleClickEdit(e) {
      this.editFormVisible = true;
      this.getDetail(e.row.id);
    },
    handleClickDelete(row) {
      this.deleteIdx = row.rowIndex;
      this.confirmVisible = true;
    },
    onSubmit({ firstError }) {
      if (firstError) { this.$message.warning(firstError); return; }
      const postdata = this.buildPostData(this.formData);
      wafPathRuleAddApi(postdata)
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
            this.addFormVisible = false;
            this.pagination.current = 1;
            this.getList();
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch((e) => console.log(e));
    },
    onSubmitEdit({ firstError }) {
      if (firstError) { this.$message.warning(firstError); return; }
      const postdata = this.buildPostData(this.formEditData);
      wafPathRuleEditApi(postdata)
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
            this.editFormVisible = false;
            this.getList();
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch((e) => console.log(e));
    },
    buildPostData(src) {
      const d = { ...src };
      d['match_type']   = Number(d['match_type']);
      d['priority']     = Number(d['priority']);
      d['target_type']  = Number(d['target_type']);
      d['strip_prefix'] = Number(d['strip_prefix']);
      d['spa_fallback'] = Number(d['spa_fallback']);
      d['remote_port']  = Number(d['remote_port']);
      d['redirect_code']= Number(d['redirect_code']);
      d['record_access_log'] = Number(d['record_access_log']);
      d['response_time_out'] = Number(d['response_time_out']);
      return d;
    },
    // 实时预览：根据表单拼出「客户端请求」与「实际转发到」两行示例，纯前端展示转发效果
    buildPreview(form) {
      const domain = this.host_dic[form.host_code] || this.$t('page.path_rule.preview_your_domain');
      const p = form.path || '/';
      const sampleUrl = p.endsWith('/') ? `${p}foo` : `${p}/foo`;
      const clientUrl = `http(s)://${domain}${sampleUrl}`;
      // 后端协议：auto=跟随客户端，否则用显式协议
      let schemeText;
      if (form.remote_scheme === 'http') schemeText = 'http';
      else if (form.remote_scheme === 'https') schemeText = 'https';
      else schemeText = this.$t('page.path_rule.preview_follow_client');
      // 去前缀后的后端路径
      let backendPath = sampleUrl;
      if (String(form.strip_prefix) === '1' && sampleUrl.startsWith(p)) {
        backendPath = sampleUrl.slice(p.length);
        if (!backendPath.startsWith('/')) backendPath = `/${backendPath}`;
      }
      const host = form.remote_host || 'backend';
      const port = form.remote_port ? `:${form.remote_port}` : '';
      const backendUrl = `${schemeText}://${host}${port}${backendPath}`;
      const stripNote = String(form.strip_prefix) === '1' ? `　（${this.$t('page.path_rule.preview_stripped')}${p}）` : '';
      const ipLine = form.remote_ip
        ? `${this.$t('page.path_rule.preview_ip')}${form.remote_ip}${port}`
        : '';
      return { clientUrl, backendUrl, stripNote, ipLine };
    },
    onClickCloseBtn() { this.addFormVisible = false; },
    onClickCloseEditBtn() { this.editFormVisible = false; },
    onConfirmDelete() {
      this.confirmVisible = false;
      const { id } = this.data[this.deleteIdx];
      wafPathRuleDelApi({ id })
        .then((res) => {
          if (res.code === 0) { this.$message.success(res.msg); this.getList(); }
          else { this.$message.warning(res.msg); }
        })
        .catch((e) => console.log(e));
      this.deleteIdx = -1;
    },
    onCancel() { this.deleteIdx = -1; },
    getDetail(id) {
      wafPathRuleDetailApi({ id })
        .then((res) => {
          if (res.code === 0) {
            const d = res.data;
            // convert number fields to string for form selects/radios
            ['match_type','priority','target_type','strip_prefix','spa_fallback','remote_port','redirect_code','record_access_log','response_time_out'].forEach((k) => {
              if (d[k] !== undefined && d[k] !== null) d[k] = d[k].toString();
            });
            // 存量规则 remote_scheme 为空串，归一为 auto（跟随客户端）
            if (!d['remote_scheme']) d['remote_scheme'] = 'auto';
            this.formEditData = { ...INITIAL_DATA, ...d };
          }
        })
        .catch((e) => console.log(e));
    },
  },
});
</script>

<style lang="less" scoped>
@import '@/style/variables';

.left-operation-container {
  padding: 0 0 6px 0;
  margin-bottom: 16px;
}

.t-button + .t-button {
  margin-left: @spacer;
}

.form-item-tips {
  color: var(--hw-muted);
  font-size: 13px;
  margin-top: 4px;
  line-height: 1.5;
}
</style>
