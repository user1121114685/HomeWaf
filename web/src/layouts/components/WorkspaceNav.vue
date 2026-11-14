<template>
  <nav class="workspace-nav" aria-label="管理功能">
    <label v-if="!collapsed" class="nav-search"
      ><Search aria-hidden="true" /><input v-model="query" placeholder="查找功能" aria-label="查找功能" /><kbd
        v-if="!query"
        >/</kbd
      ><button v-else aria-label="清除搜索" @click="query = ''"><Close /></button
    ></label>
    <div class="nav-pinned">
      <RouterLink
        v-for="item in pinned"
        :key="item.name"
        :to="item.path"
        class="workspace-link"
        :title="item.title"
        :aria-current="route.path === item.path ? 'page' : undefined"
      >
        <component :is="icons[item.name] || Document" aria-hidden="true" /><span v-if="!collapsed">{{
          item.title
        }}</span>
      </RouterLink>
    </div>
    <div v-if="!collapsed" class="nav-directory-label">功能目录</div>
    <template v-for="group in groups" :key="group.title">
      <details
        v-if="!collapsed"
        class="nav-group"
        :open="!!query || group.items.some((item) => route.path === item.path)"
      >
        <summary>
          <component :is="group.icon" aria-hidden="true" /><span>{{ group.title }}</span
          ><ArrowDown class="nav-chevron" />
        </summary>
        <RouterLink
          v-for="item in group.items"
          :key="item.path"
          :to="item.path"
          :aria-current="route.path === item.path ? 'page' : undefined"
          :target="item.newTab ? '_blank' : undefined"
          :rel="item.newTab ? 'noopener noreferrer' : undefined"
          >{{ item.title }}<TopRight v-if="item.newTab" aria-hidden="true"
        /></RouterLink>
      </details>
      <button
        v-else
        class="workspace-link nav-expand-group"
        :title="group.title"
        :aria-label="'展开' + group.title"
        @click="store.commit('setting/showSidebarCompact', false)"
      >
        <component :is="group.icon" aria-hidden="true" />
      </button>
    </template>
    <p v-if="!collapsed && !groups.length && !pinned.length" class="nav-no-results">没有找到相关功能。</p>
    <RouterLink to="/help" class="workspace-link nav-help" title="使用指南"
      ><Reading aria-hidden="true" /><span v-if="!collapsed">使用指南</span></RouterLink
    >
  </nav>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import {
  ArrowDown,
  Close,
  Connection,
  Document,
  Grid,
  House,
  Reading,
  Search,
  TopRight,
  Warning,
} from '@element-plus/icons-vue';
import { navigationItems, filterNavigation } from '@/ui/navigation';
import store from '@/store';
const props = defineProps({ menu: { type: Array, default: () => [] }, collapsed: Boolean });
const route = useRoute(),
  { t } = useI18n(),
  query = ref('');
const icons = { DashboardStart: House, WafPipeline: Connection, WafvisitLog: Document, WafAttackLog: Warning };
const pinnedNames = Object.keys(icons);
const items = computed(() => filterNavigation(navigationItems(props.menu, t), query.value));
const pinned = computed(() => pinnedNames.flatMap((name) => items.value.filter((item) => item.name === name)));
const groups = computed(() => {
  const grouped = new Map<string, ReturnType<typeof navigationItems>>();
  for (const item of items.value.filter((item) => !pinnedNames.includes(item.name))) {
    const group = item.group || '其他功能';
    grouped.set(group, [...(grouped.get(group) || []), item]);
  }
  return [...grouped].map(([title, items]) => ({
    title,
    items,
    icon: (props.menu as any[]).find((group) => t(group.meta?.title || '') === title)?.meta?.icon || Grid,
  }));
});
function focusSearch(event: KeyboardEvent) {
  if (
    event.key !== '/' ||
    event.ctrlKey ||
    event.metaKey ||
    event.altKey ||
    (event.target as HTMLElement)?.closest('input, textarea, select, [contenteditable="true"]')
  )
    return;
  event.preventDefault();
  store.commit('setting/showSidebarCompact', false);
  requestAnimationFrame(() => document.querySelector<HTMLInputElement>('.nav-search input')?.focus());
}
onMounted(() => window.addEventListener('keydown', focusSearch));
onUnmounted(() => window.removeEventListener('keydown', focusSearch));
</script>
<style scoped>
.workspace-nav {
  padding: 16px 12px;
  font-size: 14px;
}
.nav-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 10px;
  margin-bottom: 20px;
  border: 1px solid var(--hw-line);
  background: var(--hw-subtle);
  border-radius: var(--hw-r-md);
  color: var(--hw-muted);
}
.nav-search svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}
.nav-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--hw-ink);
  font: inherit;
}
.nav-search input:focus-visible {
  outline: 2px solid var(--hw-accent);
  outline-offset: 3px;
  border-radius: 2px;
}
.nav-search kbd {
  border: 1px solid var(--hw-line);
  border-radius: 3px;
  font: 13px var(--font-mono);
  padding: 0 4px;
}
.nav-search button {
  border: 0;
  padding: 0;
  background: transparent;
  display: flex;
  color: inherit;
}
.workspace-link,
.nav-group summary {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 10px 12px;
  color: var(--hw-muted);
  font: inherit;
  line-height: 24px;
  border-radius: var(--hw-r-md);
  transition:
    background 150ms,
    color 150ms;
}
.workspace-link > svg,
.nav-group summary > svg {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
.workspace-link:hover,
.nav-group summary:hover {
  color: var(--hw-ink);
  background: var(--hw-subtle);
}
.workspace-link[aria-current] {
  background: var(--hw-accent-soft);
  color: var(--hw-accent);
  font-weight: 500;
}
.nav-directory-label {
  padding: 24px 12px 8px;
  font-size: 13px;
  color: var(--hw-muted);
}
.nav-group summary {
  cursor: pointer;
  list-style: none;
}
.nav-group summary::-webkit-details-marker {
  display: none;
}
.nav-group summary .nav-chevron {
  margin-left: auto;
  width: 12px;
  transition: transform 150ms;
}
.nav-group[open] summary .nav-chevron {
  transform: rotate(180deg);
}
.nav-group > a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 40px;
  padding: 8px 12px 8px 42px;
  font-size: 13px;
  color: var(--hw-muted);
  border-radius: 4px;
}
.nav-group > a svg {
  width: 12px;
  height: 12px;
}
.nav-group > a:hover,
.nav-group > a[aria-current] {
  color: var(--hw-accent);
  background: var(--hw-subtle);
}
.nav-group > a[aria-current] {
  box-shadow: inset 2px 0 var(--hw-accent);
  font-weight: 500;
}
.nav-help {
  margin-top: 24px;
  border-top: 1px solid var(--hw-line);
  border-radius: 0;
  padding-top: 16px;
}
.nav-no-results {
  padding: 16px 12px;
  color: var(--hw-muted);
}
.nav-expand-group {
  width: 100%;
  border: 0;
  background: transparent;
  cursor: pointer;
}
@media (max-width: 760px) {
  .nav-group > a {
    min-height: 44px;
    font-size: 14px;
  }
  .nav-search {
    height: 44px;
  }
}
</style>
