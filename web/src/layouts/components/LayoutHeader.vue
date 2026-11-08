<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <common-header
    v-if="showHeader"
    :showLogo="showHeaderLogo"
    :theme="mode"
    :layout="setting.layout"
    :isFixed="setting.isHeaderFixed"
    :menu="headerMenu"
    :isCompact="setting.isSidebarCompact"
    :maxLevel="setting.splitMenu ? 1 : 3"
  />
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { mapGetters } from '@/store/helpers';
import CommonHeader from './Header.vue';

import { SettingType } from '@/interface';

export default defineComponent({
  name: 'LayoutHeader',
  components: {
    CommonHeader,
  },
  computed: {
    ...mapGetters({
      showHeader: 'setting/showHeader',
      showHeaderLogo: 'setting/showHeaderLogo',
      mode: 'setting/mode',
      menuRouters: 'permission/routers',
    }),
    setting(): SettingType {
      return this.$store.state.setting;
    },
    headerMenu() {
      const { layout, splitMenu } = this.$store.state.setting;
      const { menuRouters } = this;
      if (layout === 'mix') {
        if (splitMenu) {
          return menuRouters.map((menu) => ({
            ...menu,
            children: [],
          }));
        }
        return [];
      }
      return menuRouters;
    },
  },
});
</script>
