<template>
  <section class="service-identity-panel" :aria-busy="busy || loading">
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <template v-if="mode === 'all' || mode === 'accounts'">
      <div class="identity-heading">
        <div>
          <h3>本端口账号</h3>
          <p>账号只属于此服务。保存或删除后立即生效，旧会话会被撤销。</p>
        </div>
        <div>
          <ElButton :disabled="busy" @click="newAccount">添加账号</ElButton
          ><ElButton :loading="loading" :disabled="busy" @click="load">刷新</ElButton>
        </div>
      </div>
      <ElTable
        :data="accounts"
        :empty-text="loading ? '正在读取账号…' : error ? '账号读取失败，请重试' : '还没有访问账号'"
      >
        <ElTableColumn prop="username" label="账号" min-width="160" />
        <ElTableColumn label="状态" min-width="100"
          ><template #default="{ row }"
            ><ElTag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</ElTag></template
          ></ElTableColumn
        >
        <ElTableColumn label="到期时间" min-width="170"
          ><template #default="{ row }">{{ timeText(row.expires_at) }}</template></ElTableColumn
        >
        <ElTableColumn prop="remark" label="备注" min-width="140" />
        <ElTableColumn label="操作" min-width="180"
          ><template #default="{ row }"
            ><ElButton link :disabled="busy" @click="editAccount(row)">修改 / 重设口令</ElButton
            ><ElButton link type="danger" :disabled="busy" @click="deleteAccount(row)">删除</ElButton></template
          ></ElTableColumn
        >
      </ElTable>
    </template>
    <template v-if="mode === 'all' || mode === 'sessions'">
      <div class="identity-heading">
        <div>
          <h3>活动会话</h3>
          <p>核对账号与来源后撤销，操作立即生效。</p>
        </div>
        <ElButton :loading="loading" :disabled="busy" @click="load">刷新会话</ElButton>
      </div>
      <div class="identity-actions session-actions">
        <label>账号范围<ElSelect v-model="sessionUser" clearable :disabled="busy || loading" placeholder="当前服务全部账号" aria-label="会话账号范围"><ElOption v-for="username in sessionUsers" :key="username" :value="username" :label="username" /></ElSelect></label>
        <ElButton type="danger" plain :disabled="busy || loading || !!error || !visibleSessions.length" @click="revokeSessions">{{ sessionUser ? '撤销该账号全部会话' : '撤销此服务全部会话' }}</ElButton>
      </div>
      <ElTable
        :data="visibleSessions"
        :empty-text="loading ? '正在读取会话…' : error ? '会话读取失败，请重试' : '当前没有活动会话'"
      >
        <ElTableColumn prop="username" label="账号" min-width="160" /><ElTableColumn
          prop="client_ip"
          label="来源 IP"
          min-width="180"
        />
        <ElTableColumn label="最近活动" min-width="180"><template #default="{ row }">{{ timeText(row.last_seen) }}</template></ElTableColumn>
        <ElTableColumn label="到期时间" min-width="180"
          ><template #default="{ row }">{{ timeText(row.expires_at) }}</template></ElTableColumn
        >
        <ElTableColumn label="操作" width="120"
          ><template #default="{ row }"
            ><ElButton link type="danger" :disabled="busy" @click="kick(row)">立即踢出</ElButton></template
          ></ElTableColumn
        >
      </ElTable>
    </template>
    <section v-if="mode === 'all' || mode === 'token'" class="token-section">
      <div class="identity-heading">
        <div>
          <h3>服务令牌</h3>
          <p>轮换或撤销立即使旧令牌失效，新令牌仅显示一次。</p>
        </div>
      </div>
      <div class="identity-actions">
        <ElButton :disabled="busy" @click="rotateToken(false)">生成 / 轮换服务令牌</ElButton
        ><ElButton type="danger" plain :disabled="busy" @click="rotateToken(true)">撤销服务令牌</ElButton>
      </div>
      <ElAlert
        v-if="serviceToken"
        title="令牌只显示这一次；离开此处即清除。旧令牌已失效。"
        type="warning"
        :closable="false"
      />
      <ElInput v-if="serviceToken" :model-value="serviceToken" readonly aria-label="一次性服务令牌" />
    </section>
    <ElDialog
      v-model="accountVisible"
      title="端口账号"
      width="min(620px,94vw)"
      append-to-body
      destroy-on-close
      :close-on-click-modal="false"
      :close-on-press-escape="!busy"
      :show-close="!busy"
    >
      <div class="account-form">
        <label
          >用户名<ElInput v-model="account.username" :disabled="editingAccount || busy" aria-label="端口账号用户名"
        /></label>
        <label
          >{{ editingAccount ? '新口令（留空保持）' : '口令'
          }}<ElInput
            v-model="account.password"
            type="password"
            show-password
            autocomplete="new-password"
            aria-label="端口账号口令"
            :disabled="busy"
        /></label>
        <label>备注<ElInput v-model="account.remark" :disabled="busy" aria-label="账号备注" /></label>
        <div class="account-state">
          <ElSwitch v-model="account.enabled" active-text="账号启用" :disabled="busy" /><label
            >到期时间（留空为不限）<input
              type="datetime-local"
              :value="localDate(account.expires_at)"
              :disabled="busy"
              @change="account.expires_at = epoch(($event.target as HTMLInputElement).value)"
          /></label>
        </div>
        <details class="account-security" :open="account.totp_configured">
          <summary>
            二次验证 <span>{{ account.totp_configured ? '已设置' : '未设置' }}</span>
          </summary>
          <ElButton
            :disabled="busy"
            @click="
              account.totp_secret = newTOTPSecret();
              account.clear_totp = false;
            "
            >生成新的动态口令密钥</ElButton
          >
          <label
            >动态口令Base32密钥（可选，留空保留）<ElInput
              v-model="account.totp_secret"
              type="password"
              show-password
              autocomplete="new-password"
              aria-label="动态口令密钥"
              :disabled="busy"
          /></label>
          <small>仅表单模式支持；密钥加密保存且不回显。使用身份验证器的六位动态码登录。</small>
          <ElCheckbox v-if="account.totp_configured" v-model="account.clear_totp" :disabled="busy"
            >明确移除该账号的二次验证</ElCheckbox
          >
        </details>
        <p class="muted">不能删除或停用已开启登录门的最后一个有效账号。</p>
        <ElAlert v-if="accountError" :title="accountError" type="error" :closable="false" />
      </div>
      <template #footer
        ><ElButton :disabled="busy" @click="accountVisible = false">取消</ElButton
        ><ElButton type="primary" :loading="busy" @click="saveAccount">立即保存账号</ElButton></template
      >
    </ElDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch, type PropType } from 'vue';
import {
  ElAlert,
  ElButton,
  ElCheckbox,
  ElDialog,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElSelect,
  ElOption,
  ElSwitch,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import { newTOTPSecret } from './model';
const props = defineProps({
  port: { type: Number, required: true },
  revision: { type: Number, required: true },
  mode: { type: String as PropType<'all' | 'accounts' | 'sessions' | 'token'>, default: 'all' },
});
const emit = defineEmits(['busy', 'revision', 'editing']);
const accounts = ref<any[]>([]),
  sessions = ref<any[]>([]),
  loading = ref(false),
  busy = ref(false),
  error = ref(''),
  accountError = ref(''),
  serviceToken = ref('');
const accountVisible = ref(false),
  editingAccount = ref(false);
const sessionUser = ref('');
const sessionUsers = computed(() => [...new Set(sessions.value.map(row => row.username))]);
const visibleSessions = computed(() => sessions.value.filter(row => !sessionUser.value || row.username === sessionUser.value));
const account = reactive({
  username: '',
  password: '',
  totp_secret: '',
  clear_totp: false,
  totp_configured: false,
  enabled: true,
  expires_at: 0,
  remark: '',
});
const timeText = (value: number) => (value ? new Date(value * 1000).toLocaleString() : '不设到期');
const localDate = (value: number) => {
  if (!value) return '';
  const date = new Date(value * 1000);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
};
const epoch = (value: string) => (value ? Math.floor(new Date(value).getTime() / 1000) : 0);
async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = '';
  try {
    const [users, active] = await Promise.all([
      ['all', 'accounts'].includes(props.mode)
        ? pipelineGet<any[]>('gate/users', { port: props.port })
        : Promise.resolve([]),
      ['all', 'sessions', 'accounts'].includes(props.mode)
        ? pipelineGet<any[]>('gate/sessions', { port: props.port })
        : Promise.resolve([]),
    ]);
    accounts.value = users;
    sessions.value = active;
  } catch (cause: any) {
    error.value = cause.message || '读取失败，请重试。';
  } finally {
    loading.value = false;
  }
}
async function run(action: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  accountError.value = '';
  try {
    await action();
  } catch (cause: any) {
    if (cause !== 'cancel' && cause !== 'close') {
      const text = cause.message || '操作失败，请重试。';
      if (accountVisible.value) accountError.value = text;
      else error.value = text;
    }
  } finally {
    busy.value = false;
  }
}
function newAccount() {
  editingAccount.value = false;
  accountError.value = '';
  Object.assign(account, {
    username: '',
    password: '',
    totp_secret: '',
    clear_totp: false,
    totp_configured: false,
    enabled: true,
    expires_at: 0,
    remark: '',
  });
  accountVisible.value = true;
}
function editAccount(row: any) {
  newAccount();
  editingAccount.value = true;
  Object.assign(account, {
    username: row.username,
    enabled: row.enabled,
    expires_at: row.expires_at,
    remark: row.remark,
    totp_configured: !!row.totp_configured,
  });
}
async function saveAccount() {
  await run(async () => {
    if (account.clear_totp)
      await ElMessageBox.confirm('移除后该账号只需密码即可登录；旧会话将撤销。', '确认移除二次验证', {
        type: 'warning',
      });
    const { totp_configured, ...values } = account;
    await pipelinePost('gate/users/save', { port: props.port, ...values });
    account.password = '';
    account.totp_secret = '';
    accountVisible.value = false;
    await load();
    ElMessage.success('账号已保存，旧会话已撤销');
  });
}
async function deleteAccount(row: any) {
  await run(async () => {
    await ElMessageBox.confirm('删除 ' + row.username + ' 并撤销其会话？', '删除账号', { type: 'warning' });
    await pipelinePost('gate/users/delete', { port: props.port, username: row.username });
    await load();
  });
}
async function kick(row: any) {
  await run(async () => {
    await ElMessageBox.confirm('立即撤销该会话？', '踢出会话', { type: 'warning' });
    await pipelinePost('gate/sessions/kick', { port: props.port, id: row.id });
    await load();
  });
}
async function revokeSessions() {
  await run(async () => {
    const username = sessionUser.value;
    await ElMessageBox.confirm(
      `撤销端口 ${props.port} 的${username ? '账号「' + username + '」' : '全部账号'}在线会话？当前显示 ${visibleSessions.value.length} 个，会话持有人需要重新登录；其他服务不受影响。`,
      '确认批量撤销会话', { type: 'warning', confirmButtonText: '立即撤销', cancelButtonText: '取消' },
    );
    const result = await pipelinePost('gate/sessions/revoke', { port: props.port, scope: username ? 'account' : 'service', username });
    await load();
    ElMessage.success(`已撤销 ${result.revoked} 个会话`);
  });
}
async function rotateToken(revoke: boolean) {
  await run(async () => {
    await ElMessageBox.confirm(
      revoke ? '撤销后，使用旧服务令牌的客户端将无法登录。' : '轮换后旧令牌立即失效，新令牌只显示一次。',
      '服务令牌',
      { type: 'warning' },
    );
    const result = await pipelinePost('gate/token/rotate', { port: props.port, revoke, revision: props.revision });
    emit('revision', result.revision);
    serviceToken.value = result.token || '';
    ElMessage.success(revoke ? '服务令牌已撤销' : '服务令牌已轮换');
  });
}
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
watch(accountVisible, (value) => {
  emit('editing', value);
  if (!value) {
    account.password = '';
    account.totp_secret = '';
  }
});
onMounted(() => {
  if (props.mode !== 'token') load();
});
</script>
<style scoped>
.service-identity-panel {
  min-width: 0;
}
.identity-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
  margin: 20px 0 16px;
}
.identity-heading:first-child {
  margin-top: 0;
}
.session-actions { margin-bottom: 16px; align-items: end; }
.session-actions label { display: grid; gap: 6px; width: min(100%, 260px); }
.identity-heading > div:last-child,
.identity-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.identity-heading > div:first-child {
  display: block;
}
h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
p,
small {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--hw-muted);
  line-height: 1.7;
}
.token-section {
  margin-top: 24px;
}
.token-section > .el-alert,
.token-section > .el-input {
  margin-top: 12px;
}
.account-form,
.account-form label {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.account-form {
  gap: 20px;
}
.account-state {
  display: grid;
  grid-template-columns: 1fr 2fr;
  align-items: center;
  gap: 16px;
}
.account-state input {
  padding: 9px 12px;
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  background: var(--hw-panel);
  color: var(--hw-ink);
  min-width: 0;
}
.account-security {
  padding-top: 16px;
  border-top: 1px solid var(--hw-line);
}
.account-security summary {
  cursor: pointer;
  font-weight: 500;
  margin-bottom: 16px;
}
.account-security summary span {
  font-size: 12px;
  margin-left: 12px;
  color: var(--hw-muted);
}
.account-security > label {
  margin-top: 16px;
}
@media (max-width: 520px) {
  .account-state {
    grid-template-columns: 1fr;
  }
}
</style>
