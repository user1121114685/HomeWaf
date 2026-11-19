<!-- HomeWaf modification (2026-11-17): certificate identity, renewal and recovery. -->
<template>
  <div class="certificate-page resource-panel">
    <RouterLink :to="back">返回证书管理</RouterLink>
    <p v-if="error" role="alert" class="certificate-error">{{ error }} <ElButton @click="load">重新读取</ElButton></p>
    <p v-if="loading && !record" role="status">正在读取证书…</p>
    <template v-if="record">
      <div class="certificate-toolbar">
        <div>
          <h2>{{ record.domains || record.subject }}</h2>
          <p>{{ validityStates[record.validity] }} · {{ updateMethods[record.update_method] }}</p>
        </div>
        <div class="certificate-actions">
          <ElButton v-if="canRenew" type="primary" :loading="renewing" :disabled="saving || !!error" @click="renew">{{
            active ? '查看进度' : record.renewal?.last_error ? '重试 / 处理失败' : '立即续期'
          }}</ElButton
          ><ElButton @click="editCertificate">编辑证书与文件设置</ElButton
          ><RouterLink :to="recordsLink">查看全部记录</RouterLink>
        </div>
      </div>
      <dl class="certificate-facts">
        <div>
          <dt>有效期</dt>
          <dd>{{ showTime(record.valid_from) }} → {{ showTime(record.valid_to) }}</dd>
        </div>
        <div>
          <dt>颁发者</dt>
          <dd>{{ record.issuer }}</dd>
        </div>
      </dl>
      <ElAlert v-if="record.renewal?.last_error" :title="renewalLabel(record)" type="error" :closable="false" show-icon
        ><p>{{ record.renewal.last_error }}</p>
        <p>
          下次尝试：{{ showTime(record.renewal.next_attempt_at) }}；连续失败 {{ record.renewal.failure_count }} 次。
        </p>
        <p v-if="record.renewal.status === 'blocked'">请先处理原因，再手动重试。保存设置不会立即申请证书。</p></ElAlert
      >
      <section class="certificate-section" id="renewal">
        <div class="certificate-toolbar">
          <h3>续期设置</h3>
          <ElButton v-if="record.renewal && !editing" @click="beginEdit">编辑续期设置</ElButton>
        </div>
        <template v-if="record.renewal">
          <p>{{ renewalLabel(record) }}</p>
          <p v-if="['queued', 'running', 'deploying', 'retry_wait', 'blocked'].includes(record.renewal.status)">
            {{ record.renewal.enabled ? '自动续期已开启' : '自动续期已关闭' }}
          </p>
          <RouterLink
            v-if="['manual', 'blocked'].includes(record.renewal.status)"
            :to="{ path: certificateRoot + '/request', query: { domains: record.domains, returnTo: route.fullPath } }"
            >重新配置申请（保留当前证书）</RouterLink
          >
          <dl v-if="!editing" class="certificate-facts">
            <div>
              <dt>签发机构与验证方式</dt>
              <dd>
                {{ record.renewal.apply_platform }} ·
                {{ record.renewal.apply_method === 'dns01' ? 'DNS 验证' : 'HTTP 文件验证' }}
              </dd>
            </div>
            <div>
              <dt>DNS 凭据分组</dt>
              <dd>{{ record.renewal.apply_dns || '不适用' }} / {{ record.renewal.private_group_name || '—' }}</dd>
            </div>
            <div>
              <dt>最近成功</dt>
              <dd>{{ showTime(record.renewal.last_success_at) }}</dd>
            </div>
            <div>
              <dt>下次失败重试</dt>
              <dd>{{ showTime(record.renewal.next_attempt_at) }}</dd>
            </div>
          </dl>
          <ElForm
            v-else
            class="certificate-policy-form"
            label-position="top"
            :disabled="saving"
            @submit.prevent="savePolicy"
          >
            <p class="certificate-muted">本次仅保存策略。关闭后不再创建自动任务，已开始的任务仍可能完成。</p>
            <ElFormItem label="自动续期"><ElCheckbox v-model="draft.enabled">启用自动续期</ElCheckbox></ElFormItem>
            <ElFormItem label="验证方式"
              ><ElSelect v-model="draft.apply_method"
                ><ElOption value="dns01" label="DNS 验证（无需开放入站端口）" /><ElOption
                  value="http01"
                  label="HTTP 文件验证（需要公网 HTTP 80）" /></ElSelect
            ></ElFormItem>
            <template v-if="draft.apply_method === 'dns01'"
              ><DnsProviderConfig
                v-model:provider="draft.apply_dns"
                v-model:group="draft.private_group_name"
                :revision="credentialRevision"
                @configure="openCredentials"
                @add-group="openCredentials"
              /><ElButton text @click="credentialRevision++">刷新凭据状态</ElButton
              ><ElFormItem label="本地 DNS 传播检查"
                ><ElRadioGroup v-model="draft.skip_dns_verify"
                  ><ElRadio :value="0">检查后验证</ElRadio><ElRadio :value="1">等待后直接验证</ElRadio></ElRadioGroup
                ></ElFormItem
              ></template
            >
            <ElFormItem label="联系邮箱"><ElInput v-model="draft.apply_email" type="email" /></ElFormItem>
            <p v-if="saveError" role="alert" class="certificate-error">{{ saveError }}；输入已保留。</p>
            <div class="certificate-form-actions">
              <ElButton native-type="submit" type="primary" :loading="saving">保存续期设置</ElButton
              ><ElButton @click="cancelEdit">取消</ElButton>
            </div>
          </ElForm>
          <p class="certificate-muted">
            网络故障按 15 分钟、1 小时、4 小时、之后每 6 小时重试（带少量错峰）；每天最多 6 次自动尝试。CA
            限流时间优先，凭据错误需要人工处理。
          </p>
          <RouterLink :to="certificateRoot + '/settings'">查看实例检查计划与默认策略</RouterLink>
        </template>
        <template v-else
          ><p>
            {{
              record.update_method === 'file'
                ? '由外部程序更新文件，HomeWaf 读取证书。文件同步不是向 CA 续期。'
                : '此证书没有已确认的自动申请来源。导入 PEM 不会自动获得续期能力。'
            }}
          </p>
          <ElButton
            @click="
              router.push({
                path: certificateRoot + '/request',
                query: { domains: record.domains, returnTo: route.fullPath },
              })
            "
            >配置自动申请</ElButton
          ></template
        >
      </section>
      <section class="certificate-section">
        <div class="certificate-toolbar">
          <h3>使用范围</h3>
          <ElButton @click="openBinding">关联服务</ElButton>
        </div>
        <p class="certificate-muted">已绑定表示配置引用；监听、HTTPS 可访问性和远端证书需要分别核实。</p>
        <p v-if="!record.bindings.length">当前没有服务绑定。</p>
        <div
          v-for="binding in record.bindings"
          :key="binding.kind + binding.id + binding.source"
          class="certificate-toolbar"
        >
          <div>
            <strong>{{ binding.name }}</strong
            ><small
              >{{ binding.protocol.toUpperCase() }} {{ binding.port ? ':' + binding.port : '' }} ·
              {{
                binding.kind === 'pipeline' ? (binding.listening ? '监听已启用' : '监听未启用') : '配置已绑定'
              }}</small
            >
          </div>
          <RouterLink
            v-if="binding.kind === 'pipeline'"
            :to="{
              path: '/waf-pipeline/ports',
              query: { edit: binding.port, tab: 'network', returnTo: route.fullPath },
            }"
            >配置服务 HTTPS</RouterLink
          ><RouterLink v-if="binding.kind === 'manager'" to="/sys/VpConfig">管理端访问设置</RouterLink>
        </div>
      </section>
      <section class="certificate-section">
        <h3>文件同步与导出</h3>
        <p>自动加载：{{ record.auto_load_path ? '已开启（还受实例全局开关控制）' : '已关闭' }}</p>
        <p v-if="record.export_status">{{ record.export_status }}</p>
        <details>
          <summary>查看路径与证书信息</summary>
          <dl class="certificate-facts">
            <div v-for="(label, key) in metadataFields" :key="key">
              <dt>{{ label }}</dt>
              <dd>{{ record[key] || '未配置' }}</dd>
            </div>
          </dl>
        </details>
      </section>
      <section class="certificate-section">
        <h3>通知与恢复记录</h3>
        <p v-if="notifyError" class="certificate-error" role="alert">
          {{ notifyError }} <ElButton text @click="loadNotifications">重试</ElButton>
        </p>
        <template v-if="notifications"
          ><p>
            续期与故障：{{
              notifications.operation_subscriptions
                ? notifications.operation_subscriptions + ' 个启用订阅'
                : '未配置启用的操作通知渠道'
            }}；到期提醒：{{
              notifications.expiry_subscriptions
                ? notifications.expiry_subscriptions + ' 个启用订阅'
                : '未配置启用渠道'
            }}。
          </p>
          <p class="certificate-muted">
            订阅仍受过滤、免打扰和频控影响；渠道接收不代表用户已读。仅开启到期提醒不能收到续期失败消息。
          </p>
          <RouterLink :to="{ path: '/notify/subscription', query: { returnTo: route.fullPath } }"
            >配置通知订阅</RouterLink
          >
          <div v-for="event in notifications.events" :key="event.id" class="certificate-section">
            <strong>{{ showTime(event.create_time) }} · {{ eventKinds[event.kind] || event.kind }}</strong>
            <p class="certificate-task-result">{{ event.message }}</p>
            <p v-if="event.coverage === 'no_subscription'" class="certificate-error">
              未找到对应订阅，未向外部渠道投递。
            </p>
            <p v-if="!event.expanded">通知事件已保存，等待分配渠道。</p>
            <div
              v-for="delivery in notifications.deliveries.filter((item: any) => item.event_id === event.id)"
              :key="delivery.id"
              class="certificate-toolbar"
            >
              <span
                >{{ delivery.channel_name || '渠道已删除' }} · {{ deliveryStates[delivery.status] || delivery.status
                }}<small v-if="delivery.error">{{ delivery.error }}</small
                ><small v-if="delivery.status === 'retry_wait'"
                  >下次投递：{{ showTime(delivery.next_attempt_at) }}</small
                ></span
              ><ElButton
                v-if="['failed', 'filtered', 'disabled'].includes(delivery.status)"
                :loading="sending === delivery.id"
                @click="retryDelivery(delivery.id)"
                >重新投递</ElButton
              >
            </div>
          </div>
        </template>
      </section>
      <ElDialog
        v-model="bindingVisible"
        title="关联服务"
        width="580px"
        :close-on-click-modal="false"
        :close-on-press-escape="!bindingBusy"
        :show-close="!bindingBusy"
        ><p>将此证书关联到所选服务，替换其默认 TLS 证书。不会改变 HTTP/HTTPS 协议或启用监听；请确认域名覆盖需要。</p>
        <ElForm label-position="top"
          ><ElFormItem label="选择服务"
            ><ElSelect v-model="bindingCode" :disabled="bindingBusy" filterable
              ><ElOption
                v-for="target in bindingTargets"
                :key="target.id"
                :value="target.code"
                :label="target.name + ' :' + target.port" /></ElSelect></ElFormItem
        ></ElForm>
        <p v-if="bindingError" role="alert" class="certificate-error">{{ bindingError }}</p>
        <template #footer
          ><ElButton :disabled="bindingBusy" @click="bindingVisible = false">取消</ElButton
          ><ElButton
            type="primary"
            :loading="bindingBusy"
            :disabled="!bindingCode || !bindingTargets.length"
            @click="saveBinding"
            >确认关联</ElButton
          ></template
        ></ElDialog
      >
      <CertificateEditor ref="editor" @saved="onSaved" />
      <OrderLogs v-model:visible="logsVisible" :order-id="orderId" :retrying="renewing" @retry="renew" />
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router';
import {
  ElAlert,
  ElDialog,
  ElButton,
  ElCheckbox,
  ElForm,
  ElFormItem,
  ElInput,
  ElSelect,
  ElOption,
  ElRadio,
  ElRadioGroup,
  ElMessage,
  ElMessageBox,
} from 'element-plus';
import { v4 as uuid } from 'uuid';
import {
  certificateDetailApi,
  certificateBindingTargetsApi,
  certificateBindApi,
  certificatePolicyApi,
  certificateRenewApi,
  certificateNotificationsApi,
  certificateNotificationRetryApi,
  certificateRoot,
  showTime,
  validityStates,
  updateMethods,
  renewalLabel,
  safeCertificateReturn,
  type CertificateRecord,
} from '@/apis/certificates';
import { sslConfigDetailApi } from '@/apis/sslconfig';
import CertificateEditor from './CertificateEditor.vue';
import OrderLogs from '../sslorder/OrderLogs.vue';
import DnsProviderConfig from '../sslorder/DnsProviderConfig.vue';
import './certificates.css';
const route = useRoute(),
  router = useRouter();
const record = ref<CertificateRecord>(),
  loading = ref(false),
  error = ref(''),
  editing = ref(false),
  saving = ref(false),
  renewing = ref(false),
  saveError = ref('');
const draft = ref<Record<string, any>>({}),
  snapshot = ref(''),
  credentialRevision = ref(0);
const notifications = ref<any>(),
  notifyError = ref(''),
  sending = ref('');
const bindingVisible = ref(false),
  bindingBusy = ref(false),
  bindingError = ref(''),
  bindingCode = ref(''),
  bindingTargets = ref<any[]>([]);
const logsVisible = ref(false),
  orderId = ref(''),
  editor = ref<InstanceType<typeof CertificateEditor>>();
const id = computed(() => String(route.params.certificateId));
const back = computed(() => safeCertificateReturn(route.query.returnTo));
const recordsLink = computed(() => ({
  path: '/waf-host/wafsslOrderlist',
  query: { certificate_id: id.value, returnTo: route.fullPath },
}));
const active = computed(() => ['queued', 'running', 'deploying'].includes(record.value?.renewal?.status));
const canRenew = computed(
  () => record.value?.renewal && record.value.renewal.status !== 'manual',
);
const dirty = computed(() => editing.value && JSON.stringify(draft.value) !== snapshot.value);
const metadataFields = {
  cert_path: '证书读取路径',
  key_path: '私钥读取路径',
  export_cert_path: '证书导出路径',
  export_key_path: '私钥导出路径',
  serial_no: '序列号',
  id: '记录 ID',
};
const eventKinds: Record<string, string> = {
  failure: '处理失败',
  blocked: '需要处理',
  interrupted: '任务中断',
  recovered: '已恢复',
  success: '更新完成',
  expiry: '到期风险',
};
const deliveryStates: Record<string, string> = {
  pending: '等待投递',
  sending: '正在投递',
  retry_wait: '发送失败，等待重试',
  sent: '渠道已接收',
  failed: '发送失败',
  disabled: '订阅或渠道未启用',
  filtered: '被通知规则过滤',
};
let sequence = 0,
  requestId = '',
  timer: ReturnType<typeof setInterval> | undefined;
async function load() {
  const current = ++sequence;
  loading.value = true;
  try {
    const res = await certificateDetailApi(id.value);
    if (current !== sequence) return;
    if (res.code !== 0) throw new Error(res.msg);
    record.value = res.data;
    error.value = '';
  } catch (e) {
    if (current === sequence) error.value = e instanceof Error ? e.message : '证书读取失败';
  } finally {
    if (current === sequence) loading.value = false;
  }
}
async function loadNotifications() {
  try {
    const currentId = id.value;
    const res = await certificateNotificationsApi(currentId);
    if (currentId !== id.value) return;
    if (res.code !== 0) throw new Error(res.msg);
    notifications.value = res.data;
    notifyError.value = '';
  } catch {
    notifyError.value = '通知状态读取失败，不能据此判断是否已经投递。';
  }
}
function beginEdit() {
  const p = record.value?.renewal;
  if (!p) return;
  draft.value = {
    id: id.value,
    version: p.version,
    enabled: p.enabled,
    apply_method: p.apply_method,
    apply_dns: p.apply_dns,
    apply_email: p.apply_email,
    private_group_name: p.private_group_name,
    skip_dns_verify: p.skip_dns_verify,
  };
  snapshot.value = JSON.stringify(draft.value);
  editing.value = true;
  saveError.value = '';
}
async function discard() {
  if (saving.value || renewing.value) return false;
  if (!dirty.value) return true;
  try {
    await ElMessageBox.confirm('离开会放弃尚未保存的续期设置。', '未保存的修改', {
      confirmButtonText: '放弃修改',
      cancelButtonText: '继续编辑',
      closeOnHashChange: false,
    });
    return true;
  } catch {
    return false;
  }
}
async function cancelEdit() {
  if (await discard()) {
    editing.value = false;
    draft.value = {};
  }
}
async function savePolicy() {
  if (saving.value) return;
  saving.value = true;
  saveError.value = '';
  try {
    const res = await certificatePolicyApi(draft.value);
    if (res.code !== 0) throw new Error(res.msg);
    record.value!.renewal = res.data;
    editing.value = false;
    saving.value = false;
    requestId = '';
    const query = { ...route.query };
    delete query.edit;
    await router.replace({ query });
    ElMessage.success(res.msg);
    await load();
  } catch (e) {
    saveError.value = e instanceof Error ? e.message : '保存失败';
  } finally {
    saving.value = false;
  }
}
async function renew() {
  if (renewing.value || !record.value) return;
  if (editing.value) {
    ElMessage.warning('请先保存或取消续期设置，再发起任务');
    return;
  }
  if (active.value) {
    orderId.value = record.value.renewal.last_order_id;
    logsVisible.value = true;
    return;
  }
  try {
    await ElMessageBox.confirm(
      '将按已保存策略处理 ' +
        record.value.domains +
        '。续期成功后更新当前绑定服务；已有签发材料时只重试未完成的更新步骤。',
      '确认续期 / 恢复',
      { confirmButtonText: '开始处理', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  renewing.value = true;
  requestId ||= uuid();
  try {
    const res = await certificateRenewApi(id.value, requestId);
    if (res.code !== 0) throw new Error(res.msg);
    requestId = '';
    orderId.value = res.data.id;
    logsVisible.value = true;
    await load();
  } catch (e) {
    ElMessage.error((e instanceof Error ? e.message : '提交结果未确认') + '；可重试同一提交');
  } finally {
    renewing.value = false;
  }
}
function openCredentials() {
  const link = router.resolve({
    path: certificateRoot + '/settings',
    query: { provider: draft.value.apply_dns, group: draft.value.private_group_name },
  });
  window.open(link.href, '_blank', 'noopener');
}
async function editCertificate() {
  try {
    const res = await sslConfigDetailApi({ id: id.value });
    if (res.code !== 0) throw new Error(res.msg);
    editor.value?.open(res.data);
  } catch {
    ElMessage.error('编辑内容读取失败，请检查权限后重试');
  }
}
function onSaved(result: { message: string; exportFailed: boolean }) {
  ElMessage[result.exportFailed ? 'warning' : 'success'](result.message);
  void load();
}
async function retryDelivery(deliveryId: string) {
  if (sending.value) return;
  sending.value = deliveryId;
  try {
    const res = await certificateNotificationRetryApi(deliveryId);
    if (res.code !== 0) throw new Error(res.msg);
    await loadNotifications();
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '通知重试失败');
  } finally {
    sending.value = '';
  }
}
async function openBinding() {
  bindingVisible.value = true;
  bindingCode.value = '';
  bindingError.value = '';
  bindingTargets.value = [];
  try {
    const res = await certificateBindingTargetsApi();
    if (res.code !== 0) throw new Error(res.msg);
    bindingTargets.value = res.data || [];
  } catch {
    bindingError.value = '服务状态读取失败，请关闭后重新打开。';
  }
}
async function saveBinding() {
  if (bindingBusy.value) return;
  const target = bindingTargets.value.find((item) => item.code === bindingCode.value);
  if (!target) return;
  bindingBusy.value = true;
  try {
    const res = await certificateBindApi({
      id: id.value,
      host_code: target.code,
      target_id: target.id,
      fingerprint: target.fingerprint,
    });
    if (res.code !== 0) throw new Error(res.msg);
    bindingVisible.value = false;
    ElMessage.success(res.msg);
    await load();
  } catch (e) {
    bindingError.value = e instanceof Error ? e.message : '关联失败，请重新读取服务状态';
  } finally {
    bindingBusy.value = false;
  }
}
function unload(event: BeforeUnloadEvent) {
  if (dirty.value || saving.value) {
    event.preventDefault();
    event.returnValue = '';
  }
}
onBeforeRouteLeave(discard);
onBeforeRouteUpdate(discard);
watch(id, async () => {
  record.value = undefined;
  notifications.value = undefined;
  editing.value = false;
  await load();
  await loadNotifications();
});
onMounted(async () => {
  window.addEventListener('beforeunload', unload);
  await load();
  await loadNotifications();
  if (route.query.edit === '1') beginEdit();
  if (route.query.action === 'renew' && canRenew.value) await renew();
  timer = setInterval(() => {
    if (!editing.value && active.value) void load();
    if (!document.hidden) void loadNotifications();
  }, 5000);
});
onBeforeUnmount(() => {
  sequence++;
  clearInterval(timer);
  window.removeEventListener('beforeunload', unload);
  draft.value = {};
});
</script>
