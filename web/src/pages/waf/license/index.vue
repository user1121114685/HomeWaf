<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <el-card class="license-workspace">
    <PageActions><el-button :disabled="busy" @click="refresh">刷新状态</el-button></PageActions>
    <el-alert :closable="false" type="info" title="仅非商业用途免费；公司内部部署等商业用途需取得书面授权。" />
    <el-descriptions :column="1" border style="margin: 20px 0">
      <el-descriptions-item label="实例 ID"><code>{{ detail.machine_id || '加载中…' }}</code></el-descriptions-item>
      <el-descriptions-item label="状态">{{ status }}</el-descriptions-item>
      <el-descriptions-item label="被授权方">{{ detail.license?.username || '—' }}</el-descriptions-item>
      <el-descriptions-item label="授权编号">{{ detail.license?.license_id || '—' }}</el-descriptions-item>
      <el-descriptions-item label="有效期">{{ detail.license?.license_id ? detail.license.expiry_date : '—' }}</el-descriptions-item>
      <el-descriptions-item label="功能">{{ detail.license?.features?.join(', ') || '—' }}</el-descriptions-item>
    </el-descriptions>
    <section class="license-import"><h2>导入授权文件</h2><p>向发行方提供实例 ID，获得签名文件后先验证，再确认保存。每个管理会话的待确认文件独立，有效期为十分钟。</p>
    <input ref="fileInput" type="file" accept=".json,.bin,application/json" aria-label="选择 HomeWaf 授权文件" @change="selectFile" />
    <div style="margin: 16px 0; display: flex; gap: 12px">
      <el-button :disabled="!file" :loading="busy" @click="check">验证文件</el-button>
      <el-button type="primary" :disabled="!verified" :loading="busy" @click="confirm">确认保存授权</el-button>
    </div>
    <p role="status">{{ message }}</p></section>
    <a href="/#/help">许可条款与第三方声明</a>
  </el-card>
</template>
<script setup lang="ts">
import {computed, onMounted, ref} from 'vue';
import {ElAlert, ElButton, ElCard, ElDescriptions, ElDescriptionsItem} from 'element-plus';
import request from '@/utils/request';
const detail = ref<any>({});
const file = ref<File>();
const busy = ref(false);
const verified = ref(false);
const message = ref('');
const loaded = ref(false);
const status = computed(() => !loaded.value ? (message.value ? '状态未知，请刷新重试' : '正在读取状态…') : !detail.value.license?.license_id ? '非商业版（无商业授权）' : detail.value.license.is_expiry ? '商业授权已过期' : '商业授权有效');
function selectFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0]; verified.value = false; message.value = '';
  if (file.value && file.value.size > 65536) {file.value = undefined; message.value = '授权文件不能超过 64 KiB';}
}
async function refresh() {
  try {const result = await request({url: '/license/detail'}); if (result.code === 0) {detail.value = result.data; loaded.value = true;} else message.value = result.msg;}
  catch {message.value = '授权状态读取失败';}
}
async function check() {
  if (!file.value) return;
  busy.value = true; verified.value = false;
  try {
    const form = new FormData(); form.append('file', file.value);
    const result = await request({url: '/license/checklicense', method: 'post', data: form});
    message.value = result.msg; verified.value = result.code === 0;
  } catch {message.value = '文件验证请求失败';}
  finally {busy.value = false;}
}
async function confirm() {
  busy.value = true;
  try {
    const result = await request({url: '/license/confirm', method: 'post', data: {}});
    message.value = result.msg;
    if (result.code === 0) {detail.value = result.data; loaded.value = true; verified.value = false;}
  } catch {message.value = '授权保存请求失败';}
  finally {busy.value = false;}
}
onMounted(refresh);
</script>
<style scoped>
.license-workspace { max-width: 1000px; }
.license-import { padding-top: 24px; margin-top: 24px; border-top: 1px solid var(--hw-line); }
.license-import h2 { font-size: 17px; margin: 0 0 12px; }
.license-import p { color: var(--hw-muted); line-height: 1.8; margin-bottom: 16px; }
.license-import input { max-width: 100%; padding: 12px; border: 1px dashed var(--hw-line); border-radius: 6px; background: var(--hw-subtle); }
</style>
