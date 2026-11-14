<template>
  <div class="screen-controls">
    <button class="control-button" :disabled="!target" @click="openTarget">防护节点</button>
    <button class="control-button" @click="openShares">只读分享</button>
    <dialog ref="targetDialog" class="control-dialog" aria-labelledby="target-title" @click="closeBackdrop">
      <div class="control-heading">
        <h2 id="target-title">地图防护节点</h2>
        <button aria-label="关闭节点设置" @click="targetDialog?.close()">×</button>
      </div>
      <p>
        自动识别服务器出网公网 IP 和城市，优先使用国内 IPIP 服务，失败后尝试 IPinfo。定位由后端完成，每 30 分钟更新；IP
        定位为近似位置。
      </p>
      <form @submit.prevent="saveTarget">
        <label>节点名称<input v-model="targetForm.label" required maxlength="32" /></label>
        <label
          >显示方式<select v-model="targetForm.mode" aria-label="显示方式">
            <option value="auto">自动获取公网 IP 与城市</option>
            <option value="coordinates">自定义部署坐标</option>
          </select></label
        >
        <div v-if="targetForm.mode === 'auto'" class="node-discovery">
          <dl v-if="target?.status === 'ready' && target.mode === 'auto'">
            <dt>公网 IP</dt>
            <dd>{{ target.public_ip }}</dd>
            <dt>归属城市</dt>
            <dd>{{ [target.country, target.region, target.city].filter(Boolean).join(' · ') }}</dd>
            <dt>数据来源</dt>
            <dd>{{ target.source === 'ipip.net' ? 'IPIP（国内优先）' : 'IPinfo（备用）' }}</dd>
            <dt>探测时间</dt>
            <dd>{{ target.checked_at ? new Date(target.checked_at).toLocaleString() : '—' }}</dd>
          </dl>
          <p v-else>
            {{
              target?.mode === 'auto' ? target.message || '正在获取公网 IP 与城市…' : '保存后将自动获取公网 IP 与城市。'
            }}
          </p>
          <p>
            仅返回城市时，使用本地城市点位表；无法匹配时不放置节点。NAT、VPN 或多出口环境可能与站点入口 IP
            不同，可改用实际部署坐标。
          </p>
        </div>
        <div v-if="targetForm.mode === 'coordinates'" class="coordinate-fields">
          <label
            >经度<input v-model="targetForm.longitude" required type="number" min="-180" max="180" step="any"
          /></label>
          <label
            >纬度<input v-model="targetForm.latitude" required type="number" min="-90" max="90" step="any"
          /></label>
        </div>
        <p v-if="targetError" class="control-error" role="alert">{{ targetError }}</p>
        <div class="control-footer">
          <button class="control-button primary" :disabled="saving">{{ saving ? '保存中…' : '保存节点' }}</button>
        </div>
      </form>
    </dialog>
    <dialog ref="shareDialog" class="control-dialog share-dialog" aria-labelledby="share-title" @click="closeBackdrop">
      <div class="control-heading">
        <h2 id="share-title">大屏只读分享</h2>
        <button aria-label="关闭分享管理" @click="shareDialog?.close()">×</button>
      </div>
      <p>
        持有链接的人无需登录。访问范围固定为全部站点的大屏汇总与最近 12
        条脱敏事件；IP、站点名和规则详情已隐藏，不能操作管理功能。
      </p>
      <form @submit.prevent="createShare">
        <div class="coordinate-fields">
          <label>分享名称<input v-model="shareLabel" maxlength="64" placeholder="值班大屏" /></label
          ><label
            >有效期<select v-model.number="shareHours" aria-label="有效期">
              <option :value="1">1 小时</option>
              <option :value="24">24 小时</option>
              <option :value="168">7 天</option>
            </select></label
          >
        </div>
        <button class="control-button primary" :disabled="creating">{{ creating ? '创建中…' : '创建只读链接' }}</button>
      </form>
      <div v-if="shareURL" class="new-share">
        <p v-if="localOrigin">当前链接使用本机地址。其他设备需要通过可访问的管理地址打开大屏后再创建分享链接。</p>
        <label
          >新链接（仅本次显示，请及时复制）<input
            ref="linkInput"
            :value="shareURL"
            readonly
            @focus="linkInput?.select()"
        /></label>
        <div class="control-links">
          <button class="control-button" @click="copyLink">{{ copied ? '已复制' : '复制链接' }}</button
          ><a :href="shareURL" target="_blank" rel="noopener noreferrer">预览只读大屏 ↗</a>
        </div>
      </div>
      <p v-if="shareError" class="control-error" role="alert">{{ shareError }}</p>
      <div class="share-list-title">
        <strong>分享记录</strong
        ><button class="control-button" :disabled="loading" @click="loadShares">刷新列表</button>
      </div>
      <div v-if="loading" class="control-empty">正在读取分享记录…</div>
      <div v-else-if="!shares.length" class="control-empty">暂无分享链接</div>
      <ul v-else class="share-records">
        <li v-for="item in shares" :key="item.id">
          <div>
            <strong>{{ item.label }}</strong
            ><span
              >{{ item.revoked_at ? '已撤销' : item.expires_at <= Date.now() ? '已过期' : '有效' }} · 到期
              {{ new Date(item.expires_at).toLocaleString() }}</span
            >
          </div>
          <button
            class="control-button"
            :disabled="Boolean(item.revoked_at) || item.expires_at <= Date.now() || revoking === item.id"
            @click="revokeShare(item.id)"
          >
            {{ revoking === item.id ? '撤销中…' : '撤销' }}
          </button>
        </li>
      </ul>
    </dialog>
  </div>
</template>
<script setup lang="ts">
import { reactive, ref, type PropType } from 'vue';
import { useRouter } from 'vue-router';
import request from '@/utils/request';
import type { ScreenTarget } from './data';
const props = defineProps({ target: { type: Object as PropType<ScreenTarget | null>, default: null } });
const emit = defineEmits(['updated']);
const router = useRouter();
const localOrigin = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
const targetDialog = ref<HTMLDialogElement>(),
  shareDialog = ref<HTMLDialogElement>(),
  linkInput = ref<HTMLInputElement>();
const targetForm = reactive({
  mode: 'auto',
  label: 'HomeWaf 防护节点',
  longitude: 0 as number | string,
  latitude: 0 as number | string,
});
const targetError = ref(''),
  shareError = ref('');
const saving = ref(false),
  creating = ref(false),
  loading = ref(false),
  copied = ref(false),
  revoking = ref('');
const shareLabel = ref('值班大屏'),
  shareHours = ref(24),
  shareURL = ref(''),
  generatedID = ref('');
interface Share {
  id: string;
  label: string;
  created_at: number;
  expires_at: number;
  revoked_at: number;
}
const shares = ref<Share[]>([]);
function closeBackdrop(event: MouseEvent) {
  const node = event.target as HTMLDialogElement;
  if (node === targetDialog.value || node === shareDialog.value) node.close();
}
function openTarget() {
  Object.assign(targetForm, {
    mode: props.target?.mode === 'coordinates' ? 'coordinates' : 'auto',
    label: props.target?.label || 'HomeWaf 防护节点',
    longitude: props.target?.longitude || 0,
    latitude: props.target?.latitude || 0,
  });
  targetError.value = '';
  targetDialog.value?.showModal();
}
async function saveTarget() {
  saving.value = true;
  targetError.value = '';
  try {
    const res = await request({
      url: 'dashboard/target',
      method: 'post',
      background: true,
      data: { ...targetForm, latitude: Number(targetForm.latitude), longitude: Number(targetForm.longitude) },
    });
    if (res.code !== 0) throw new Error(res.msg || '节点保存失败');
    emit('updated');
    targetDialog.value?.close();
  } catch (error) {
    targetError.value = error instanceof Error ? error.message : '节点保存失败';
  } finally {
    saving.value = false;
  }
}
function openShares() {
  shareError.value = '';
  shareDialog.value?.showModal();
  void loadShares();
}
async function loadShares() {
  loading.value = true;
  shareError.value = '';
  try {
    const res = await request({ url: 'dashboard/shares', method: 'get' });
    if (res.code !== 0 || !Array.isArray(res.data)) throw new Error(res.msg || '分享列表读取失败');
    shares.value = res.data;
  } catch (error) {
    shareError.value = error instanceof Error ? error.message : '分享列表读取失败';
  } finally {
    loading.value = false;
  }
}
async function createShare() {
  creating.value = true;
  shareError.value = '';
  copied.value = false;
  try {
    const res = await request({
      url: 'dashboard/shares',
      method: 'post',
      background: true,
      data: { label: shareLabel.value, hours: shareHours.value },
    });
    if (res.code !== 0 || !res.data?.token) throw new Error(res.msg || '创建分享失败');
    const href = router.resolve({ name: 'DashboardShared', query: { token: res.data.token } }).href;
    shareURL.value = new URL(href, location.href).href;
    generatedID.value = res.data.share.id;
    await loadShares();
  } catch (error) {
    shareError.value = error instanceof Error ? error.message : '创建分享失败';
  } finally {
    creating.value = false;
  }
}
async function revokeShare(id: string) {
  revoking.value = id;
  shareError.value = '';
  try {
    const res = await request({ url: 'dashboard/shares/revoke', method: 'post', background: true, data: { id } });
    if (res.code !== 0) throw new Error(res.msg || '撤销失败');
    if (generatedID.value === id) shareURL.value = '';
    await loadShares();
  } catch (error) {
    shareError.value = error instanceof Error ? error.message : '撤销失败';
  } finally {
    revoking.value = '';
  }
}
async function copyLink() {
  try {
    if (navigator.clipboard && window.isSecureContext) await navigator.clipboard.writeText(shareURL.value);
    else {
      linkInput.value?.focus();
      linkInput.value?.select();
      if (!document.execCommand('copy')) throw new Error('复制失败，请选中链接手动复制');
    }
    copied.value = true;
  } catch {
    linkInput.value?.focus();
    linkInput.value?.select();
    shareError.value = '请选中链接后手动复制。';
  }
}
</script>
<style scoped>
.screen-controls {
  display: flex;
  gap: 8px;
}
.control-button {
  border: 1px solid var(--screen-border);
  border-radius: 6px;
  background: transparent;
  color: var(--screen-muted);
  padding: 8px 11px;
  cursor: pointer;
  font-family: inherit;
  font-size: 13px;
  white-space: nowrap;
}
.control-button.primary {
  background: var(--screen-soft);
  color: var(--screen-accent);
  border-color: #aa96ed77;
}
.control-button:disabled {
  opacity: 0.5;
  cursor: default;
}
.control-dialog {
  border: 1px solid var(--screen-border);
  border-radius: var(--hw-r-lg);
  background: var(--screen-panel);
  color: var(--screen-text);
  width: min(580px, calc(100vw - 32px));
  max-height: 85dvh;
  overflow: auto;
  padding: 26px;
}
.control-dialog::backdrop {
  background: #10182788;
  backdrop-filter: blur(4px);
}
.control-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.control-heading h2 {
  font-size: 18px;
  margin: 0;
}
.control-heading > button {
  font-size: 24px;
  border: 0;
  background: none;
  color: var(--screen-muted);
  cursor: pointer;
}
.control-dialog p {
  color: var(--screen-muted);
  font-size: 13px;
  line-height: 1.8;
  margin: 16px 0;
}
.control-dialog label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  margin: 16px 0;
  flex: 1;
  min-width: 0;
}
.control-dialog input,
.control-dialog select {
  padding: 10px;
  border: 1px solid var(--screen-border);
  border-radius: 6px;
  background: var(--screen-bg);
  color: var(--screen-text);
  font: inherit;
  width: 100%;
  box-sizing: border-box;
}
.coordinate-fields {
  display: flex;
  gap: 16px;
}
.node-discovery {
  padding: 0 12px;
  border: 1px solid var(--screen-border);
  border-radius: 8px;
}
.node-discovery dl {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px 16px;
  font-size: 13px;
  line-height: 1.5;
}
.node-discovery dt {
  color: var(--screen-muted);
}
.node-discovery dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.control-footer {
  text-align: right;
  margin-top: 20px;
}
.control-error {
  color: var(--screen-danger) !important;
}
.new-share {
  margin-top: 18px;
  border-top: 1px solid var(--screen-border);
}
.control-links {
  display: flex;
  gap: 16px;
  align-items: center;
}
.control-links a {
  color: var(--screen-accent);
  font-size: 13px;
  text-decoration: none;
}
.share-list-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 24px;
  font-size: 13px;
}
.control-empty {
  padding: 24px;
  text-align: center;
  color: var(--screen-muted);
  font-size: 13px;
}
.share-records {
  padding: 0;
  list-style: none;
  max-height: 260px;
  overflow: auto;
}
.share-records li {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  padding: 14px 0;
  border-bottom: 1px solid var(--screen-border);
}
.share-records strong {
  display: block;
  font-size: 13px;
}
.share-records span {
  display: block;
  color: var(--screen-muted);
  font-size: 13px;
  margin-top: 7px;
}
.control-button:focus-visible,
.control-dialog input:focus-visible,
.control-dialog select:focus-visible {
  outline: 2px solid var(--screen-accent);
  outline-offset: 2px;
}
@media (max-width: 600px) {
  .coordinate-fields {
    flex-direction: column;
    gap: 0;
  }
  .control-dialog {
    padding: 20px;
  }
  .screen-controls {
    gap: 6px;
  }
}
</style>
