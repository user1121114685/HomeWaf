<template>
  <ElDrawer v-model="visible" title="显示与本机设置" size="400px" class="console-preferences">
    <section class="preference-section">
      <h2>界面外观</h2>
      <p>此设置仅保存在当前浏览器。</p>
      <ElRadioGroup v-model="mode" aria-label="界面外观" class="appearance-options">
        <ElRadioButton value="light"><Sunny />浅色</ElRadioButton>
        <ElRadioButton value="dark"><Moon />深色</ElRadioButton>
        <ElRadioButton value="auto"><Monitor />跟随系统</ElRadioButton>
      </ElRadioGroup>
    </section>
    <section class="preference-section">
      <h2>请求等待时间</h2>
      <p>只影响当前浏览器的管理请求，不修改服务端监听或转发超时。</p>
      <label for="console-request-timeout">等待时间（秒）</label>
      <ElInputNumber
        id="console-request-timeout"
        v-model="timeout"
        :min="MIN_REQUEST_TIMEOUT / 1000"
        :max="MAX_REQUEST_TIMEOUT / 1000"
        :step="1"
        controls-position="right"
      />
      <div class="preference-actions">
        <ElButton type="primary" @click="saveTimeout">保存等待时间</ElButton
        ><ElButton @click="resetTimeout">恢复默认</ElButton>
      </div>
      <p class="preference-note">默认 {{ DEFAULT_REQUEST_TIMEOUT / 1000 }} 秒。保存后立即生效。</p>
    </section>
  </ElDrawer>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElButton, ElDrawer, ElInputNumber, ElMessage, ElRadioButton, ElRadioGroup } from 'element-plus';
import { Monitor, Moon, Sunny } from '@element-plus/icons-vue';
import store from '@/store';
import {
  getRequestTimeout,
  setRequestTimeout,
  DEFAULT_REQUEST_TIMEOUT,
  MIN_REQUEST_TIMEOUT,
  MAX_REQUEST_TIMEOUT,
} from '@/config/requestTimeout';
const visible = computed({
  get: () => store.state.setting.showSettingPanel,
  set: (value) => store.commit('setting/toggleSettingPanel', value),
});
const mode = computed({
  get: () => store.state.setting.mode,
  set: (value) => store.dispatch('setting/changeTheme', { ...store.state.setting, mode: value }),
});
const timeout = ref(getRequestTimeout() / 1000);
function saveTimeout() {
  timeout.value = setRequestTimeout(Number(timeout.value) * 1000) / 1000;
  ElMessage.success('等待时间已保存');
}
function resetTimeout() {
  timeout.value = setRequestTimeout(DEFAULT_REQUEST_TIMEOUT) / 1000;
  ElMessage.success('已恢复默认等待时间');
}
</script>
<style scoped>
.preference-section + .preference-section {
  border-top: 1px solid var(--hw-line);
  margin-top: 32px;
  padding-top: 32px;
}
.preference-section h2 {
  font-size: 16px;
  font-weight: 500;
  margin-bottom: 8px;
}
.preference-section p {
  color: var(--hw-muted);
  line-height: 1.8;
  margin-bottom: 20px;
}
.appearance-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;
}
.appearance-options :deep(.el-radio-button__inner) {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 20px 8px;
  width: 100%;
}
.appearance-options svg {
  width: 24px;
  height: 24px;
}
.preference-section label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
}
.preference-actions {
  display: flex;
  gap: 8px;
  margin: 20px 0 12px;
}
.preference-note {
  font-size: 13px;
}
</style>
