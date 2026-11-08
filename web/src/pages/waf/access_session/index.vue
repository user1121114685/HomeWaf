<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <hw-row justify="space-between">
        <div class="left-operation-container">
          <hw-button theme="danger" variant="outline" @click="kickAllVisible = true">
            {{ $t('page.access.session.button_kick_all') }}
          </hw-button>
        </div>
        <div class="right-operation-container">
          <hw-form ref="searchForm" :data="searchformData" :label-width="80" layout="inline" colon
            :style="{ marginBottom: '8px' }">
            <hw-form-item :label="$t('page.access.session.label_account')" name="account_name">
              <hw-input v-model="searchformData.account_name" class="search-input" clearable></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.access.session.label_ip')" name="client_ip">
              <hw-input v-model="searchformData.client_ip" class="search-input" clearable></hw-input>
            </hw-form-item>
            <hw-form-item :label="$t('page.access.session.label_status')" name="status">
              <hw-select v-model="searchformData.status" :style="{ width: '120px' }" clearable
                :placeholder="$t('common.select_placeholder')">
                <hw-option :value="1" :label="$t('page.access.session.status_valid')"></hw-option>
                <hw-option :value="0" :label="$t('page.access.session.status_revoked')"></hw-option>
              </hw-select>
            </hw-form-item>
            <hw-form-item>
              <hw-button theme="primary" :style="{ marginLeft: '8px' }" @click="getList()">
                {{ $t('common.search') }}
              </hw-button>
            </hw-form-item>
          </hw-form>
        </div>
      </hw-row>
      <help-block :summary="$t('page.access.session.alert_message')" doc="guide/AccessSession" />
      <div class="table-container">
        <hw-table :columns="columns" :data="data" :rowKey="rowKey" :verticalAlign="verticalAlign" :hover="hover"
          :pagination="pagination" :loading="dataLoading" @page-change="rehandlePageChange"
          :headerAffixedTop="true" :headerAffixProps="{ offsetTop: offsetTop, container: getContainer }">
          <template #scope="{ row }">
            <hw-tag v-if="row.scope === 'sso'" theme="primary" variant="light">{{ $t('page.access.session.scope_sso') }}</hw-tag>
            <hw-tag v-else theme="default" variant="light">{{ $t('page.access.session.scope_local') }}</hw-tag>
          </template>
          <template #status="{ row }">
            <hw-tag v-if="row.status === 1" theme="success" variant="light">{{ $t('page.access.session.status_valid') }}</hw-tag>
            <hw-tooltip v-else :content="row.revoke_reason">
              <hw-tag theme="default" variant="light">{{ $t('page.access.session.status_revoked') }}</hw-tag>
            </hw-tooltip>
          </template>
          <template #token_count="{ row }">
            <span v-if="!row.token_count">-</span>
            <hw-tooltip v-else :content="(row.token_hosts || []).join('\n')">
              <hw-tag theme="primary" variant="light">{{ hostSummary(row.token_hosts) }}</hw-tag>
            </hw-tooltip>
          </template>
          <template #location="{ row }">
            <span>{{ [row.country, row.city].filter(Boolean).join(' ') || '-' }}</span>
          </template>
          <template #op="slotProps">
            <a v-if="slotProps.row.status === 1" class="t-button-link" @click="handleKick(slotProps.row)">
              {{ $t('page.access.session.button_kick') }}
            </a>
            <span v-else>-</span>
          </template>
        </hw-table>
      </div>
    </hw-card>

    <hw-dialog :header="$t('page.access.session.button_kick')" :body="$t('page.access.session.kick_confirm')"
      v-model:visible="kickVisible" @confirm="onConfirmKick"></hw-dialog>

    <hw-dialog :header="$t('page.access.session.button_kick_all')" v-model:visible="kickAllVisible"
      :confirm-btn="{ content: $t('common.confirm'), theme: 'danger' }" :cancel-btn="$t('common.close')"
      @confirm="onConfirmKickAll">
      <template #body><div >
        <hw-alert theme="warning" :message="$t('page.access.session.kick_all_confirm')"></hw-alert>
      </div></template>
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { prefix } from '@/config/global';
import { wafAccessSessionListApi, wafAccessSessionKickApi, wafAccessSessionKickAllApi } from '@/apis/access';

export default defineComponent({
  name: 'WafAccessSession',
  data() {
    return {
      prefix,
      dataLoading: false,
      data: [],
      kickVisible: false,
      kickAllVisible: false,
      pendingId: '',
      columns: [
        { title: this.$t('page.access.session.label_account'), align: 'left', width: 140, ellipsis: true, colKey: 'account_name' },
        { title: this.$t('page.access.session.col_scope'), width: 100, colKey: 'scope' },
        { title: this.$t('page.access.session.col_bind_host'), width: 180, ellipsis: true, colKey: 'bind_host' },
        { title: this.$t('page.access.session.label_ip'), width: 140, ellipsis: true, colKey: 'client_ip' },
        { title: this.$t('page.access.session.col_location'), width: 140, ellipsis: true, colKey: 'location' },
        { title: this.$t('page.access.session.col_token_count'), width: 220, ellipsis: true, colKey: 'token_count' },
        { title: this.$t('page.access.session.col_login_time'), width: 170, ellipsis: true, colKey: 'login_time' },
        { title: this.$t('page.access.session.col_last_active'), width: 170, ellipsis: true, colKey: 'last_active_time' },
        { title: this.$t('page.access.session.col_expire'), width: 170, ellipsis: true, colKey: 'expire_time' },
        { title: this.$t('page.access.session.label_status'), width: 100, colKey: 'status' },
        { title: this.$t('page.access.session.col_ua'), width: 220, ellipsis: true, colKey: 'user_agent' },
        { align: 'left', width: 100, colKey: 'op', title: this.$t('common.op') },
      ],
      rowKey: 'id',
      verticalAlign: 'top',
      hover: true,
      pagination: { total: 0, current: 1, pageSize: 10 },
      // 这个页面叫「在线会话」，默认就该只给在线的。
      // 已失效的会话要留到清理任务删掉那天(7天)，默认全列出来的话，
      // 用得越久噪音越大，真正在线的人反而要翻页找。想看历史把筛选清空即可。
      searchformData: { account_name: '', client_ip: '', status: 1 },
    };
  },
  computed: {
    offsetTop() {
      return this.$store.state.setting.isUseTabsRouter ? 48 : 0;
    },
  },
  mounted() {
    this.getList();
  },
  methods: {
    getContainer() {
      return document.querySelector('.tdesign-starter-layout');
    },
    // 站点多的时候标签会把列撑爆，超过两个就折叠成「a.com 等 N 个」，完整列表挂在 tooltip 上
    hostSummary(hosts) {
      const list = (hosts || []).filter(Boolean);
      if (list.length === 0) return '-';
      if (list.length <= 2) return list.join('、');
      return this.$t('page.access.session.host_more')
        .replace('{first}', list[0])
        .replace('{n}', list.length);
    },
    getList() {
      const that = this;
      this.dataLoading = true;
      wafAccessSessionListApi({
        pageSize: this.pagination.pageSize,
        pageIndex: this.pagination.current,
        account_name: this.searchformData.account_name,
        client_ip: this.searchformData.client_ip,
        // status 用 null 表示"不筛选"：传 0 是有意义的取值(已注销)，不能用假值判断
        status: this.searchformData.status === undefined || this.searchformData.status === ''
          ? null : this.searchformData.status,
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
    handleKick(row) {
      this.pendingId = row.id;
      this.kickVisible = true;
    },
    onConfirmKick() {
      const that = this;
      wafAccessSessionKickApi({ id: this.pendingId }).then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
        that.kickVisible = false;
      });
    },
    onConfirmKickAll() {
      const that = this;
      wafAccessSessionKickAllApi().then((res) => {
        if (res.code === 0) {
          that.$message.success(res.msg);
          that.getList();
        } else {
          that.$message.warning(res.msg);
        }
        that.kickAllVisible = false;
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
}

.search-input {
  width: 160px;
}
</style>
