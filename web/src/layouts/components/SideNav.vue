<template>
  <div class="console-side-inner">
    <RouterLink to="/dashboard/start" class="console-brand" aria-label="HomeWaf 工作台">
      <Logo class="console-brand-mark" aria-hidden="true" />
      <span v-if="!collapsed" class="console-brand-name"><strong>HomeWaf</strong><small>服务防护控制台</small></span>
    </RouterLink>
    <div class="console-side-scroll"><WorkspaceNav :menu="menu" :collapsed="collapsed" /></div>
    <button class="console-version" aria-label="查看系统信息" title="查看系统信息" @click="systemInfoVisible = true">
      <Monitor aria-hidden="true" /><span v-if="!collapsed">{{ version.version || '系统信息' }}</span>
    </button>
    <SystemInfoDialog v-model:visible="systemInfoVisible" />
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Monitor } from '@element-plus/icons-vue';
import Logo from '@/assets/assets-t-logo.svg';
import store from '@/store';
import { SysVersionApi } from '@/apis/sysinfo';
import WorkspaceNav from './WorkspaceNav.vue';
import SystemInfoDialog from './SystemInfoDialog.vue';
defineProps({ menu: { type: Array, default: () => [] }, collapsed: Boolean });
const systemInfoVisible = ref(false);
const version = computed(() => store.getters['version/getversion']);
onMounted(async () => {
  try {
    const result = await SysVersionApi();
    if (result.code === 0) store.commit('version/setVersionData', result.data);
  } catch {
    /* Request client records connectivity failures. */
  }
});
</script>
