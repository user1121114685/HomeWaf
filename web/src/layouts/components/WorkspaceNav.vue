<template>
  <div class="workspace-nav">
    <div v-if="!collapsed" class="nav-section-label">日常管理</div>
    <nav aria-label="常用功能">
      <RouterLink
        v-for="item in links"
        :key="item.name"
        :to="item.path"
        class="workspace-link"
        :class="{ active: route.path === item.path }"
        :title="item.label"
        :aria-current="route.path === item.path ? 'page' : undefined"
      >
        <component :is="item.icon" aria-hidden="true" /><span v-if="!collapsed">{{ item.label }}</span>
      </RouterLink>
    </nav>
    <div v-if="!collapsed" class="nav-section-label nav-tools-label">更多工具</div>
    <button
      class="workspace-link nav-all-toggle"
      :aria-expanded="allVisible"
      aria-controls="workspace-all-functions"
      :title="allVisible ? '收起全部功能' : '查看全部功能'"
      :aria-label="allVisible ? '收起全部功能' : '查看全部功能'"
      @click="allVisible = !allVisible"
    >
      <Grid aria-hidden="true" /><span v-if="!collapsed">全部功能</span
      ><ArrowDown v-if="!collapsed" class="nav-chevron" :class="{ expanded: allVisible }" aria-hidden="true" />
    </button>
    <div v-if="allVisible" id="workspace-all-functions" class="nav-all-functions">
      <template v-if="!collapsed">
        <label class="nav-search"
          ><Search aria-hidden="true" /><input v-model="query" placeholder="搜索功能" aria-label="搜索全部功能"
        /></label>
        <template v-for="group in groups" :key="group.title">
          <details :open="!!query || group.items.some((item) => item.path === route.path)">
            <summary>{{ group.title }}</summary>
            <RouterLink
              v-for="item in group.items"
              :key="item.path"
              :to="item.path"
              :aria-current="route.path === item.path ? 'page' : undefined"
              :target="item.newTab ? '_blank' : undefined"
              :rel="item.newTab ? 'noopener noreferrer' : undefined"
              >{{ item.title }}</RouterLink
            >
          </details>
        </template>
        <p v-if="!groups.length" class="nav-no-results">没有找到相关功能，试试“证书”或“日志”。</p>
      </template>
      <button v-else class="nav-expand" @click="store.commit('setting/showSidebarCompact', false)">展开</button>
    </div>
    <RouterLink to="/help" class="workspace-link nav-help" title="使用指南"
      ><Reading aria-hidden="true" /><span v-if="!collapsed">使用指南</span></RouterLink
    >
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import type { PropType } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import {
  ArrowDown,
  Connection,
  DataAnalysis,
  Document,
  Grid,
  House,
  Reading,
  Search,
  SetUp,
  Warning,
} from '@element-plus/icons-vue';
import store from '@/store';
const props = defineProps({ menu: { type: Array as PropType<any[]>, default: () => [] }, collapsed: Boolean });
const route = useRoute(),
  { t } = useI18n();
const allVisible = ref(false),
  query = ref('');
const catalog = computed(() =>
  (props.menu || []).map((group) => ({
    title: t(group.meta?.title || ''),
    items: (group.children?.length ? group.children : [group])
      .filter((item) => !item.meta?.hidden)
      .map((item) => ({
        name: item.name,
        title: t(item.meta?.title || ''),
        path: item.path.startsWith('/') || !group.children?.length ? item.path : `${group.path}/${item.path}`,
        newTab: !!item.meta?.openInNewTab,
      })),
  })),
);
const links = computed(() => {
  const items = catalog.value.flatMap((group) => group.items);
  return [
    { name: 'DashboardStart', label: '工作台', icon: House },
    { name: 'WafPipeline', label: '服务接入', icon: Connection },
    { name: 'WafvisitLog', label: '访问记录', icon: Document },
    { name: 'WafAttackLog', label: '安全事件', icon: Warning },
    { name: 'DashboardBase', label: '流量概览', icon: DataAnalysis },
    { name: 'WafPipelineTemplates', label: '策略模板', icon: SetUp },
  ].flatMap((item) => {
    const found = items.find((route) => route.name === item.name);
    return found ? [{ ...item, path: found.path }] : [];
  });
});
const groups = computed(() =>
  catalog.value
    .map((group) => ({
      ...group,
      items: group.items.filter((item) =>
        `${group.title} ${item.title}`.toLowerCase().includes(query.value.trim().toLowerCase()),
      ),
    }))
    .filter((group) => group.items.length),
);
</script>

<style scoped>
.workspace-nav {
  padding: 16px 12px;
  font-size: 14px;
}
.nav-section-label {
  padding: 8px 12px 12px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  letter-spacing: 0.5px;
}
.nav-tools-label {
  margin-top: 28px;
}
.workspace-link {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 10px 12px;
  margin-bottom: 4px;
  border-radius: 8px;
  text-decoration: none;
  color: var(--el-text-color-regular);
  font: inherit;
  line-height: 24px;
  transition:
    background 150ms,
    color 150ms;
  box-sizing: border-box;
}
.workspace-link > svg {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}
.workspace-link:hover {
  color: var(--el-text-color-primary);
  background: var(--el-fill-color-light);
}
.workspace-link.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}
.workspace-link:focus-visible,
.nav-all-functions a:focus-visible,
summary:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}
.nav-all-toggle {
  border: 0;
  background: transparent;
  cursor: pointer;
  width: 100%;
  text-align: left;
}
.workspace-link .nav-chevron {
  width: 14px;
  height: 14px;
  margin-left: auto;
  transition: transform 150ms;
}
.nav-chevron.expanded {
  transform: rotate(180deg);
}
.nav-all-functions {
  margin: 12px 0 20px;
}
.nav-search {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  padding: 0 10px;
  margin-bottom: 12px;
  min-height: 44px;
}
.nav-search svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: var(--el-text-color-regular);
}
.nav-search input {
  width: 100%;
  min-width: 0;
  background: transparent;
  color: var(--el-text-color-primary);
  border: 0;
  font: inherit;
  outline: 0;
}
.nav-search:focus-within {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 1px;
}
.nav-all-functions summary {
  cursor: pointer;
  line-height: 24px;
  padding: 10px 12px;
  color: var(--el-text-color-primary);
  font-size: 13px;
}
.nav-all-functions a {
  display: block;
  padding: 10px 12px 10px 28px;
  min-height: 44px;
  color: var(--el-text-color-regular);
  text-decoration: none;
  font-size: 13px;
  line-height: 24px;
  border-radius: 6px;
}
.nav-all-functions a:hover,
.nav-all-functions a[aria-current] {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}
.nav-no-results {
  font-size: 13px;
  line-height: 1.7;
  padding: 0 12px;
}
.nav-expand {
  background: none;
  border: 0;
  color: var(--el-color-primary);
  padding: 8px 0;
  min-height: 44px;
  cursor: pointer;
}
.nav-help {
  margin-top: 12px;
}
@media (prefers-reduced-motion: reduce) {
  .workspace-link,
  .nav-chevron {
    transition: none;
  }
}
</style>
