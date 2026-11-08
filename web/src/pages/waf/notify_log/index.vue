<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="right-operation-container">
          <hw-form ref="form" :data="searchformData" :label-width="80" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.notify_log.label_message_type')" name="message_type">
              <hw-select v-model="searchformData.message_type" clearable :style="{ width: '150px' }">
                <hw-option value="rule_trigger" :label="$t('page.notify_log.message_type_rule_trigger')"></hw-option>
                <hw-option value="operation_notice" :label="$t('page.notify_log.message_type_operation_notice')"></hw-option>
                <hw-option value="user_login" :label="$t('page.notify_log.message_type_user_login')"></hw-option>
                <hw-option value="attack_info" :label="$t('page.notify_log.message_type_attack_info')"></hw-option>
                <hw-option value="weekly_report" :label="$t('page.notify_log.message_type_weekly_report')"></hw-option>
                <hw-option value="ssl_expire" :label="$t('page.notify_log.message_type_ssl_expire')"></hw-option>
                <hw-option value="system_error" :label="$t('page.notify_log.message_type_system_error')"></hw-option>
                <hw-option value="ip_ban" :label="$t('page.notify_log.message_type_ip_ban')"></hw-option>
                <hw-option value="access_login" :label="$t('page.notify_log.message_type_access_login')"></hw-option>
                <hw-option value="access_abnormal" :label="$t('page.notify_log.message_type_access_abnormal')"></hw-option>
                <hw-option value="manage_login_abnormal" :label="$t('page.notify_log.message_type_manage_login_abnormal')"></hw-option>
              </hw-select>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_log.label_send_status')" name="status">
              <hw-select v-model="searchformData.status" clearable :style="{ width: '120px' }">
                <hw-option :value="1" :label="$t('page.notify_log.status_success')"></hw-option>
                <hw-option :value="0" :label="$t('page.notify_log.status_failed')"></hw-option>
                <!-- 被抑制的通知也留痕，这样"为什么没收到"能直接在这里查到 -->
                <hw-option :value="2" :label="$t('page.notify_log.status_suppressed')"></hw-option>
              </hw-select>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_log.label_start_time')" name="start_time">
              <hw-date-picker v-model="searchformData.start_time" enable-time-picker format="YYYY-MM-DD HH:mm:ss" :style="{ width: '200px' }"></hw-date-picker>
            </hw-form-item>
            <hw-form-item :label="$t('page.notify_log.label_end_time')" name="end_time">
              <hw-date-picker v-model="searchformData.end_time" enable-time-picker format="YYYY-MM-DD HH:mm:ss" :style="{ width: '200px' }"></hw-date-picker>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" @click="getList">{{ $t('common.search') }}</hw-button>
              <hw-button variant="base" theme="default" @click="handleReset" :style="{ marginLeft: '8px' }">{{ $t('common.reset') }}</hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.notify_log.alert_message')" doc="guide/NotifyLog" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :loading="dataLoading" @page-change="rehandlePageChange" @change="rehandleChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">
          <template #channel_type="{ row }">
            <hw-tag v-if="row.channel_type === 'dingtalk'" theme="primary">{{ $t('page.notify_channel.type_dingtalk') }}</hw-tag>
            <hw-tag v-else-if="row.channel_type === 'feishu'" theme="success">{{ $t('page.notify_channel.type_feishu') }}</hw-tag>
            <hw-tag v-else theme="default">{{ row.channel_type }}</hw-tag>
          </template>
          <template #message_type="{ row }">
            <hw-tag theme="primary">{{ getMessageTypeName(row.message_type) }}</hw-tag>
          </template>
          <template #status="{ row }">
            <hw-tag v-if="row.status === 1" theme="success">{{ $t('page.notify_log.status_success') }}</hw-tag>
            <hw-tag v-else-if="row.status === 2" theme="warning">{{ $t('page.notify_log.status_suppressed') }}</hw-tag>
            <hw-tag v-else theme="danger">{{ $t('page.notify_log.status_failed') }}</hw-tag>
          </template>
          <template #suppress_reason="{ row }">
            <span v-if="row.status !== 2">-</span>
            <span v-else>
              {{ getSuppressReasonName(row.suppress_reason) }}
              <hw-tag v-if="row.suppress_count > 1" theme="warning" variant="light" size="small">×{{ row.suppress_count }}</hw-tag>
            </span>
          </template>
          <template #op="slotProps">
            <a class="t-button-link" @click="handleViewDetail(slotProps)">{{ $t('page.notify_log.view_detail') }}</a>
            <a class="t-button-link" @click="handleClickDelete(slotProps)">{{ $t('common.delete') }}</a>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <!-- 日志详情对话框 -->
    <hw-dialog :header="$t('page.notify_log.log_detail')" v-model:visible="detailVisible" :width="800" :footer="false">
      <template #body><div >
        <hw-form :labelWidth="150" :style="{ marginTop: '16px' }">
          <hw-form-item :label="$t('page.notify_log.label_channel_name')">
            <span>{{ detailData.channel_name }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_channel_type')">
            <hw-tag v-if="detailData.channel_type === 'dingtalk'" theme="primary">{{ $t('page.notify_channel.type_dingtalk') }}</hw-tag>
            <hw-tag v-else-if="detailData.channel_type === 'feishu'" theme="success">{{ $t('page.notify_channel.type_feishu') }}</hw-tag>
            <hw-tag v-else theme="default">{{ detailData.channel_type }}</hw-tag>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_message_type')">
            <hw-tag theme="primary">{{ getMessageTypeName(detailData.message_type) }}</hw-tag>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_message_title')">
            <span>{{ detailData.message_title }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_message_content')">
            <hw-textarea :value="detailData.message_content" :autosize="{ minRows: 4, maxRows: 10 }" readonly></hw-textarea>
          </hw-form-item>
          <hw-form-item v-if="detailData.channel_type === 'email'" :label="$t('page.notify_log.label_recipients')">
            <span>{{ detailData.recipients || '-' }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_send_status')">
            <hw-tag v-if="detailData.status === 1" theme="success">{{ $t('page.notify_log.status_success') }}</hw-tag>
            <hw-tag v-else-if="detailData.status === 2" theme="warning">{{ $t('page.notify_log.status_suppressed') }}</hw-tag>
            <hw-tag v-else theme="danger">{{ $t('page.notify_log.status_failed') }}</hw-tag>
          </hw-form-item>
          <hw-form-item v-if="detailData.status === 2" :label="$t('page.notify_log.label_suppress_reason')">
            <span>{{ getSuppressReasonName(detailData.suppress_reason) }}</span>
            <div style="font-size: 12px; color: #666; margin-top: 4px;">
              {{ $t('page.notify_log.label_suppress_count') }}: {{ detailData.suppress_count || 1 }} —— {{ $t('page.notify_log.suppress_tips') }}
            </div>
          </hw-form-item>
          <hw-form-item v-if="detailData.template_used" :label="$t('page.notify_log.label_template_used')">
            <hw-tag :theme="detailData.template_used === 'custom_fallback' ? 'danger' : 'default'">
              {{ getTemplateUsedName(detailData.template_used) }}
            </hw-tag>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_error_msg')" v-if="detailData.status === 0 && detailData.error_msg">
            <hw-textarea :value="detailData.error_msg" :autosize="{ minRows: 2, maxRows: 5 }" readonly></hw-textarea>
          </hw-form-item>
          <hw-form-item :label="$t('page.notify_log.label_send_time')">
            <span>{{ detailData.send_time }}</span>
          </hw-form-item>
          <hw-form-item style="float: right; margin-top: 16px;">
            <hw-button variant="outline" @click="detailVisible = false">{{ $t('common.close') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- 删除确认对话框 -->
    <hw-dialog :header="$t('common.confirm_delete')" :body="confirmBody" v-model:visible="confirmVisible" @confirm="onConfirmDelete" :onCancel="onCancel">
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { MessagePlugin } from '@/ui';
import { getNotifyLogList, deleteNotifyLog } from '@/apis/notify_log';

export default defineComponent({
  name: 'NotifyLog',
  data() {
    return {
      data: [],
      dataLoading: false,
      columns: [
        { colKey: 'row-select', type: 'multiple', width: 50, fixed: 'left' },
        { title: this.$t('page.notify_log.label_channel_name'), colKey: 'channel_name', width: 150 },
        { title: this.$t('page.notify_log.label_channel_type'), colKey: 'channel_type', width: 120 },
        { title: this.$t('page.notify_log.label_message_type'), colKey: 'message_type', width: 150 },
        { title: this.$t('page.notify_log.label_message_title'), colKey: 'message_title', ellipsis: true, width: 200 },
        { title: this.$t('page.notify_log.label_send_status'), colKey: 'status', width: 100 },
        { title: this.$t('page.notify_log.label_suppress_reason'), colKey: 'suppress_reason', width: 160 },
        { title: this.$t('page.notify_log.label_send_time'), colKey: 'send_time', width: 180 },
        {
          align: 'left',
          fixed: 'right',
          width: 150,
          colKey: 'op',
          title: this.$t('common.op'),
        },
      ],
      rowKey: 'id',
      verticalAlign: 'top',
      hover: true,
      pagination: {
        pageSize: 20,
        total: 0,
        current: 1,
      },
      searchformData: {
        pageIndex: 1,
        pageSize: 20,
        message_type: '',
        status: undefined,
        start_time: '',
        end_time: '',
      },
      detailVisible: false,
      detailData: {} as any,
      confirmVisible: false,
      confirmBody: '',
      deleteId: '',
      offsetTop: 0,
    };
  },
  mounted() {
    this.getList();
  },
  methods: {
    getContainer() {
      return document.querySelector('.tdesign-starter-main');
    },
    async getList() {
      this.dataLoading = true;
      try {
        const res = await getNotifyLogList(this.searchformData);
        if (res.code === 0) {
          this.data = res.data.list || [];
          this.pagination.total = res.data.total;
          this.pagination.current = this.searchformData.pageIndex;
          this.pagination.pageSize = this.searchformData.pageSize;
        }
      } catch (e) {
        console.error(e);
      } finally {
        this.dataLoading = false;
      }
    },
    handleReset() {
      this.searchformData = {
        pageIndex: 1,
        pageSize: 20,
        message_type: '',
        status: undefined,
        start_time: '',
        end_time: '',
      };
      this.getList();
    },
    getMessageTypeName(type: string) {
      const typeMap: any = {
        rule_trigger: this.$t('page.notify_log.message_type_rule_trigger'),
        operation_notice: this.$t('page.notify_log.message_type_operation_notice'),
        user_login: this.$t('page.notify_log.message_type_user_login'),
        attack_info: this.$t('page.notify_log.message_type_attack_info'),
        weekly_report: this.$t('page.notify_log.message_type_weekly_report'),
        ssl_expire: this.$t('page.notify_log.message_type_ssl_expire'),
        system_error: this.$t('page.notify_log.message_type_system_error'),
        ip_ban: this.$t('page.notify_log.message_type_ip_ban'),
        access_login: this.$t('page.notify_log.message_type_access_login'),
        access_abnormal: this.$t('page.notify_log.message_type_access_abnormal'),
        manage_login_abnormal: this.$t('page.notify_log.message_type_manage_login_abnormal'),
      };
      return typeMap[type] || type;
    },
    // 抑制原因中文化：直接回答"为什么这条没发出去"
    getSuppressReasonName(reason: string) {
      const reasonMap: any = {
        cooldown: this.$t('page.notify_log.suppress_reason_cooldown'),
        rate_limit: this.$t('page.notify_log.suppress_reason_rate_limit'),
        quiet_hours: this.$t('page.notify_log.suppress_reason_quiet_hours'),
        filter_miss: this.$t('page.notify_log.suppress_reason_filter_miss'),
      };
      return reasonMap[reason] || reason || '-';
    },
    getTemplateUsedName(used: string) {
      const usedMap: any = {
        default: this.$t('page.notify_log.template_used_default'),
        custom: this.$t('page.notify_log.template_used_custom'),
        custom_fallback: this.$t('page.notify_log.template_used_fallback'),
      };
      return usedMap[used] || used;
    },
    rehandlePageChange(pageInfo: any) {
      this.searchformData.pageIndex = pageInfo.current;
      this.searchformData.pageSize = pageInfo.pageSize;
      this.getList();
    },
    rehandleChange(changeParams: any, triggerAndData: any) {
      console.log('统一Change', changeParams, triggerAndData);
    },
    handleViewDetail(e: any) {
      this.detailData = { ...e.row };
      this.detailVisible = true;
    },
    handleClickDelete(e: any) {
      this.deleteId = e.row.id;
      this.confirmBody = this.$t('page.notify_log.delete_confirm');
      this.confirmVisible = true;
    },
    async onConfirmDelete() {
      try {
        const res = await deleteNotifyLog({ id: this.deleteId });
        if (res.code === 0) {
          MessagePlugin.success(this.$t('page.notify_log.delete_success'));
          this.confirmVisible = false;
          this.getList();
        } else {
          MessagePlugin.error(res.msg || this.$t('page.notify_log.delete_failed'));
        }
      } catch (error) {
        MessagePlugin.error(this.$t('page.notify_log.delete_failed'));
      }
    },
    onCancel() {
      this.confirmVisible = false;
    },
  },
});
</script>

<style lang="less" scoped>
.list-card-container {
  padding: 16px;
}

.table-container {
  margin-top: 16px;
}
</style>
