<template>
  <div class="resource-panel">
    <div class="certificate-page">
      <RouterLink :to="certificateRoot">返回证书管理</RouterLink>
      <p class="certificate-muted">维护共享验证资源。修改凭据会影响引用该分组的后续申请与续期。</p>
      <section class="certificate-section">
        <h3>实例检查计划</h3>
        <p v-if="error" role="alert" class="certificate-error">{{ error }} <ElButton @click="load">重试</ElButton></p>
        <template v-if="settings"
          ><p>
            任务时区：{{ settings.timezone }}；域名证书提前 {{ settings.renew_days }} 天、IP 证书提前
            {{ settings.ip_renew_days }} 天进入续期窗口。
          </p>
          <dl class="certificate-facts">
            <div v-for="task in settings.tasks" :key="task.id">
              <dt>{{ task.task_name }}</dt>
              <dd>{{ task.task_unit }} / {{ task.task_value }} · {{ task.task_at || '按间隔执行' }}</dd>
            </div>
          </dl>
          <p>
            文件自动加载总开关：{{
              settings.file_sync_enabled ? '已开启' : '已关闭'
            }}。单张证书还需要开启自身的文件加载设置。
          </p>
          <RouterLink to="/sys/task">编辑实例计划任务</RouterLink> ·
          <RouterLink to="/sys/SystemConfig">编辑实例阈值</RouterLink></template
        >
      </section>
      <section v-if="settings?.credential_usage?.length" class="certificate-section">
        <h3>凭据使用范围</h3>
        <p class="certificate-muted">修改同一分组会影响以下证书的后续任务。</p>
        <div v-for="item in settings.credential_usage" :key="item.id">
          <RouterLink :to="certificateRoot + '/' + item.id">{{ item.apply_domain }}</RouterLink> ·
          {{ item.apply_dns || item.apply_method }} / {{ item.private_group_name || '不适用' }} ·
          {{ item.enabled ? '自动续期开启' : '自动续期关闭' }}
        </div>
      </section>
    </div>
    <CertificateRequest ref="resourceEditor" mode="settings" />
  </div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { onBeforeRouteLeave } from 'vue-router';
import { ElButton } from 'element-plus';
import CertificateRequest from '../sslorder/CertificateRequest.vue';
import { certificateRoot, certificateSettingsApi } from '@/apis/certificates';
import './certificates.css';
const resourceEditor = ref<InstanceType<typeof CertificateRequest>>();
onBeforeRouteLeave(() => resourceEditor.value?.confirmDiscard() ?? true);
const settings = ref<any>(),
  error = ref('');
async function load() {
  try {
    const res = await certificateSettingsApi();
    if (res.code !== 0) throw new Error(res.msg);
    settings.value = res.data;
    error.value = '';
  } catch {
    error.value = '实例计划读取失败，不能按默认值判断当前配置。';
  }
}
onMounted(load);
</script>
