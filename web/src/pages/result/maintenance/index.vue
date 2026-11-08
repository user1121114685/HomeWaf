<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <result title="系统维护中" tip="系统维护中，请稍后再试" type="maintenance">
    <hw-button theme="default" @click="() => $router.push('/')">返回首页</hw-button>
  </result>
</template>

<script>
import result from '@/components/result/index.vue';

export default {
  name: 'ResultMaintenance',
  components: { result },
};
</script>
