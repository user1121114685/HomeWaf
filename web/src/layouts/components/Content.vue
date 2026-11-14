<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <router-view v-slot="{ Component, route }">
    <keep-alive :include="isUseTabsRouter && route.meta.keepAlive !== false ? aliveViews : []">
      <component :is="Component" v-if="!isRefreshing" />
    </keep-alive>
  </router-view>
</template>
<script lang="ts">
import { mapGetters } from '@/store/helpers';

export default {
  computed: {
    ...mapGetters({
      tabRouterList: 'tabRouter/tabRouterList',
      isRefreshing: 'tabRouter/isRefreshing',
      isUseTabsRouter: 'setting/isUseTabsRouter',
    }),
    aliveViews() {
      return this.tabRouterList.filter((route) => route.isAlive).map((route) => route.name);
    },
  },
};
</script>
