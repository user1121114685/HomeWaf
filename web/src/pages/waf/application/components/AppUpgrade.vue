<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div class="application-maintenance" data-testid="application-maintenance">
    <p class="editor-context">
      当前应用：<code>{{ appCode }}</code
      >。更新与恢复都会作用于这个应用的文件。
    </p>
    <hw-alert v-if="operationError" theme="error" :close-btn="false">{{ operationError }}</hw-alert>
    <hw-tabs v-model="activeTask">
      <hw-tab-panel value="update" label="更新应用" :disabled="uploading">
        <EditorSection title="更新文件" description="选择文件并核对 SHA-256，再选择仅上传或备份后升级。">
          <hw-form :data="formData" label-align="right" :label-width="120" :inert="uploading">
            <hw-form-item :label="$t('page.application.upload_file')">
              <hw-upload :action="''" :auto-upload="false" accept="*" :multiple="false" @change="onFileChange">
                <hw-button variant="outline">{{ $t('page.application.select_file') }}</hw-button>
              </hw-upload>
              <div v-if="selectedFile" class="editor-context">
                {{ selectedFile.name }} ({{ formatSize(selectedFile.size) }})
              </div>
            </hw-form-item>
            <hw-form-item :label="$t('page.application.hash_sha256')">
              <hw-input
                v-model="formData.hash"
                :placeholder="
                  hashComputing ? $t('page.application.hash_computing') : $t('page.application.hash_placeholder')
                "
                :disabled="hashComputing"
                clearable
              />
            </hw-form-item>
            <hw-form-item :label="$t('page.application.upgrade_action')">
              <hw-radio-group v-model="upgradeMode">
                <hw-radio value="upload">{{ $t('page.application.upload_only') }}</hw-radio>
                <hw-radio value="upgrade">{{ $t('page.application.upload_and_upgrade') }}</hw-radio>
              </hw-radio-group>
            </hw-form-item>
          </hw-form>
        </EditorSection>
        <p class="editor-context">升级会先等待进程停止，再备份并替换文件。完成后请检查运行状态与应用日志。</p>
      </hw-tab-panel>
      <hw-tab-panel value="restore" label="恢复备份" :disabled="uploading">
        <EditorSection
          title="可恢复的备份"
          description="选择备份后确认恢复。恢复前会停止应用，当前文件将被所选备份替换。"
        >
          <div class="editor-toolbar">
            <span>{{ backups.length }} 份备份</span
            ><hw-button :loading="backupsLoading" :disabled="uploading" @click="loadBackups">刷新备份</hw-button>
          </div>
          <hw-alert v-if="backupsError" theme="error" :close-btn="false">{{ backupsError }}</hw-alert>
          <hw-table
            v-if="backupsLoaded"
            :data="backups"
            :columns="backupColumns"
            row-key="filename"
            size="small"
            :empty="$t('page.application.no_backups')"
          >
            <template #op="slotProps">
              <hw-button
                variant="text"
                :disabled="uploading || backupsLoading || !!backupsError"
                @click="doRollback(slotProps.row.filename)"
              >
                {{ $t('page.application.rollback') }}
              </hw-button>
            </template>
          </hw-table>
        </EditorSection>
      </hw-tab-panel>
    </hw-tabs>
    <p v-if="uploading" role="status" class="editor-context">{{ uploadBtnLabel }}</p>
    <div class="editor-footer">
      <hw-button
        v-if="activeTask === 'update'"
        theme="primary"
        :loading="uploading"
        @click="doUpload"
        :disabled="!selectedFile || hashComputing"
      >
        {{ uploadBtnLabel }}
      </hw-button>
      <hw-button theme="default" :disabled="uploading" @click="$emit('close')">{{ $t('common.cancel') }}</hw-button>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';
import { ElMessageBox } from 'element-plus';

import { hashBytes } from '@/utils/fileHash';
import request from '@/utils/request';
import {
  wafAppBackupsApi,
  wafAppStatusApi,
  buildUploadConfig,
  buildRollbackConfig,
  getAppOpPassword,
} from '@/apis/application';

export default defineComponent({
  name: 'AppUpgrade',
  emits: ['done', 'close'],
  props: {
    appCode: { type: String, default: '' },
    opPassword: { type: String, default: '' },
  },
  data() {
    return {
      activeTask: 'update',
      operationError: '',
      backupsLoading: false,
      backupsLoaded: false,
      backupsError: '',
      formData: { hash: '' },
      selectedFile: null as File | null,
      upgradeMode: 'upgrade',
      uploading: false,
      uploadPhase: '' as '' | 'stopping' | 'uploading',
      hashComputing: false,
      backups: [],
      backupColumns: [
        { colKey: 'filename', title: this.$t('page.application.backup_filename'), ellipsis: true },
        {
          colKey: 'size',
          title: this.$t('page.application.backup_size'),
          width: 100,
          cell: (h, { row }) => this.formatSize(row.size),
        },
        {
          colKey: 'created_at',
          title: this.$t('page.application.backup_time'),
          width: 180,
          cell: (h, { row }) => row.created_at || '-',
        },
        { colKey: 'op', title: this.$t('common.op'), width: 80, cell: 'op' },
      ],
    };
  },
  computed: {
    uploadBtnLabel(): string {
      if (this.uploading) {
        if (!this.uploadPhase) return '等待确认恢复';
        if (this.uploadPhase === 'stopping') return this.$t('page.application.stopping') as string;
        return this.$t('page.application.upgrading') as string;
      }
      return this.upgradeMode === 'upgrade'
        ? (this.$t('page.application.upload_and_upgrade') as string)
        : (this.$t('page.application.upload_only') as string);
    },
  },
  mounted() {
    this.loadBackups();
  },
  methods: {
    onFileChange(files) {
      if (files && files.length > 0) {
        this.selectedFile = files[0].raw || files[0];
        this.computeHash(this.selectedFile);
      } else {
        this.selectedFile = null;
        this.formData.hash = '';
      }
    },
    computeHash(file: File) {
      this.hashComputing = true;
      this.formData.hash = '';
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const hash = hashBytes(e.target!.result as ArrayBuffer);
          this.formData.hash = hash;
        } catch (_) {
          this.formData.hash = '';
        } finally {
          this.hashComputing = false;
        }
      };
      reader.onerror = () => {
        this.formData.hash = '';
        this.hashComputing = false;
      };
      reader.readAsArrayBuffer(file);
    },
    formatSize(bytes: number): string {
      if (!bytes) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    },
    // 根据文件大小动态计算超时：假设最慢 100 KB/s，再加 90s 处理余量
    calcTimeout(fileSize: number): number {
      return Math.max(300000, Math.ceil(fileSize / 102400) * 1000 + 90000);
    },
    // 停止应用并轮询直到确认已停止（最多等 60s）
    // stop 本身不在高危组，但升级/回滚流程需要携带密码头以备路由规则变动
    stopAndWait(): Promise<void> {
      return new Promise((resolve, reject) => {
        const pwd = this.opPassword || getAppOpPassword();
        const headers: any = pwd ? { 'X-App-Op-Password': pwd } : {};
        request({
          url: '/application/app/stop',
          method: 'get',
          params: { code: this.appCode },
          timeout: 15000,
          headers,
        })
          .then(() => {
            const deadline = Date.now() + 60000;
            const poll = () => {
              if (Date.now() >= deadline) {
                reject(new Error(this.$t('page.application.stop_wait_timeout') as string));
                return;
              }
              wafAppStatusApi({ code: this.appCode })
                .then((res) => {
                  if (res && res.code === 0 && res.data.run_status === 0) {
                    resolve(undefined);
                  } else {
                    setTimeout(poll, 1000);
                  }
                })
                .catch(() => setTimeout(poll, 1000));
            };
            setTimeout(poll, 800);
          })
          .catch(reject);
      });
    },
    async doUpload() {
      if (!this.selectedFile || this.uploading || this.hashComputing) return;
      this.operationError = '';
      this.uploading = true;
      try {
        if (this.upgradeMode === 'upgrade') {
          // 步骤 1：先停止，确认进程退出后再发送文件
          this.uploadPhase = 'stopping';
          await this.stopAndWait();
        }

        // 步骤 2：上传（此时后端 StopApp 因进程已退出会即时返回，超时仅受网速影响）
        this.uploadPhase = 'uploading';
        const fd = new FormData();
        fd.append('code', this.appCode);
        fd.append('hash', this.formData.hash);
        fd.append('file', this.selectedFile);
        const url = this.upgradeMode === 'upgrade' ? '/application/app/upgrade' : '/application/app/upload';
        const timeout = this.calcTimeout(this.selectedFile.size);

        const res = await request(buildUploadConfig(url, fd, timeout, this.opPassword));
        if (res && res.code === 0) {
          this.$message.success(res.msg || this.$t('common.success'));
          this.loadBackups();
          this.$emit('done');
        } else {
          this.operationError = res?.msg || this.$t('common.failed');
        }
      } catch (err: any) {
        this.operationError = err.message || this.$t('common.failed');
      } finally {
        this.uploading = false;
        this.uploadPhase = '';
      }
    },
    async loadBackups() {
      if (this.backupsLoading) return;
      this.backupsLoading = true;
      this.backupsError = '';
      try {
        const res = await wafAppBackupsApi({ code: this.appCode });
        if (!res || res.code !== 0) throw new Error(res?.msg || '备份读取失败');
        this.backups = res.data.list || [];
        this.backupsLoaded = true;
      } catch (error: any) {
        this.backupsError =
          (error.message || '备份读取失败') + (this.backupsLoaded ? '，当前保留上次结果。' : '，请重试。');
      } finally {
        this.backupsLoading = false;
      }
    },
    async doRollback(filename: string) {
      if (this.uploading || this.backupsLoading || this.backupsError) return;
      this.uploading = true;
      try {
        await ElMessageBox.confirm(
          `将停止应用 ${this.appCode}，并使用 ${filename} 替换当前文件。是否继续？`,
          '恢复应用备份',
          { type: 'warning', confirmButtonText: '停止并恢复', cancelButtonText: '取消' },
        );
      } catch {
        this.uploading = false;
        return;
      }
      this.operationError = '';
      this.uploading = true;
      this.uploadPhase = 'stopping';
      this.stopAndWait()
        .then(() => {
          this.uploadPhase = 'uploading';
          return request(buildRollbackConfig({ code: this.appCode, filename }, this.opPassword));
        })
        .then((res) => {
          if (res && res.code === 0) {
            this.$message.success(res.msg || this.$t('page.application.rollback_success'));
            this.$emit('done');
          } else {
            this.operationError = res?.msg || this.$t('common.failed');
          }
        })
        .catch((err) => {
          this.operationError = err.message || this.$t('common.failed');
        })
        .finally(() => {
          this.uploading = false;
          this.uploadPhase = '';
        });
    },
  },
});
</script>

<style scoped>
.t-button-link {
  color: var(--td-brand-color);
  cursor: pointer;
}
</style>
