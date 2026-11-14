<template>
  <PageHeading :title="title" :eyebrow="group" :description="description" />
  <div class="console-page" :data-page="String(route.name || '')">
    <router-view v-slot="{ Component }"><component :is="Component" :key="route.name" /></router-view>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import PageHeading from '@/ui/PageHeading.vue';
import { pageDescriptions } from '@/ui/pageInfo';
const route = useRoute(),
  { t, locale } = useI18n();
const title = computed(() => t(String(route.meta.title || 'HomeWaf')));
const group = computed(() => (route.matched[0]?.meta.title ? t(String(route.matched[0].meta.title)) : 'HomeWaf'));
const description = computed(() => (locale.value === 'zh_CN' ? pageDescriptions[String(route.name)] || '' : ''));
</script>
