<!-- HomeWaf modification (2026-11-15): complete TCP/UDP connection inspection. -->
<template>
  <section data-testid="tunnel-connections" :aria-busy="loading">
    <div class="editor-toolbar">
      <div>
        <strong>{{ tunnel.name || '隧道连接' }}</strong>
        <p class="editor-context">按监听端口核对连接数量，再查看来源；刷新不会中断连接。</p>
      </div>
      <hw-button :loading="loading" @click="load">刷新连接</hw-button>
    </div>
    <hw-alert v-if="error" theme="error" :close-btn="false">{{ error }}</hw-alert>
    <EditorSection title="监听端口" description="分别列出 TCP / UDP 的来源连接与目标连接数量。">
      <hw-table
        :data="ports"
        :columns="portColumns"
        row-key="port"
        :loading="loading"
        :empty="error && !loaded ? '读取失败，请刷新重试' : '当前没有监听端口记录'"
      />
    </EditorSection>
    <EditorSection title="连接来源" description="选择监听端口，按协议查看来源 IP 和地区。">
      <div class="connection-filters">
        <label
          >监听端口<hw-select v-model="selectedPort" aria-label="连接来源端口"
            ><hw-option
              v-for="port in ports"
              :key="port.port"
              :value="port.port"
              :label="String(port.port)" /></hw-select
        ></label>
        <label
          >协议<hw-select v-model="protocol" aria-label="连接来源协议" placeholder="全部协议"
            ><hw-option value="" label="全部协议" /><hw-option value="TCP" label="TCP" /><hw-option
              value="UDP"
              label="UDP" /></hw-select
        ></label>
        <label>查找来源<hw-input v-model="query" aria-label="查找连接来源" placeholder="IP 或地区" clearable /></label>
      </div>
      <hw-table
        :data="pageRows"
        :columns="sourceColumns"
        row-key="key"
        :empty="query || protocol ? '没有匹配的来源，请调整筛选' : '这个端口当前没有来源连接'"
      />
      <ElPagination
        v-if="filtered.length > 10"
        v-model:current-page="page"
        :page-size="10"
        :total="filtered.length"
        layout="prev, pager, next"
        :pager-count="5"
      />
    </EditorSection>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { ElPagination } from 'element-plus';
import { wafTunnelConnectionApi } from '@/apis/tunnel';
const props = defineProps({ tunnelCode: { type: String, default: '' } });
const ports = ref<any[]>([]),
  tunnel = ref<any>({}),
  selectedPort = ref<number>(),
  protocol = ref(''),
  query = ref(''),
  page = ref(1),
  loading = ref(false),
  loaded = ref(false),
  error = ref('');
const portColumns = computed(() => {
  const protocols = tunnel.value.protocol === 'udp' ? ['udp', 'tcp'] : ['tcp', 'udp'];
  return [
    { colKey: 'port', title: '监听端口', width: 110 },
    ...protocols.flatMap((protocol) => [
      { colKey: protocol + '_source_count', title: protocol.toUpperCase() + ' 来源', minWidth: 110 },
      { colKey: protocol + '_target_count', title: protocol.toUpperCase() + ' 目标', minWidth: 110 },
    ]),
  ];
});
const sourceColumns = [
  { colKey: 'protocol', title: '协议', width: 85 },
  { colKey: 'ip', title: '来源 IP', minWidth: 170 },
  { colKey: 'region', title: '地区', minWidth: 170 },
];
const sources = computed(() => {
  const port = ports.value.find((port) => port.port === selectedPort.value);
  if (!port) return [];
  return ['TCP', 'UDP'].flatMap((type) =>
    (port[type.toLowerCase() + '_source_ips'] || []).map((source: any, index: number) => ({
      ...source,
      protocol: type,
      key: type + ':' + index + ':' + source.ip,
    })),
  );
});
const filtered = computed(() =>
  sources.value.filter(
    (source) =>
      (!protocol.value || protocol.value === source.protocol) &&
      (!query.value || [source.ip, source.region].join(' ').toLowerCase().includes(query.value.trim().toLowerCase())),
  ),
);
const pageRows = computed(() => filtered.value.slice((page.value - 1) * 10, page.value * 10));
let sequence = 0;
async function load() {
  const token = ++sequence;
  if (!props.tunnelCode) return;
  loading.value = true;
  error.value = '';
  try {
    const result = await wafTunnelConnectionApi({ id: props.tunnelCode });
    if (token !== sequence) return;
    if (result.code !== 0) throw new Error(result.msg || '连接读取失败');
    ports.value = result.data.port_info || [];
    tunnel.value = result.data.tunnel_info || {};
    loaded.value = true;
    if (!ports.value.some((port) => port.port === selectedPort.value)) selectedPort.value = ports.value[0]?.port;
    if (page.value > Math.max(1, Math.ceil(filtered.value.length / 10))) page.value = 1;
  } catch (cause: any) {
    if (token === sequence)
      error.value = (cause.message || '连接读取失败') + (loaded.value ? '。当前保留上次结果。' : '。请重试。');
  } finally {
    if (token === sequence) loading.value = false;
  }
}
watch(
  () => props.tunnelCode,
  () => {
    ports.value = [];
    tunnel.value = {};
    loaded.value = false;
    selectedPort.value = undefined;
    void load();
  },
  { immediate: true },
);
watch([selectedPort, protocol, query], () => {
  page.value = 1;
});
onBeforeUnmount(() => {
  sequence++;
});
defineExpose({ refresh: load });
</script>
<style scoped>
.connection-filters {
  display: grid;
  grid-template-columns: 160px 150px minmax(160px, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.connection-filters label {
  display: grid;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
}
.el-pagination {
  margin-top: 16px;
  justify-content: flex-end;
}
@media (max-width: 700px) {
  .connection-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .connection-filters label:last-child {
    grid-column: 1/-1;
  }
}
</style>
