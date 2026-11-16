<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div>
    <div class="editor-toolbar">
      <span class="editor-context">
        <span v-if="result && result.cached_at">{{ $t('page.application.net_cached_at') }}{{ result.cached_at }}</span>
        <span v-if="result && result.pids && result.pids.length > 1" style="margin-left: 12px">
          PIDs: {{ result.pids.join(', ') }}
        </span>
      </span>
      <hw-button size="small" :loading="loading" @click="load">{{ $t('common.refresh') }}</hw-button>
    </div>

    <hw-alert v-if="error" theme="error" :close-btn="false">{{ error }}</hw-alert>
    <EditorSection
      :title="$t('page.application.net_ports')"
      description="先确认应用实际监听的地址与进程，再查看远端连接来源。"
    >
      <hw-table
        :data="ports"
        :columns="portColumns"
        row-key="local_addr"
        size="small"
        :empty="$t('page.application.net_no_ports')"
      >
      </hw-table>
    </EditorSection>
    <EditorSection
      :title="$t('page.application.net_connections')"
      :description="`${connSummaryAll.length} 个远端 IP，按来源汇总连接数量及本地端口。`"
    >
      <hw-alert
        v-if="result && result.connections && result.connections.length >= 1000"
        theme="warning"
        :message="$t('page.application.net_conn_limit')"
        style="margin-bottom: 8px"
      />
      <hw-table
        :data="connPageData"
        :columns="connColumns"
        row-key="remote_ip"
        size="small"
        :pagination="connPagination"
        @page-change="onConnPageChange"
        :empty="$t('page.application.net_no_connections')"
      >
      </hw-table>
    </EditorSection>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

import { wafAppNetStatsApi } from '@/apis/application';

function parsePort(addr: string): number {
  if (!addr) return 0;
  const lastColon = addr.lastIndexOf(':');
  if (lastColon < 0) return 0;
  return parseInt(addr.slice(lastColon + 1), 10) || 0;
}

export default defineComponent({
  name: 'AppNetwork',
  props: { appCode: { type: String, default: '' } },
  data() {
    return {
      loading: false,
      error: '',
      result: null as any,
      connCurrentPage: 1,
      connPageSize: 20,
      portColumns: [
        { colKey: 'protocol', title: this.$t('page.application.net_protocol'), width: 80 },
        { colKey: 'local_addr', title: this.$t('page.application.net_local_addr'), ellipsis: true },
        { colKey: 'port', title: this.$t('page.application.net_port'), width: 80 },
        { colKey: 'pid', title: 'PID', width: 80 },
      ],
      connColumns: [
        { colKey: 'remote_ip', title: this.$t('page.application.net_remote_ip'), ellipsis: true },
        { colKey: 'count', title: this.$t('page.application.net_conn_count'), width: 100 },
        { colKey: 'local_ports', title: this.$t('page.application.net_local_ports'), ellipsis: true },
      ],
    };
  },
  computed: {
    ports(): any[] {
      return this.result && this.result.ports ? this.result.ports : [];
    },
    connSummaryAll(): any[] {
      const rawConns: any[] = this.result && this.result.connections ? this.result.connections : [];
      const map: Record<string, { remote_ip: string; count: number; portSet: Set<number> }> = {};
      for (const c of rawConns) {
        const ip = c.remote_ip || c.remote_addr || '';
        if (!ip) continue;
        if (!map[ip]) {
          map[ip] = { remote_ip: ip, count: 0, portSet: new Set() };
        }
        map[ip].count++;
        const port = parsePort(c.local_addr);
        if (port) map[ip].portSet.add(port);
      }
      return Object.values(map)
        .map((v) => ({
          remote_ip: v.remote_ip,
          count: v.count,
          local_ports: Array.from(v.portSet).join(', '),
        }))
        .sort((a, b) => b.count - a.count);
    },
    connPageData(): any[] {
      const start = (this.connCurrentPage - 1) * this.connPageSize;
      return this.connSummaryAll.slice(start, start + this.connPageSize);
    },
    connPagination(): object {
      return {
        current: this.connCurrentPage,
        pageSize: this.connPageSize,
        total: this.connSummaryAll.length,
      };
    },
  },
  mounted() {
    this.load();
  },
  methods: {
    load() {
      if (!this.appCode || this.loading) return;
      this.loading = true;
      this.error = '';
      this.connCurrentPage = 1;
      wafAppNetStatsApi({ code: this.appCode })
        .then((res) => {
          if (res && res.code === 0) {
            this.result = res.data;
          } else throw new Error(res?.msg || '网络连接读取失败');
        })
        .catch((error) => {
          this.error = (error.message || '网络连接读取失败') + (this.result ? '，当前保留上次结果。' : '，请重试。');
        })
        .finally(() => {
          this.loading = false;
        });
    },
    onConnPageChange(curr: any) {
      this.connCurrentPage = curr.current;
      this.connPageSize = curr.pageSize;
    },
  },
});
</script>

<style scoped></style>
