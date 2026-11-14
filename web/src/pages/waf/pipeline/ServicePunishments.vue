<template>
  <section data-testid="service-punishments" :aria-busy="loading || busy">
    <div class="editor-toolbar">
      <label v-if="!port" class="punishment-service"
        >服务范围<ElSelect
          v-model="selectedPort"
          :disabled="busy || loadingServices"
          aria-label="处罚记录的服务"
          placeholder="选择服务"
          @change="updateLocation"
          ><ElOption
            v-for="item in services"
            :key="item.port"
            :value="item.port"
            :label="`${item.name || '未命名服务'} · :${item.port}`" /></ElSelect
      ></label>
      <span v-else
        >当前服务端口 <code>{{ port }}</code></span
      >
      <ElButton :loading="loading || loadingServices" :disabled="busy" @click="refresh">刷新处罚记录</ElButton>
    </div>
    <p class="editor-context">
      这里显示原生服务的自动封禁、蜜罐与端口锁定。共享封禁会影响多个服务；配置名单与当前处罚是不同对象。
    </p>
    <ElAlert v-if="error" :title="error" type="error" :closable="false" class="space" />
    <p v-if="loadingServices" role="status">正在读取服务范围…</p>
    <p v-else-if="!port && servicesLoaded && !services.length" class="editor-context">
      尚未接入服务。<RouterLink :to="{ name: 'WafPipeline' }">前往我的服务</RouterLink>
    </p>
    <p v-else-if="!effectivePort" class="editor-context">选择服务后查看处罚记录。</p>
    <ElTable
      v-else
      :data="rows"
      row-key="id"
      :empty-text="loading ? '正在读取处罚记录…' : error && !loaded ? '读取失败，请重试' : '当前服务没有处罚记录'"
      class="space"
    >
      <ElTableColumn label="状态" min-width="110"
        ><template #default="{ row }"
          ><ElTag :type="row.kind === 'ban' ? 'danger' : 'warning'">{{ kinds[row.kind] || row.kind }}</ElTag></template
        ></ElTableColumn
      >
      <ElTableColumn prop="prefix" label="来源 / 网段" min-width="155" />
      <ElTableColumn label="作用域" min-width="115"
        ><template #default="{ row }">{{
          row.scope === 'global' ? '多个服务共享' : '当前服务'
        }}</template></ElTableColumn
      >
      <ElTableColumn prop="reason" label="触发原因" min-width="180" />
      <ElTableColumn label="到期时间" min-width="170"
        ><template #default="{ row }">{{
          row.expires_at ? new Date(row.expires_at * 1000).toLocaleString('zh-CN') : '永久'
        }}</template></ElTableColumn
      >
      <ElTableColumn label="操作" min-width="235"
        ><template #default="{ row }"
          ><ElButton link type="primary" :disabled="busy || loading || !!error" @click="manage(row, 'release')">{{
            row.kind === 'lockdown' ? '解除锁定' : '释放'
          }}</ElButton
          ><ElButton
            v-if="row.kind === 'ban' && row.expires_at"
            link
            type="warning"
            :disabled="busy || loading || !!error"
            @click="manage(row, 'permanent')"
            >转为永久</ElButton
          ><ElButton v-if="row.prefix" link :disabled="busy || loading || !!error" @click="manage(row, 'whitelist')"
            >设为可信并释放</ElButton
          ></template
        ></ElTableColumn
      >
    </ElTable>
    <p class="editor-context">
      “设为可信”只修改当前服务的 trusted 名单，仍受本服务的 IP/地域黑名单、手动 deny 和访问总开关约束。
    </p>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  ElAlert,
  ElButton,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
const props = defineProps({ port: Number });
const emit = defineEmits(['changed']);
const route = useRoute(),
  router = useRouter();
const services = ref<{ port: number; name: string }[]>([]),
  selectedPort = ref<number>(),
  rows = ref<any[]>([]);
const error = ref(''),
  loading = ref(false),
  loaded = ref(false),
  loadingServices = ref(false),
  servicesLoaded = ref(false),
  busy = ref(false);
const effectivePort = computed(() => props.port || selectedPort.value),
  kinds: Record<string, string> = { ban: '封禁', honeypot: '蜜罐', lockdown: '端口锁定' };
let sequence = 0;
async function readServices() {
  loadingServices.value = true;
  try {
    const result = await pipelineGet<any>('overview');
    services.value = (result.ports || []).map((item) => item.config);
    servicesLoaded.value = true;
    const requested = Number(route.query.port);
    if (services.value.some((item) => item.port === requested)) selectedPort.value = requested;
    else if (!services.value.some((item) => item.port === selectedPort.value))
      selectedPort.value = services.value[0]?.port;
  } catch (cause: any) {
    error.value = cause.message || '服务范围读取失败，请重试。';
  } finally {
    loadingServices.value = false;
  }
}
async function read() {
  const port = effectivePort.value,
    token = ++sequence;
  if (!port) {
    rows.value = [];
    loaded.value = false;
    return;
  }
  loading.value = true;
  error.value = '';
  try {
    const result = await pipelineGet<any[]>('state', { port });
    if (token !== sequence) return;
    rows.value = result || [];
    loaded.value = true;
  } catch (cause: any) {
    if (token === sequence)
      error.value =
        (cause.message || '处罚记录读取失败') + (loaded.value ? '。当前保留上次结果，请刷新后再操作。' : '。请重试。');
  } finally {
    if (token === sequence) loading.value = false;
  }
}
async function refresh() {
  if (busy.value || loading.value || loadingServices.value) return;
  error.value = '';
  if (!props.port) await readServices();
  if (!error.value) await read();
}
function updateLocation() {
  router.replace({ query: { ...route.query, port: selectedPort.value } });
}
async function manage(row: any, action: string) {
  if (busy.value || loading.value || error.value) return;
  const port = effectivePort.value;
  busy.value = true;
  try {
    const label = {
      release: row.kind === 'lockdown' ? '解除端口锁定' : '释放处罚',
      permanent: '改为永久封禁',
      whitelist: '加入当前服务 trusted 名单并释放',
    }[action];
    await ElMessageBox.confirm(
      `${label}：${row.prefix || '端口 ' + port}。${row.scope === 'global' ? '这是共享记录，释放会同时影响其他服务。' : ''}`,
      '确认处罚操作',
      { type: 'warning', confirmButtonText: '确认操作', cancelButtonText: '取消' },
    );
    await pipelinePost('state/manage', { port, id: row.id, action });
    ElMessage.success('处罚状态已更新');
    emit('changed');
    await read();
  } catch (cause: any) {
    if (cause !== 'cancel' && cause !== 'close') error.value = cause.message || '操作失败，请刷新后重试。';
  } finally {
    busy.value = false;
  }
}
watch(effectivePort, () => {
  rows.value = [];
  loaded.value = false;
  void read();
});
watch(
  () => route.query.port,
  () => {
    if (!props.port) {
      const requested = Number(route.query.port);
      if (services.value.some((item) => item.port === requested)) selectedPort.value = requested;
    }
  },
);
onMounted(async () => {
  if (props.port) await read();
  else await readServices();
});
onBeforeUnmount(() => {
  sequence++;
});
</script>
<style scoped>
.punishment-service {
  display: grid;
  gap: 8px;
  width: min(100%, 340px);
  font-size: 13px;
  font-weight: 500;
}
.space {
  margin: 16px 0;
}
</style>
