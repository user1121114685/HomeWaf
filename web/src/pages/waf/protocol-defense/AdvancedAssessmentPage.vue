<template>
  <main class="advanced-assessment-page">
    <header class="advanced-heading">
      <div>
        <p>防御评分 · 高级工具</p>
        <h1>自定义测试</h1>
      </div>
      <RouterLink :to="{ name: 'WafProtocolDefense', query: { tab: 'scores' } }">返回自动检测</RouterLink>
    </header>
    <p class="advanced-intro">
      编辑一条 HTTP 请求，查看所选服务会在哪一步放行、观察、挑战或拦截。只运行隔离判定，不访问源站，也不保存服务草稿。
    </p>
    <ElAlert v-if="handoffError" type="warning" :closable="false" :title="handoffError" />
    <ElSkeleton v-else-if="waiting" :rows="5" animated />
    <AdvancedAssessmentEditor
      v-else
      :draft-config="handoff.config"
      :initial-source="handoff.source"
      :initial-port="port"
    />
  </main>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { ElAlert, ElSkeleton } from 'element-plus';
import AdvancedAssessmentEditor from './AdvancedAssessmentEditor.vue';
import { receiveAssessmentHandoff, type AssessmentHandoff } from './assessmentWindow';
const route = useRoute(),
  port = Number(route.query.port) || undefined;
const waiting = ref(!!route.query.handoff),
  handoff = ref<AssessmentHandoff>({}),
  handoffError = ref('');
let stop = () => {};
onMounted(() => {
  if (typeof route.query.handoff === 'string')
    stop = receiveAssessmentHandoff(
      route.query.handoff,
      (value) => {
        handoff.value = value;
        waiting.value = false;
      },
      () => {
        waiting.value = false;
        handoffError.value = '原窗口的测试草稿已不可用。请返回原窗口，重新打开“自定义测试”；未切换为已保存配置。';
      },
    );
});
onBeforeUnmount(() => stop());
</script>
<style scoped>
.advanced-assessment-page {
  display: grid;
  gap: 20px;
  min-width: 0;
}
.advanced-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.advanced-heading p,
.advanced-intro {
  margin: 0;
  color: var(--hw-muted);
  line-height: 1.7;
}
.advanced-heading h1 {
  font-size: 24px;
  margin: 5px 0 0;
}
@media (max-width: 760px) {
  .advanced-heading {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
