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
      这里显示服务的封禁、蜜罐、连接丢弃与端口锁定。共享封禁会影响多个服务；配置名单与当前处罚是不同对象。
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
      class="space punishment-table"
    >
      <ElTableColumn label="状态" min-width="110"
        ><template #default="{ row }"
          ><ElTag :type="row.kind === 'ban' ? 'danger' : 'warning'">{{ kinds[row.kind] || row.kind }}</ElTag></template
        ></ElTableColumn
      >
      <ElTableColumn label="来源 / 网段" min-width="155">
        <template #default="{ row }">{{ row.kind === 'synology_login_lockdown' ? '所有 DSM 用户' : row.prefix }}</template>
      </ElTableColumn>
      <ElTableColumn label="作用域" min-width="115"
        ><template #default="{ row }">{{
          row.scope === 'global' ? '多个服务共享' : '当前服务'
        }}</template></ElTableColumn
      >
      <ElTableColumn label="触发原因" min-width="180">
        <template #default="{ row }">{{ reason(row) }}</template>
      </ElTableColumn>
      <ElTableColumn label="到期时间" min-width="170"
        ><template #default="{ row }">{{
          row.expires_at ? new Date(row.expires_at * 1000).toLocaleString('zh-CN') : '永久'
        }}</template></ElTableColumn
      >
      <ElTableColumn label="操作" min-width="235"
        ><template #default="{ row }"
          ><ElButton
            link
            type="primary"
            :disabled="disabled || busy || loading || !!error"
            @click="manage(row, 'release')"
            >{{ row.kind.endsWith('lockdown') ? '解除锁定' : '释放' }}</ElButton
          ><ElButton
            v-if="row.kind === 'ban' && row.expires_at"
            link
            type="warning"
            :disabled="disabled || busy || loading || !!error"
            @click="manage(row, 'permanent')"
            >转为永久</ElButton
          ><ElButton
            v-if="row.prefix"
            link
            :disabled="disabled || busy || loading || !!error"
            @click="manage(row, 'whitelist')"
            >设为可信并释放</ElButton
          ></template
        ></ElTableColumn
      >
    </ElTable>
    <div v-if="effectivePort" class="punishment-cards">
      <p v-if="!rows.length" role="status">
        {{ loading ? '正在读取处罚记录…' : error && !loaded ? '读取失败，请重试' : '当前服务没有处罚记录' }}
      </p>
      <article v-for="row in rows" :key="row.id">
        <header>
          <ElTag :type="row.kind === 'ban' ? 'danger' : 'warning'">{{ kinds[row.kind] || row.kind }}</ElTag
          ><span>{{ row.scope === 'global' ? '多个服务共享' : '当前服务' }}</span>
        </header>
        <strong>{{ row.kind === 'synology_login_lockdown' ? '所有 DSM 用户' : row.prefix || `端口 ${effectivePort}` }}</strong>
        <dl>
          <div>
            <dt>触发原因</dt>
            <dd>{{ reason(row) }}</dd>
          </div>
          <div>
            <dt>到期时间</dt>
            <dd>{{ row.expires_at ? new Date(row.expires_at * 1000).toLocaleString('zh-CN') : '永久' }}</dd>
          </div>
        </dl>
        <div class="punishment-actions">
          <ElButton :disabled="disabled || busy || loading || !!error" @click="manage(row, 'release')">{{
            row.kind.endsWith('lockdown') ? '解除锁定' : '释放'
          }}</ElButton>
          <ElButton
            v-if="row.kind === 'ban' && row.expires_at"
            :disabled="disabled || busy || loading || !!error"
            @click="manage(row, 'permanent')"
            >转为永久</ElButton
          >
          <ElButton
            v-if="row.prefix"
            :disabled="disabled || busy || loading || !!error"
            @click="manage(row, 'whitelist')"
            >设为可信并释放</ElButton
          >
        </div>
      </article>
    </div>
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
const props = defineProps({ port: Number, disabled: Boolean });
const emit = defineEmits(['changed', 'busy-change', 'loaded']);
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
  kinds: Record<string, string> = {
    ban: '封禁',
    honeypot: '蜜罐',
    lockdown: '端口锁定',
    drop: '连接丢弃',
    honeypot_lockdown: '蜜罐锁定',
    synology_login_lockdown: '群晖登录锁定',
  };
const reason = (row: any) => row.kind === 'synology_login_lockdown' ? `DSM 认证失败累计 ${row.hits} 次` : row.reason || '未记录';
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
    if (!Array.isArray(result)) throw new Error('处罚响应不完整，请重试。');
    rows.value = result;
    loaded.value = true;
    emit('loaded', rows.value.length);
  } catch (cause: any) {
    if (token === sequence) {
      emit('loaded', null);
      error.value =
        (cause.message || '处罚记录读取失败') + (loaded.value ? '。当前保留上次结果，请刷新后再操作。' : '。请重试。');
    }
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
  if (props.disabled || busy.value || loading.value || error.value) return;
  const port = effectivePort.value;
  busy.value = true;
  emit('busy-change', true);
  try {
    const label = {
      release: row.kind === 'synology_login_lockdown' ? '恢复本服务所有用户的 DSM 登录' : row.kind.endsWith('lockdown') ? '解除端口锁定' : '释放处罚',
      permanent: '改为永久封禁',
      whitelist: '加入当前服务 trusted 名单并释放',
    }[action];
    await ElMessageBox.confirm(
      `${label}：${row.prefix || '端口 ' + port}（当前服务 :${port}）。${row.scope === 'global' ? '这是共享记录，修改或释放会同时影响其他服务。' : ''}${action === 'whitelist' ? '可信名单仅写入当前服务；IP/地域黑名单与访问总开关仍然有效。' : ''}`,
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
    emit('busy-change', false);
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
defineExpose({ refresh });
</script>
<style scoped>
.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.editor-context {
  color: var(--hw-muted);
  font-size: 13px;
  line-height: 1.7;
  margin: 16px 0;
}
.punishment-cards {
  display: none;
}
@media (max-width: 1000px) {
  .punishment-table {
    display: none;
  }
  .punishment-cards {
    display: grid;
    gap: 16px;
    margin: 20px 0;
  }
  .punishment-cards article {
    padding: 20px 0;
    border-top: 1px solid var(--hw-line);
  }
  .punishment-cards header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    font-size: 13px;
    color: var(--hw-muted);
  }
  .punishment-cards strong {
    display: block;
    margin: 16px 0;
    overflow-wrap: anywhere;
  }
  .punishment-cards dl {
    display: grid;
    gap: 12px;
    margin: 0;
    font-size: 13px;
  }
  .punishment-cards dt {
    color: var(--hw-muted);
  }
  .punishment-cards dd {
    margin: 4px 0 0;
    overflow-wrap: anywhere;
  }
  .punishment-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 16px;
  }
  .punishment-actions .el-button {
    min-height: 44px;
    margin: 0;
  }
}
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
