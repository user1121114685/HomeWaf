<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <template v-for="item in list" :key="item.path">
      <template v-if="!item.children || !item.children.length || item.meta?.single">
        <hw-menu-item
                    v-if="getHref(item)"
          :href="getHref(item)?.[0]"
          :name="item.path"
          :value="item.meta?.single ? item.redirect : item.path"
        >
          <template #icon>
            <hw-icon v-if="typeof item.icon === 'string' && item.icon" :name="item.icon" />
            <renderFnIcon :item="item" />
          </template>
          {{ $t(item.title) }}
        </hw-menu-item>
        <hw-menu-item
                    v-else
          :name="item.path"
          :value="item.meta?.single ? item.redirect : item.path"
          :to="item.path"
        >
          <template #icon>
            <hw-icon v-if="typeof item.icon === 'string' && item.icon" :name="item.icon" />
            <renderFnIcon :item="item" />
          </template>
          {{ $t(item.title) }}
        </hw-menu-item>
      </template>
      <hw-submenu v-else :name="item.path" :value="item.path" :title="$t(item.title)">
        <template #icon>
          <hw-icon v-if="typeof item.icon === 'string' && item.icon" :name="item.icon" />
          <renderFnIcon :item="item" />
        </template>
        <menu-content v-if="item.children" :nav-data="item.children" />
      </hw-submenu>
    </template>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';

import {PropType,h} from 'vue';

import { prefix } from '@/config/global';
import { MenuRoute } from '@/interface';

const getMenuList = (list: MenuRoute[], basePath?: string): MenuRoute[] => {
  if (!list) {
    return [];
  }
  return list
    .map((item) => {
      const path = basePath ? `${basePath}/${item.path}` : item.path;
      return {
        path,
        title: item.meta?.title,
        icon: item.meta?.icon || '',
        children: getMenuList(item.children, path),
        meta: item.meta,
        redirect: item.redirect,
      };
    })
    .filter((item) => item.meta && item.meta.hidden !== true);
};

const RenderFnIcon=defineComponent({
  props: {
    item: {
      type: Object as PropType<MenuRoute>,
      required: true,
    },
  },
  // 遵循最小改动的原则，这里仍然使用 createElement
  render() {
    if (typeof this.item.icon === 'function' || (this.item.icon && (typeof this.item.icon.render === 'function' || typeof this.item.icon.setup === 'function'))) {
      return h(this.item.icon, {
        class: 't-icon',
      });
    }
    return undefined;
  },
});

export default defineComponent({
  name: 'MenuContent',
  components:{renderFnIcon:RenderFnIcon},
  props: {
    navData: Array,
  },
  data() {
    return {
      prefix,
    };
  },
  computed: {
    list(): Array<MenuRoute> {
      return getMenuList(this.navData);
    },
  },
  methods: {
    getHref(item: MenuRoute) {
      return item.path.match(/(http|https):\/\/([\w.]+\/?)\S*/);
    },
  },
});
</script>
