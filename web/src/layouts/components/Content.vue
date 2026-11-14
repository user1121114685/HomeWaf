<template>
  <PageHeading :title="title" :eyebrow="group" :description="description" />
  <nav v-if="siblings.length > 1" class="task-navigation" :aria-label="group">
    <RouterLink v-for="item in siblings" :key="item.name" :to="siblingTarget(item)"
      :aria-current="route.name === item.name ? 'page' : undefined">{{ item.title }}</RouterLink>
  </nav>
  <div class="console-page" :data-page="String(route.name || '')">
    <router-view v-slot="{ Component }"><component :is="Component" :key="route.name" /></router-view>
  </div>
</template>
<script setup lang="ts">
import { computed, provide } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import PageHeading from '@/ui/PageHeading.vue';
import { pageDescriptions } from '@/ui/pageInfo';
import { navigationItems, type NavigationItem } from '@/ui/navigation';
import { taskFamilies, taskTitles, workspaceFor } from '@/ui/workspace';
import store from '@/store';
provide('page-actions-target', '#page-actions');
const route = useRoute(),
  { t, locale } = useI18n();
const title = computed(() => taskTitles[String(route.name)]?.[locale.value === 'en_US' ? 1 : 0] || t(String(route.meta.title || 'HomeWaf')));
const group = computed(() => workspaceFor(String(route.name))?.label[locale.value === 'en_US' ? 1 : 0] || (route.matched[0]?.meta.title ? t(String(route.matched[0].meta.title)) : 'HomeWaf'));
const description = computed(() => (locale.value === 'zh_CN' ? pageDescriptions[String(route.name)] || '' : ''));
const siblings = computed(() => {
  const family = taskFamilies.find((names) => (names as readonly string[]).includes(String(route.name)));
  const permitted = navigationItems(store.getters['permission/routers'], t, locale.value);
  return family?.flatMap((name) => permitted.filter((item) => item.name === name)) || [];
});
function siblingTarget(item: NavigationItem) {
  return { path: item.path, query: route.query.host_code ? { host_code: String(route.query.host_code) } : {} };
}
</script>
