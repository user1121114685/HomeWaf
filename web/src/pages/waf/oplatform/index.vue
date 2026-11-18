<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div>
    <hw-card class="list-card-container resource-panel">
      <div class="op-master">
        <div class="op-master__box">
          <hw-switch :model-value="openPlatformEnabled" :aria-label="$t('page.oplatform.master_switch')"
            :loading="masterLoading" :disabled="masterLoading || !masterKnown" @change="handleToggleMaster" />
          <span class="op-master__t">{{ $t('page.oplatform.master_switch') }}</span>
          <hw-tag v-if="masterKnown" :theme="openPlatformEnabled ? 'success' : 'warning'">
            {{ $t(openPlatformEnabled ? 'page.oplatform.master_on' : 'page.oplatform.master_off') }}
          </hw-tag>
          <span v-if="masterError" role="alert">{{ masterError }}</span>
          <hw-button v-if="masterError" variant="text" @click="fetchMasterSwitch">{{ $t('page.oplatform.retry') }}</hw-button>
          <span v-else class="op-master__hint">{{ openPlatformEnabled
            ? $t('page.oplatform.master_until', { time: new Date(enabledUntil * 1000).toLocaleString() })
            : $t('page.oplatform.master_window') }}</span>
        </div>
      </div>

      <hw-row justify="space-between">
        <div class="left-operation-container">
          <PageActions><hw-button theme="primary" @click="handleAdd">{{ $t('page.oplatform.new_key') }}</hw-button><RouterLink class="el-button" to="/oplatform/mcp">MCP 接入</RouterLink></PageActions>
        </div>
        <div class="right-operation-container resource-filters">
          <hw-form ref="searchForm" :data="searchformData" layout="inline" colon :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.oplatform.key_name')" name="key_name">
              <hw-input v-model="searchformData.key_name" style="width: min(100%, 200px)" clearable />
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>

      <help-block
        :summary="$t('page.oplatform.help_summary')"
        :items="helpItems"
        :note="$t('page.oplatform.help_note')"
        :title="$t('page.oplatform.help_title')"
        doc="/#/help"
        storage-key="oplatform-key"
      />

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
          <template #api_key="slotProps">
            <span>{{ slotProps.row.api_key }}</span>
          </template>
          <template #mcp_access="{ row }">
            {{ row.mcp_access === 'write' ? 'MCP · 排障与配置' : row.mcp_access === 'read' ? 'MCP · 只读' : '开放 API' }}
          </template>
          <template #status="slotProps">
            <hw-tag :theme="slotProps.row.status === 1 ? 'success' : 'danger'">
              {{ slotProps.row.status === 1 ? $t('page.oplatform.status_enabled') : $t('page.oplatform.status_disabled') }}
            </hw-tag>
          </template>
          <template #rate_limit="slotProps">
            {{ slotProps.row.rate_limit > 0 ? slotProps.row.rate_limit + ' ' + $t('page.oplatform.per_minute') : $t('page.oplatform.no_limit') }}
          </template>
          <template #op="slotProps">
            <button type="button" class="t-button-link" @click="handleEdit(slotProps)">{{ $t('common.edit') }}</button>
            <button type="button" class="t-button-link" @click="handleToggleStatus(slotProps)">
              {{ slotProps.row.status === 1 ? $t('page.oplatform.disable') : $t('page.oplatform.enable') }}
            </button>
            <button type="button" class="t-button-link" @click="handleResetSecret(slotProps)">{{ $t('page.oplatform.reset_key') }}</button>
            <button type="button" class="t-button-link" style="color: #e34d59" @click="handleDelete(slotProps)">{{ $t('common.delete') }}</button>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <KeyEditor v-if="editorVisible" :initial="editingKey" :suggested-source="suggestedSource"
      @close="editorVisible = false" @saved="keySaved" />

    <!-- 新建/重置 Key 结果对话框 -->
    <hw-dialog
      :header="secretDialogTitle"
      v-model:visible="secretDialogVisible"
      :width="520"
      :cancel-btn="null"
      @confirm="secretDialogVisible = false"
      @closed="newSecret = ''"
    >
      <template #body><div >
        <hw-alert theme="warning" :message="$t('page.oplatform.key_once_warning')" style="margin-bottom:16px" />
        <hw-form :labelWidth="120">
          <hw-form-item label="API Key">
            <hw-input v-model="newSecret" readonly :style="{ width: 'min(100%, 300px)' }">
              <template #suffix>
                <hw-button size="small" variant="text" @click="copyText(newSecret)">{{ $t('page.oplatform.copy') }}</hw-button>
              </template>
            </hw-input>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

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
  </div>
</template>

<script>
import {
  oplatform_key_list_api,
  oplatform_doc_status_api,
  oplatform_key_edit_api,
  oplatform_key_del_api,
  oplatform_key_reset_secret_api,
  oplatform_key_detail_api
} from '@/apis/oplatform'
import { edit_system_config_by_item_api } from '@/apis/systemconfig'
import { MessagePlugin } from '@/ui'
import KeyEditor from './KeyEditor.vue'

// 开放平台总开关在「参数配置」里的键名。它是个普通的系统配置项，
// 后端 editByItem 改完会立刻 TaskLoadSetting，不需要重启。
const MASTER_ITEM = 'open_platform_enabled'

export default {
  name: 'OPlatformKey',
  components: { KeyEditor },
  data() {
    return {
      dataLoading: false,
      openPlatformEnabled: false,
      masterLoading: false,
      masterKnown: false,
      masterError: '',
      enabledUntil: 0,
      expiryTimer: 0,
      suggestedSource: '',
      active: true,
      data: [],
      pagination: {
        current: 1,
        pageSize: 10,
        total: 0
      },
      searchformData: {
        key_name: ''
      },
      columns: [
        { colKey: 'key_name', title: this.$t('page.oplatform.key_name'), width: 150 },
        { colKey: 'api_key', title: 'API Key', width: 240 },
        { colKey: 'mcp_access', title: '用途 / 权限', width: 160 },
        { colKey: 'status', title: this.$t('page.oplatform.status'), width: 100 },
        { colKey: 'rate_limit', title: this.$t('page.oplatform.rate_limit'), width: 120 },
        { colKey: 'ip_whitelist', title: this.$t('page.oplatform.ip_whitelist'), width: 160, ellipsis: true },
        { colKey: 'call_count', title: this.$t('page.oplatform.call_count'), width: 100 },
        { colKey: 'last_use_time', title: this.$t('page.oplatform.last_use_time'), width: 160 },
        { colKey: 'expire_time', title: this.$t('page.oplatform.expire_time'), width: 150 },
        { colKey: 'op', title: this.$t('common.operation'), width: 260, fixed: 'right' }
      ],
      editorVisible: false,
      editingKey: undefined,
      secretDialogVisible: false,
      confirmVisible: false,
      confirmDeleteId: '',
      newSecret: '',
      secretDialogTitle: '',
    }
  },
  computed: {
    // 说明条目挑的是实际会被问到的：开关没开、请求头怎么带、为什么返回不是密文、
    // 哪些接口用 Key 调不了。不是把 API 文档抄一遍。
    helpItems() {
      return [
        { k: this.$t('page.oplatform.help_k_switch'), v: this.$t('page.oplatform.help_v_switch'), tone: 'brand' },
        { k: this.$t('page.oplatform.help_k_header'), v: this.$t('page.oplatform.help_v_header'), tone: 'brand' },
        { k: this.$t('page.oplatform.help_k_limit'), v: this.$t('page.oplatform.help_v_limit') },
        { k: this.$t('page.oplatform.help_k_plain'), v: this.$t('page.oplatform.help_v_plain') },
        { k: this.$t('page.oplatform.help_k_forbid'), v: this.$t('page.oplatform.help_v_forbid'), tone: 'danger' },
      ]
    }
  },
  mounted() {
    this.getList()
    this.fetchMasterSwitch()
  },
  beforeUnmount() {
    this.active = false
    clearTimeout(this.expiryTimer)
  },
  methods: {
    async fetchMasterSwitch() {
      this.masterLoading = true
      this.masterError = ''
      try {
        const res = await oplatform_doc_status_api()
        if (res.code !== 0 || ![0, 1].includes(res.data?.enabled)) throw new Error(res.msg || this.$t('page.oplatform.status_failed'))
        if (!this.active) return
        this.openPlatformEnabled = res.data.enabled === 1
        this.masterKnown = true
        this.suggestedSource = res.data.suggested_source || ''
        this.enabledUntil = res.data.enabled_until
        clearTimeout(this.expiryTimer)
        if (this.openPlatformEnabled) {
          this.expiryTimer = setTimeout(this.fetchMasterSwitch, Math.max(1000, this.enabledUntil * 1000 - Date.now()))
        }
      } catch (error) {
        this.masterKnown = false
        this.masterError = error.message || this.$t('page.oplatform.status_failed')
      } finally {
        this.masterLoading = false
      }
    },
    async handleToggleMaster(value) {
      if (this.masterLoading || !this.masterKnown) return
      this.masterLoading = true
      clearTimeout(this.expiryTimer)
      try {
        const res = await edit_system_config_by_item_api({ item: MASTER_ITEM, value: value ? '1' : '0' })
        if (res.code !== 0) throw new Error(res.msg || this.$t('common.tips.save_failed'))
        MessagePlugin.success(this.$t('common.tips.save_success'))
      } catch (error) {
        MessagePlugin.error(error.message || this.$t('common.tips.save_failed'))
      } finally {
        if (this.active) await this.fetchMasterSwitch()
      }
    },
    getList() {
      this.dataLoading = true
      oplatform_key_list_api({
        pageIndex: this.pagination.current,
        pageSize: this.pagination.pageSize,
        key_name: this.searchformData.key_name
      }).then((res) => {
        this.dataLoading = false
        if (res && res.code === 0) {
          this.data = res.data.list || []
          this.pagination.total = res.data.total
        }
      }).catch(() => { this.dataLoading = false })
    },
    handleAdd() {
      this.editingKey = undefined
      this.editorVisible = true
    },
    async handleEdit({ row }) {
      try {
        const res = await oplatform_key_detail_api({ id: row.id })
        if (res.code !== 0 || res.data?.id !== row.id) throw new Error(res.msg || this.$t('page.oplatform.edit_fail'))
        this.editingKey = res.data
        this.editorVisible = true
      } catch (error) {
        MessagePlugin.error(error.message || this.$t('page.oplatform.edit_fail'))
      }
    },
    keySaved(secret) {
      this.editorVisible = false
      if (secret) {
        this.newSecret = secret
        this.secretDialogTitle = this.$t('page.oplatform.new_key_created')
        this.secretDialogVisible = true
      } else {
        MessagePlugin.success(this.$t('page.oplatform.edit_success'))
      }
      this.getList()
    },
    handleToggleStatus(slotProps) {
      const row = slotProps.row
      const newStatus = row.status === 1 ? 0 : 1
      oplatform_key_edit_api({
        id: row.id,
        key_name: row.key_name,
        status: newStatus,
        remark: row.remark,
        rate_limit: row.rate_limit,
        ip_whitelist: row.ip_whitelist,
        expire_time: row.expire_time
      }).then((res) => {
        if (res && res.code === 0) {
          MessagePlugin.success(newStatus === 1 ? this.$t('page.oplatform.enable_success') : this.$t('page.oplatform.disable_success'))
          this.getList()
        } else {
          MessagePlugin.error(res?.msg || this.$t('page.oplatform.edit_fail'))
        }
      }).catch(() => MessagePlugin.error(this.$t('page.oplatform.edit_fail')))
    },
    handleResetSecret(slotProps) {
      oplatform_key_reset_secret_api({ id: slotProps.row.id }).then((res) => {
        if (res && res.code === 0) {
          this.newSecret = res.data.api_key
          this.secretDialogTitle = this.$t('page.oplatform.reset_key')
          this.secretDialogVisible = true
          this.getList()
        }
      })
    },
    handleDelete(slotProps) {
      this.confirmDeleteId = slotProps.row.id
      this.confirmVisible = true
    },
    onConfirmDelete() {
      this.confirmVisible = false
      oplatform_key_del_api({ id: this.confirmDeleteId }).then((res) => {
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
    },
    copyText(text) {
      navigator.clipboard.writeText(text).then(() => {
        MessagePlugin.success(this.$t('page.oplatform.copy_success'))
      }).catch(() => {
        // fallback
        const ta = document.createElement('textarea')
        ta.value = text
        document.body.appendChild(ta)
        ta.select()
        document.execCommand('copy')
        document.body.removeChild(ta)
        MessagePlugin.success(this.$t('page.oplatform.copy_success'))
      })
    }
  }
}
</script>

<style scoped>
.list-card-container {
  padding: 16px;
}
.op-master {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
.op-master__box {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  padding: 7px 14px;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: var(--td-radius-medium, 3px);
  background: var(--td-bg-color-container-hover, #fafafa);
}
.op-master__hint { color: var(--hw-muted); font-size: 13px; }
.op-master__t {
  font-weight: 500;
}
.left-operation-container {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
.right-operation-container {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
}
</style>
