<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="breadcrumb-container">
    <hw-breadcrumb :max-item-width="'150'" class="tdesign-breadcrumb">
      <hw-breadcrumbItem v-for="item in crumbs" :key="item.to" :to="item.to">
        {{ $t(item.title)}}
      </hw-breadcrumbItem>
    </hw-breadcrumb>
    <a href="https://mp.weixin.qq.com/s/AMKCsYO6XcwiUzuCcKUS1g" target="_blank" rel="noopener noreferrer" class="comm-channel-link">{{ $t('common.online_channel') }}</a>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';



export default defineComponent({
  name: 'TdesignStarterBreadcrumb',
  props: {
    isVisible: Boolean,
  },
  computed: {
    crumbs() {
      const pathArray = this.$route.path.split('/');
      pathArray.shift();

      const breadcrumbs = pathArray.reduce((breadcrumbArray, path, idx) => {
        breadcrumbArray.push({
          path,
          to: breadcrumbArray[idx - 1] ? `/${breadcrumbArray[idx - 1].path}/${path}` : `/${path}`,
          title: this.$route.matched[idx].meta.title || path,
        });
        return breadcrumbArray;
      }, []);
      return breadcrumbs;
    },
  },
});
</script>
<style scoped>
.breadcrumb-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.tdesign-breadcrumb {
  margin-bottom: 0;
}
.comm-channel-link {
  font-size: 14px;
  color: var(--td-brand-color);
  text-decoration: none;
}
.comm-channel-link:hover {
  text-decoration: underline;
}
</style>
