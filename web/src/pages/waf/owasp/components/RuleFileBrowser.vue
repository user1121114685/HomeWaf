<template>
  <ElDialog
    v-model="visible"
    title="OWASP 规则来源文件"
    width="min(1000px,96vw)"
    :close-on-click-modal="false"
    @closed="close"
  >
    <p class="file-help">只读查看已安装的规则文件。单条规则的启用、改写仍在规则列表中操作。</p>
    <div class="file-toolbar">
      <label for="owasp-source-file">规则文件</label>
      <ElSelect
        id="owasp-source-file"
        v-model="selected"
        filterable
        :loading="listLoading"
        placeholder="选择来源文件"
        aria-label="规则文件"
        @change="readContent"
      >
        <ElOption v-for="file in files" :key="file.file" :label="file.file" :value="file.file" />
      </ElSelect>
      <ElButton :loading="listLoading" @click="loadFiles">刷新文件列表</ElButton>
    </div>
    <ElAlert v-if="listError" :title="listError" type="error" :closable="false" />
    <p v-else-if="!listLoading && !files.length" role="status">尚无可查看的规则文件。请检查 OWASP 安装状态。</p>
    <p v-if="metadata" class="file-help">
      {{ metadata.rule_count }} 条规则 · 修改时间 {{ new Date(metadata.mtime * 1000).toLocaleString() }}
    </p>
    <p v-if="contentLoading" role="status">正在读取文件内容…</p>
    <ElAlert v-else-if="contentError" :title="contentError" type="error" :closable="false"
      ><ElButton text @click="readContent">重试读取文件</ElButton></ElAlert
    >
    <pre v-else-if="selected && contentLoaded" class="file-content" tabindex="0" aria-label="规则文件原文">{{
      content || '（空文件）'
    }}</pre>
    <template #footer><ElButton @click="visible = false">返回规则列表</ElButton></template>
  </ElDialog>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElAlert, ElButton, ElDialog, ElOption, ElSelect } from 'element-plus';
import { owaspFilesListApi, owaspFileContentApi } from '@/apis/owasp';
const visible = ref(false),
  selected = ref(''),
  files = ref<any[]>([]);
const listLoading = ref(false),
  contentLoading = ref(false),
  contentLoaded = ref(false);
const listError = ref(''),
  contentError = ref(''),
  content = ref('');
const metadata = computed(() => files.value.find((file) => file.file === selected.value));
let listSequence = 0,
  contentSequence = 0;
async function loadFiles() {
  const sequence = ++listSequence;
  listLoading.value = true;
  listError.value = '';
  try {
    const result = await owaspFilesListApi();
    if (!visible.value || sequence !== listSequence) return;
    if (result.code !== 0 || !Array.isArray(result.data)) throw new Error(result.msg || '文件列表读取失败');
    files.value = result.data;
    if (selected.value && !files.value.some((file) => file.file === selected.value)) {
      selected.value = '';
      content.value = '';
      contentLoaded.value = false;
      contentSequence++;
      contentLoading.value = false;
      contentError.value = '';
    }
    if (selected.value) await readContent();
  } catch (cause: any) {
    if (sequence === listSequence) listError.value = cause.message || '文件列表读取失败，请重试。';
  } finally {
    if (sequence === listSequence) listLoading.value = false;
  }
}
async function readContent() {
  const sequence = ++contentSequence,
    path = selected.value;
  content.value = '';
  contentError.value = '';
  contentLoaded.value = false;
  if (!path) return;
  contentLoading.value = true;
  try {
    const result = await owaspFileContentApi({ path });
    if (!visible.value || sequence !== contentSequence) return;
    if (result.code !== 0 || typeof result.data?.content !== 'string') throw new Error(result.msg || '文件读取失败');
    content.value = result.data.content;
    contentLoaded.value = true;
  } catch (cause: any) {
    if (sequence === contentSequence) contentError.value = cause.message || '文件读取失败，请重试。';
  } finally {
    if (sequence === contentSequence) contentLoading.value = false;
  }
}
function close() {
  listSequence++;
  contentSequence++;
  content.value = '';
  contentLoaded.value = false;
  listLoading.value = false;
  contentLoading.value = false;
}
function open(path = '') {
  selected.value = path;
  content.value = '';
  contentLoaded.value = false;
  contentError.value = '';
  visible.value = true;
  void loadFiles();
}
defineExpose({ open });
</script>
<style scoped>
.file-help {
  color: var(--hw-muted);
  line-height: 1.7;
}
.file-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  margin: 18px 0;
}
.file-toolbar .el-select {
  flex: 1;
  min-width: 200px;
}
.file-content {
  max-height: 58vh;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  padding: 16px;
  border: 1px solid var(--hw-line);
  border-radius: 6px;
  background: var(--hw-bg);
  color: var(--hw-ink);
  font-size: 13px;
  line-height: 1.7;
}
@media (max-width: 600px) {
  .file-toolbar {
    align-items: stretch;
  }
  .file-toolbar label {
    width: 100%;
  }
  .file-toolbar .el-select {
    flex-basis: 100%;
  }
}
</style>
