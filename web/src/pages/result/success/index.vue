<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="result-success">
    <check-circle-icon class="result-success-icon" />
    <div class="result-success-title">项目已创建成功</div>
    <div class="result-success-describe">可以联系负责人分发应用</div>
    <div>
      <hw-button @click="() => this.$router.push('/form/base')">返回首页</hw-button>
      <hw-button @click="() => this.$router.push('/detail/advanced')" theme="default">查看进度</hw-button>
    </div>
  </div>
</template>

<script>
import { CheckCircleIcon } from '@/ui/icons';

export default {
  name: 'ResultSuccess',
  components: {
    CheckCircleIcon,
  },
};
</script>

<style lang="less" scoped>
@import '@/style/variables.less';

.result-success {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 75vh;

  &-icon {
    font-size: 72px;
    color: var(--td-success-color);
  }

  &-title {
    margin-top: 28px;
    font-size: 20px;
    color: var(--td-text-color-primary);
    text-align: center;
    line-height: 28px;
    font-weight: 500;
  }

  &-describe {
    margin: 8px 0 32px;
    font-size: 14px;
    color: var(--td-text-color-secondary);
    line-height: 22px;
  }
}

.t-button + .t-button {
    margin-left: @spacer;
 }
</style>
