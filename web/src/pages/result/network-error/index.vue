<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <result title="网络异常" tip="网络异常，请稍后再试" type="wifi">
    <div>
      <hw-button @click="() => $router.push('/')">重新加载</hw-button>
      <hw-button theme="default" @click="() => $router.push('/')">返回首页</hw-button>
    </div>
  </result>
</template>

<script>
import result from '@/components/result/index.vue';

export default {
  name: 'ResultNetworkError',
  components: { result },
};
</script>
<style lang="less" scoped>
@import '@/style/variables.less';

.t-button + .t-button {
    margin-left: @spacer;
 }
</style>