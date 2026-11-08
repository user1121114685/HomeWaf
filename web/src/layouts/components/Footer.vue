<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div :class="prefix + '-footer'">HomeWaf · <a href="/#/help">许可与第三方声明</a></div>
</template>

<script>
import {defineComponent} from 'vue';

import { prefix } from '@/config/global';


export default defineComponent({
  name: `${prefix}-footer`,
  data() {
    return {
      prefix,
    };
  },
});
</script>

<style lang="less" scoped>
@import '@/style/variables';

.@{starter-prefix}-footer {
  color: var(--td-text-color-placeholder);
  line-height: 20px;
  text-align: center;
}
</style>
