<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container resource-panel">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <PageActions
            ><hw-button @click="handleAddSslConfig" theme="primary"> {{ $t('common.new') }} </hw-button></PageActions
          >
        </div>
        <div class="right-operation-container resource-filters">
          <hw-form
            ref="form"
            :data="searchformData"
            :label-width="300"
            layout="inline"
            colon
            :style="{ marginBottom: '8px' }"
          >
            <hw-form-item :label="$t('page.ssl.label_domains')" name="domains">
              <hw-input v-model="searchformData.domains" class="search-input" clearable></hw-input>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList('all')">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.ssl.alert_message')" doc="guide/SSL" />
      <ElAlert
        v-if="savedResult?.exportFailed"
        :title="savedResult.message"
        type="warning"
        show-icon
        @close="savedResult = null"
      />
      <div v-if="listError" role="alert" class="certificate-list-error">
        <span>{{ listError }}</span>
        <hw-button @click="getList('')">{{ $t('page.ssl.editor.retry') }}</hw-button>
      </div>
      <div v-if="!listError || data.length" class="table-container">
        <hw-table
          :columns="columns"
          :data="data"
          :rowKey="rowKey"
          :verticalAlign="verticalAlign"
          :hover="hover"
          :pagination="pagination"
          :selected-row-keys="selectedRowKeys"
          :loading="dataLoading"
          @page-change="rehandlePageChange"
          :headerAffixedTop="true"
        >
          <template #domain="{ row }">
            <span> {{ row.domain }}</span>
          </template>

          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleClickEdit(slotProps)">
              {{ $t('common.edit') }}
            </button>
            <button type="button" class="t-button-link" @click="handleClickDelete(slotProps)">
              {{ $t('common.delete') }}
            </button>
          </template>
        </hw-table>
      </div>
      <div>
        <router-view></router-view>
      </div>
    </hw-card>

    <CertificateEditor ref="certificateEditor" @saved="onCertificateSaved" />

    <hw-dialog
      :header="$t('common.confirm_delete')"
      :body="confirmBody"
      v-model:visible="confirmVisible"
      @confirm="onConfirmDelete"
      :onCancel="onCancel"
    >
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

import { sslConfigListApi, sslConfigDelApi } from '@/apis/sslconfig';
import { ElAlert } from 'element-plus';
import CertificateEditor from './CertificateEditor.vue';

export default defineComponent({
  name: 'SslConfigList',
  components: {
    ElAlert,
    CertificateEditor,
  },
  data() {
    return {
      confirmVisible: false,
      savedResult: null as null | { message: string; exportFailed: boolean },
      listError: '',
      listRequestVersion: 0,
      dataLoading: false,
      data: [], // 列表数据信息
      selectedRowKeys: [],
      columns: [
        {
          align: 'left',
          width: 200,
          colKey: 'op',
          title: this.$t('common.op'),
        },
        {
          title: this.$t('page.ssl.label_bind_hosts'),
          align: 'left',
          width: 300,
          ellipsis: true,
          colKey: 'bind_hosts',
          cell: (h, { row }) => {
            if (row.bind_hosts && row.bind_hosts.length > 0) {
              return row.bind_hosts.join(', ');
            }
            return '-';
          },
        },
        {
          title: this.$t('page.ssl.label_valid_from'),
          width: 200,
          ellipsis: true,
          colKey: 'valid_from',
        },
        {
          title: this.$t('page.ssl.label_valid_to'),
          width: 200,
          ellipsis: true,
          colKey: 'valid_to',
        },
        {
          title: this.$t('page.ssl.label_expire_time'),
          width: 200,
          ellipsis: true,
          colKey: 'expiration_info',
        },
        {
          title: this.$t('page.ssl.label_domains'),
          width: 250,
          ellipsis: true,
          colKey: 'domains',
        },
        {
          title: this.$t('page.ssl.label_subject'),
          align: 'left',
          width: 250,
          ellipsis: true,
          colKey: 'subject',
        },
        {
          title: this.$t('page.ssl.label_issuer'),
          align: 'left',
          width: 250,
          ellipsis: true,
          colKey: 'issuer',
        },
        {
          title: this.$t('page.ssl.label_serial_no'),
          align: 'left',
          width: 250,
          ellipsis: true,
          colKey: 'serial_no',
        },
        {
          title: 'id',
          align: 'left',
          width: 250,
          ellipsis: true,
          colKey: 'id',
        },
      ],
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
        domains: '',
      },
      deleteIdx: -1,
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
  mounted() {
    this.getList('');
  },
  methods: {
    rehandlePageChange(curr, pageInfo) {
      this.pagination.current = curr.current;
      if (this.pagination.pageSize != curr.pageSize) {
        this.pagination.current = 1;
        this.pagination.pageSize = curr.pageSize;
      }
      this.getList('');
    },
    async getList(keyword) {
      if (keyword === 'all') this.pagination.current = 1;
      const version = ++this.listRequestVersion;
      this.dataLoading = true;
      this.listError = '';
      try {
        const res = await sslConfigListApi({
          pageSize: this.pagination.pageSize,
          pageIndex: this.pagination.current,
          ...this.searchformData,
        });
        if (version !== this.listRequestVersion) return;
        if (res.code !== 0) throw new Error(res.msg || this.$t('page.ssl.editor.listFailed'));
        this.data = res.data.list ?? [];
        this.pagination = { ...this.pagination, total: res.data.total };
      } catch {
        if (version === this.listRequestVersion) this.listError = this.$t('page.ssl.editor.listFailed');
      } finally {
        if (version === this.listRequestVersion) this.dataLoading = false;
      }
    },
    handleAddSslConfig() {
      this.$refs.certificateEditor.open();
    },
    handleClickEdit({ row }) {
      this.$refs.certificateEditor.open(row);
    },
    onCertificateSaved(result) {
      this.savedResult = result;
      if (result.exportFailed) this.$message.warning(result.message);
      else this.$message.success(result.message);
      this.getList('');
    },
    handleClickDelete(slotProps) {
      const { row } = slotProps;
      this.deleteIdx = row.id;
      this.confirmVisible = true;
    },
    onConfirmDelete() {
      let that = this;
      sslConfigDelApi({
        id: this.deleteIdx,
      }).then((res) => {
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
  },
});
</script>

<style scoped>
.list-card-container {
  padding: 20px;
}
.table-container {
  margin-top: 20px;
}
.search-input {
  width: 200px;
}

.certificate-list-error {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
  color: var(--hw-danger);
}
</style>
