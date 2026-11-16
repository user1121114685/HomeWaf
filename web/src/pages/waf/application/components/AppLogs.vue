<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div data-testid="application-logs">
    <div class="editor-toolbar">
      <span class="editor-context"
        >应用 <code>{{ appCode }}</code> · {{ logs.length }} 行</span
      >
      <div>
        <hw-button :loading="loading" :disabled="clearing" @click="loadLogs">{{ $t('common.refresh') }}</hw-button>
        <hw-button
          theme="danger"
          variant="outline"
          :loading="clearing"
          :disabled="loading || !logs.length"
          @click="clearLogs"
          >{{ $t('page.application.clear_logs') }}</hw-button
        >
      </div>
    </div>
    <hw-alert v-if="error" theme="error" :close-btn="false">{{ error }}</hw-alert>
    <div ref="logBox" class="log-box" tabindex="0" aria-label="应用运行日志" :aria-busy="loading">
      <div v-if="logs.length === 0" class="log-empty">
        {{ loading ? '正在读取日志…' : error ? '日志未能读取，请刷新重试。' : $t('page.application.no_logs') }}
      </div>
      <div v-for="(line, idx) in logs" :key="idx" class="log-line">{{ line }}</div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { ElMessageBox } from 'element-plus';

import { wafAppLogsApi, wafAppClearLogsApi } from '@/apis/application';

export default defineComponent({
  name: 'AppLogs',
  props: {
    appCode: { type: String, default: '' },
  },
  data() {
    return { logs: [] as string[], loading: false, clearing: false, error: '' };
  },
  mounted() {
    this.loadLogs();
  },
  methods: {
    async loadLogs() {
      if (!this.appCode || this.loading || this.clearing) return;
      this.loading = true;
      this.error = '';
      try {
        const res = await wafAppLogsApi({ code: this.appCode });
        if (!res || res.code !== 0) throw new Error(res?.msg || '日志读取失败');
        this.logs = res.data.logs || [];
        this.$nextTick(() => {
          const box = this.$refs.logBox as HTMLElement;
          if (box) box.scrollTop = box.scrollHeight;
        });
      } catch (error: any) {
        this.error = (error.message || '日志读取失败') + (this.logs.length ? '，仍显示上次读取的日志。' : '');
      } finally {
        this.loading = false;
      }
    },
    async clearLogs() {
      if (this.clearing || this.loading) return;
      this.clearing = true;
      try {
        await ElMessageBox.confirm(`清空应用 ${this.appCode} 当前保留的运行日志？此操作无法撤销。`, '清空应用日志', {
          type: 'warning',
          confirmButtonText: '确认清空',
          cancelButtonText: '取消',
        });
        const res = await wafAppClearLogsApi({ code: this.appCode });
        if (!res || res.code !== 0) throw new Error(res?.msg || '日志清空失败');
        this.logs = [];
        this.error = '';
        this.$message.success(this.$t('page.application.logs_cleared'));
      } catch (error: any) {
        if (error !== 'cancel' && error !== 'close') this.error = error.message || '日志清空失败';
      } finally {
        this.clearing = false;
      }
    },
  },
});
</script>

<style scoped>
.log-box {
  background: var(--hw-subtle);
  color: var(--hw-ink);
  font-family: var(--font-mono);
  font-size: 13px;
  height: min(52dvh, 480px);
  overflow-y: auto;
  padding: 12px;
  border-radius: 4px;
}
.log-line {
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-empty {
  color: var(--hw-muted);
  text-align: center;
  padding: 64px 16px;
}
</style>
