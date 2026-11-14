<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->
<template>
  <div class="runtime-page">
    <PageActions><ElButton :loading="loading" @click="load">刷新参数</ElButton></PageActions>
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <section class="runtime-panel">
      <div class="runtime-filter">
        <label for="runtime-search">查找运行参数</label
        ><ElInput id="runtime-search" v-model="query" clearable placeholder="输入参数名称或值" /><span
          >{{ filtered.length }} 项</span
        >
      </div>
      <p class="runtime-note">这里显示实例当前的运行参数；修改配置后，可在此核对实际生效值。</p>
      <p v-if="loading && !loaded" role="status" class="runtime-empty">正在读取运行参数…</p>
      <p v-else-if="!filtered.length" class="runtime-empty">
        {{
          error ? '读取失败，请刷新重试。' : query ? '没有匹配的参数，请调整搜索条件。' : '当前没有可显示的运行参数。'
        }}
      </p>
      <dl v-else class="runtime-values">
        <div v-for="(item, index) in filtered" :key="item.name + '-' + index">
          <dt>{{ item.name }}</dt>
          <dd>
            <pre>{{ format(item.value) }}</pre>
          </dd>
        </div>
      </dl>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { ElAlert, ElButton, ElInput } from 'element-plus';
import { wafStatRuntimeSysinfoapi } from '@/apis/stats';
const runtimes = ref<any[]>([]),
  query = ref(''),
  loading = ref(false),
  loaded = ref(false),
  error = ref('');
const format = (value: unknown) => (typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value ?? '—'));
const filtered = computed(() => {
  const term = query.value.trim().toLocaleLowerCase();
  return runtimes.value.filter(
    (item) => !term || (item.name + ' ' + format(item.value)).toLocaleLowerCase().includes(term),
  );
});
async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  try {
    const result = await wafStatRuntimeSysinfoapi('');
    if (result.code !== 0) throw new Error(result.msg || '运行参数读取失败');
    if (!Array.isArray(result.data)) throw new Error('运行参数返回格式不正确');
    runtimes.value = result.data;
    loaded.value = true;
  } catch (cause: any) {
    error.value = (cause.message || '运行参数读取失败') + (loaded.value ? '。当前仍显示上次读取的结果。' : '');
  } finally {
    loading.value = false;
  }
}
onMounted(load);
</script>
<style scoped>
.runtime-page {
  display: grid;
  gap: 16px;
}
.runtime-panel {
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: 8px;
  padding: 24px;
}
.runtime-filter {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.runtime-filter label,
.runtime-filter > span {
  font-size: 13px;
  color: var(--hw-muted);
}
.runtime-filter .el-input {
  width: min(100%, 400px);
}
.runtime-note {
  color: var(--hw-muted);
  font-size: 13px;
  margin: 16px 0 24px;
}
.runtime-values {
  margin: 0;
}
.runtime-values > div {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(0, 3fr);
  gap: 24px;
  padding: 18px 0;
  border-top: 1px solid var(--hw-line);
}
.runtime-values dt {
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.runtime-values dd {
  margin: 0;
  min-width: 0;
}
.runtime-values pre {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.runtime-empty {
  padding: 36px 16px;
  color: var(--hw-muted);
  text-align: center;
}
@media (max-width: 600px) {
  .runtime-panel {
    padding: 16px;
  }
  .runtime-values > div {
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
  }
}
</style>
