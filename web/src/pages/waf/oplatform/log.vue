<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container"></div>
        <div class="right-operation-container">
          <hw-form ref="searchForm" :data="searchformData" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.oplatform.key_name')" name="key_name">
              <hw-input v-model="searchformData.key_name" style="width:160px" clearable />
            </hw-form-item>
            <hw-form-item :label="$t('page.oplatform.log_path')" name="request_path">
              <hw-input v-model="searchformData.request_path" style="width:200px" clearable />
            </hw-form-item>
            <hw-form-item :label="$t('page.oplatform.log_ip')" name="client_ip">
              <hw-input v-model="searchformData.client_ip" style="width:140px" clearable />
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>

      <div class="table-container">
        <hw-table
          :columns="columns"
          :data="data"
          rowKey="id"
          :hover="true"
          :pagination="pagination"
          :loading="dataLoading"
          @page-change="rehandlePageChange"
        >
          <template #status_code="slotProps">
            <hw-tag :theme="slotProps.row.status_code === 200 ? 'success' : 'warning'">
              {{ slotProps.row.status_code }}
            </hw-tag>
          </template>
          <template #duration="slotProps">
            {{ slotProps.row.duration }} ms
          </template>
          <template #op="slotProps">
            <a class="t-button-link" @click="handleDetail(slotProps)">{{ $t('common.details') }}</a>
            <a class="t-button-link" style="color: #e34d59" @click="handleDelete(slotProps)">{{ $t('common.delete') }}</a>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <!-- 删除确认对话框 -->
    <hw-dialog
      :header="$t('common.delete')"
      v-model:visible="confirmVisible"
      :width="400"
      @confirm="onConfirmDelete"
      @cancel="confirmVisible = false"
    >
      <template #body><div >{{ $t('page.oplatform.delete_confirm') }}</div></template>
    </hw-dialog>

    <!-- 日志详情对话框 -->
    <hw-dialog
      :header="$t('page.oplatform.log_detail')"
      v-model:visible="detailDialogVisible"
      :width="800"
      :cancel-btn="null"
      @confirm="detailDialogVisible = false"
    >
      <template #body><div >
        <hw-form :labelWidth="120">
          <hw-form-item :label="$t('page.oplatform.key_name')">
            <span>{{ detailData.key_name }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_path')">
            <span>{{ detailData.request_method }} {{ detailData.request_path }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_ip')">
            <span>{{ detailData.client_ip }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_status')">
            <span>{{ detailData.status_code }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_duration')">
            <span>{{ detailData.duration }} ms</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_time')">
            <span>{{ detailData.time_str }}</span>
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_request_body')">
            <hw-textarea :value="detailData.request_body" readonly :style="{ width: '580px' }" :autosize="{ minRows: 3, maxRows: 8 }" />
          </hw-form-item>
          <hw-form-item :label="$t('page.oplatform.log_response_body')">
            <hw-textarea :value="detailData.response_body" readonly :style="{ width: '580px' }" :autosize="{ minRows: 3, maxRows: 8 }" />
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>
  </div>
</template>

<script>
import { oplatform_log_list_api, oplatform_log_detail_api, oplatform_log_del_api } from '@/apis/oplatform'
import { MessagePlugin } from '@/ui'

export default {
  name: 'OPlatformLog',
  data() {
    return {
      dataLoading: false,
      data: [],
      pagination: {
        current: 1,
        pageSize: 10,
        total: 0
      },
      searchformData: {
        key_name: '',
        request_path: '',
        client_ip: ''
      },
      columns: [
        { colKey: 'key_name', title: this.$t('page.oplatform.key_name'), width: 130 },
        { colKey: 'request_method', title: this.$t('page.oplatform.log_method'), width: 80 },
        { colKey: 'request_path', title: this.$t('page.oplatform.log_path'), width: 220, ellipsis: true },
        { colKey: 'client_ip', title: this.$t('page.oplatform.log_ip'), width: 130 },
        { colKey: 'status_code', title: this.$t('page.oplatform.log_status'), width: 90 },
        { colKey: 'duration', title: this.$t('page.oplatform.log_duration'), width: 100 },
        { colKey: 'time_str', title: this.$t('page.oplatform.log_time'), width: 160 },
        { colKey: 'op', title: this.$t('common.operation'), width: 130, fixed: 'right' }
      ],
      detailDialogVisible: false,
      detailData: {},
      confirmVisible: false,
      confirmDeleteId: ''
    }
  },
  mounted() {
    this.getList()
  },
  methods: {
    getList() {
      this.dataLoading = true
      oplatform_log_list_api({
        pageIndex: this.pagination.current,
        pageSize: this.pagination.pageSize,
        key_name: this.searchformData.key_name,
        request_path: this.searchformData.request_path,
        client_ip: this.searchformData.client_ip
      }).then((res) => {
        this.dataLoading = false
        if (res && res.code === 0) {
          this.data = res.data.list || []
          this.pagination.total = res.data.total
        }
      }).catch(() => { this.dataLoading = false })
    },
    handleDetail(slotProps) {
      oplatform_log_detail_api({ id: slotProps.row.id }).then((res) => {
        if (res && res.code === 0) {
          this.detailData = res.data
          this.detailDialogVisible = true
        }
      })
    },
    handleDelete(slotProps) {
      this.confirmDeleteId = slotProps.row.id
      this.confirmVisible = true
    },
    onConfirmDelete() {
      this.confirmVisible = false
      oplatform_log_del_api({ id: this.confirmDeleteId }).then((res) => {
        if (res && res.code === 0) {
          MessagePlugin.success(this.$t('page.oplatform.delete_success'))
          this.getList()
        }
      })
    },
    rehandlePageChange(pagination) {
      this.pagination.current = pagination.current
      this.pagination.pageSize = pagination.pageSize
      this.getList()
    }
  }
}
</script>

<style scoped>
.list-card-container {
  padding: 16px;
}
.left-operation-container {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.right-operation-container {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
</style>
