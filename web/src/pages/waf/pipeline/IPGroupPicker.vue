<template>
  <div class="group-picker">
    <label :for="`${purpose}-service-groups`">{{ title }}</label>
    <ElSelectV2
      :id="`${purpose}-service-groups`"
      :model-value="modelValue"
      :options="options"
      multiple
      filterable
      clearable
      :loading="loading"
      :aria-label="title"
      placeholder="选择已有 IP 组"
      @update:model-value="emit('update:modelValue', $event)"
    />
    <p v-if="loadError" class="error" role="alert">{{ loadError }}</p>
    <div class="group-tools">
      <ElButton :aria-expanded="creating" @click="creating = !creating">{{
        creating ? '收起新建表单' : '新建 IP 组'
      }}</ElButton>
      <ElButton :loading="loading" @click="emit('refresh')">刷新 IP 组</ElButton>
      <RouterLink to="/waf-host/wafipgroup" target="_blank" rel="noopener noreferrer">管理 IP 组</RouterLink>
    </div>
    <p>修改组内地址后，所有引用它的服务立即同步。支持 IPv4 和 IPv6。</p>
    <div v-if="creating" class="create-group" :aria-busy="saving">
      <h4>新建{{ purpose === 'allow' ? '白名单' : '黑名单' }} IP 组</h4>
      <label :for="`${purpose}-group-name`">组名称</label>
      <ElInput
        :id="`${purpose}-group-name`"
        v-model="name"
        maxlength="100"
        :disabled="saving"
        aria-label="新IP组名称"
        placeholder="例如家庭内网、办公室出口"
      />
      <label :for="`${purpose}-group-addresses`">组内地址</label>
      <ElInput
        :id="`${purpose}-group-addresses`"
        v-model="addresses"
        type="textarea"
        :rows="3"
        :disabled="saving"
        aria-label="新IP组地址"
        placeholder="每行一项；支持 IP、CIDR、通配符和地址区间"
      />
      <div class="group-tools">
        <ElButton v-if="purpose === 'allow'" :disabled="saving" @click="usePrivateNetworks">填入常见内网地址</ElButton>
        <ElButton type="primary" :loading="saving" @click="createGroup">创建并选用 IP 组</ElButton>
      </div>
      <p>IP 组会立即保存到统一列表；本服务的引用在保存服务配置后生效。</p>
      <p v-if="createError" class="error" role="alert">{{ createError }}</p>
    </div>
    <p v-if="createdMessage" role="status">{{ createdMessage }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import type { PropType } from 'vue';
import { ElButton, ElInput, ElSelectV2 } from 'element-plus';
import { wafIPGroupAddApi } from '@/apis/ipgroup';
const props = defineProps({
  modelValue: { type: Array as PropType<string[]>, required: true },
  purpose: { type: String as PropType<'allow' | 'deny'>, required: true },
  title: { type: String, required: true },
  groups: {
    type: Array as PropType<Array<{ group_code: string; group_name: string; item_count: number }>>,
    required: true,
  },
  loading: Boolean,
  loadError: String,
});
const emit = defineEmits(['update:modelValue', 'refresh', 'created', 'busy']);
const creating = ref(false),
  saving = ref(false),
  name = ref(''),
  addresses = ref(''),
  createError = ref(''),
  createdMessage = ref('');
const options = computed(() => {
  const known = new Set(props.groups.map((group) => group.group_code));
  return [
    ...props.groups.map((group) => ({
      value: group.group_code,
      label: `${group.group_name} · ${group.item_count} 条地址`,
    })),
    ...props.modelValue
      .filter((code) => !known.has(code))
      .map((code) => ({
        value: code,
        label: props.loading || props.loadError ? '已选 IP 组（待加载）' : 'IP 组已不存在，请重新选择',
      })),
  ];
});
function usePrivateNetworks() {
  if (!name.value.trim()) name.value = '家庭内网';
  addresses.value = '10.0.0.0/8\n172.16.0.0/12\n192.168.0.0/16\nfc00::/7';
}
async function createGroup() {
  if (saving.value) return;
  createError.value = '';
  createdMessage.value = '';
  const ips = [
    ...new Set(
      addresses.value
        .split(/\r?\n/)
        .map((value) => value.trim())
        .filter(Boolean),
    ),
  ];
  if (!name.value.trim() || !ips.length) {
    createError.value = '填写组名称和至少一条地址，或使用常见内网地址。';
    return;
  }
  saving.value = true;
  emit('busy', true);
  try {
    const result: any = await wafIPGroupAddApi({ group_name: name.value.trim(), initial_ips: ips });
    if (result.code !== 0 || !result.data?.group_code) throw new Error(result.msg || 'IP 组创建失败，请重试。');
    emit('created', result.data);
    emit('update:modelValue', [...new Set([...props.modelValue, result.data.group_code])]);
    creating.value = false;
    createdMessage.value = `已创建并选用“${result.data.group_name}”。`;
    name.value = '';
    addresses.value = '';
  } catch (cause: any) {
    createError.value = cause.message || 'IP 组创建失败，请重试。';
  } finally {
    saving.value = false;
    emit('busy', false);
  }
}
</script>

<style scoped>
.group-picker,
.create-group {
  display: grid;
  gap: 12px;
  min-width: 0;
}
.create-group {
  border-left: 2px solid var(--el-border-color);
  padding: 4px 0 4px 16px;
}
h4,
p {
  margin: 0;
}
h4 {
  font-size: 14px;
  font-weight: 500;
}
p {
  font-size: 13px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
}
label {
  font-size: 14px;
}
.error {
  color: var(--el-color-danger);
}
.group-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  align-items: center;
}
.group-tools :deep(.el-button) {
  margin-left: 0;
  min-height: 44px;
}
.group-tools a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--el-color-primary);
}
:deep(.el-select) {
  width: 100%;
}
:deep(.el-select__wrapper),
:deep(.el-input__wrapper) {
  min-height: 44px;
}
@media (max-width: 600px) {
  .group-tools :deep(.el-button) {
    flex: 1 1 auto;
  }
  :deep(input),
  :deep(textarea) {
    font-size: 16px;
  }
}
</style>
