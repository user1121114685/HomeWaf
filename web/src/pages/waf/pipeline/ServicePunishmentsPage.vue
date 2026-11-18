<template>
  <div class="service-punishments-page" data-testid="service-punishments-page">
    <PageActions>
      <ElButton :disabled="busy" @click="goBack">
        <ArrowLeft class="action-icon" aria-hidden="true" />返回我的服务
      </ElButton>
    </PageActions>
    <ElAlert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <ElButton v-if="error && validPort" :loading="loading" @click="loadService">重试读取服务</ElButton>
    <div v-if="!error && loading" class="punishments-empty" role="status">正在读取服务状态…</div>
    <div v-else-if="!error && !loading && !service" class="punishments-empty" role="alert">
      <h1>未找到此服务</h1>
      <p>端口 {{ port }} 的服务可能已删除，请返回我的服务核对。</p>
    </div>
    <template v-else-if="!error && service">
      <header class="punishments-heading">
        <div>
          <h1>{{ service.config.name || `服务 ${port}` }} · 当前处罚</h1>
          <p>{{ service.config.protocol.toUpperCase() }} · :{{ port }}</p>
        </div>
      </header>
      <ServicePunishments :port="port" :disabled="busy" @busy-change="busy = $event" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router';
import { ElAlert, ElButton, ElMessage } from 'element-plus';
import { ArrowLeft } from '@element-plus/icons-vue';
import { pipelineGet } from '@/apis/pipeline';
import PageActions from '@/ui/PageActions.vue';
import ServicePunishments from './ServicePunishments.vue';
import { serviceReturnPath } from './diagnostics';

const route = useRoute();
const router = useRouter();
const port = computed(() => Number(route.params.port));
const validPort = computed(() => Number.isInteger(port.value) && port.value > 0 && port.value <= 65535);
const service = ref<any>(null);
const error = ref('');
const loading = ref(false);
const busy = ref(false);
let generation = 0;

async function loadService() {
  const currentGeneration = ++generation;
  service.value = null;
  error.value = '';
  if (!validPort.value) {
    error.value = '服务端口无效，请返回我的服务重新选择。';
    return;
  }

  loading.value = true;
  try {
    const overview = await pipelineGet<any>('overview');
    if (currentGeneration !== generation) return;
    if (!Array.isArray(overview?.ports)) throw new Error('服务状态响应不完整，请重试。');
    service.value = overview.ports.find((row: any) => row.config?.port === port.value) || null;
  } catch (cause: any) {
    if (currentGeneration === generation) error.value = cause.message || '服务状态读取失败，请重试。';
  } finally {
    if (currentGeneration === generation) loading.value = false;
  }
}

function goBack() {
  if (!busy.value) void router.push(serviceReturnPath(route.query.returnTo, true));
}

function allowNavigation() {
  if (!busy.value) return true;
  ElMessage.warning('处罚操作尚未完成，请先完成或取消确认。');
  return false;
}

watch(port, () => void loadService(), { immediate: true });
onBeforeRouteLeave(allowNavigation);
onBeforeRouteUpdate((to, from) => to.params.port === from.params.port || allowNavigation());
onBeforeUnmount(() => {
  generation++;
});
</script>

<style scoped>
.service-punishments-page {
  display: grid;
  gap: 16px;
  min-width: 0;
}

.action-icon {
  width: 16px;
  height: 16px;
  margin-right: 6px;
}

.punishments-heading {
  padding-bottom: 16px;
  border-bottom: 1px solid var(--hw-line);
}

.punishments-heading h1,
.punishments-empty h1 {
  margin: 0;
  font-size: 22px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.punishments-heading p,
.punishments-empty p {
  margin: 6px 0 0;
  color: var(--hw-muted);
  line-height: 1.6;
}

.punishments-empty {
  padding: 24px 0;
}
</style>
