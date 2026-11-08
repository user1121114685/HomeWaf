<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <img :class="className" :src="url" />
</template>
<script lang="ts">
import {defineComponent} from 'vue';



export default defineComponent({
  name: 'thumbnail',
  props: {
    url: {
      type: String,
      default: '',
    },
    type: {
      type: String,
      default: 'layout',
    },
  },
  computed: {
    className() {
      return [
        'thumbnail-container',
        {
          'thumbnail-circle': this.type === 'circle',
          'thumbnail-layout': this.type === 'layout',
        },
      ];
    },
  },
});
</script>
<style lang="less" scoped>
@import url('@/style/index.less');

.thumbnail {
  &-container {
    display: inline-block;
  }

  &-circle {
    border-radius: @border-radius-50;
  }

  &-layout {
    width: 88px;
    height: 48px;
  }
}
</style>
