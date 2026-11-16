<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div class="channel-header-wrapper">
    <strong class="channel-provider-name">{{ label }}</strong>
    <hw-dropdown trigger="click" :min-column-width="240">
      <hw-button variant="text" size="small" :aria-label="label + '订阅批量操作'"
        >批量操作 <hw-icon name="chevron-down" />
      </hw-button>
      <template #dropdown
        ><hw-dropdown-menu>
          <hw-dropdown-item disabled>仅影响 {{ label }} 渠道的全部消息类型</hw-dropdown-item>
          <hw-dropdown-item @click="$emit('batch', { action: 'add', type: type })">
            <hw-icon name="add" /> 订阅全部消息类型
          </hw-dropdown-item>
          <hw-dropdown-item @click="$emit('batch', { action: 'enable', type: type })">
            <hw-icon name="check-circle" /> 启用已有订阅
          </hw-dropdown-item>
          <hw-dropdown-item @click="$emit('batch', { action: 'disable', type: type })">
            <hw-icon name="close-circle" /> 暂停已有订阅
          </hw-dropdown-item>
          <hw-dropdown-item @click="$emit('batch', { action: 'config', type: type })">
            <hw-icon name="setting" /> 批量设置通知频率
          </hw-dropdown-item>
          <hw-dropdown-item @click="$emit('batch', { action: 'delete', type: type })">
            <hw-icon name="delete" /> 删除全部订阅
          </hw-dropdown-item>
        </hw-dropdown-menu></template
      >
    </hw-dropdown>
  </div>
</template>

<script lang="ts">
import { defineComponent } from 'vue';

// 渠道列表头：原来 5 个渠道各复制一份同样的 25 行模板，
// 加一个"批量配置"菜单项就要改 5 处，抽出来后只改一处。
export default defineComponent({
  name: 'NotifySubscriptionChannelHeader',
  props: {
    type: { type: String, required: true },
    label: { type: String, required: true },
    theme: { type: String, default: 'primary' },
  },
});
</script>

<style lang="less" scoped>
.channel-header-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.channel-provider-name {
  color: var(--hw-ink);
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
}
</style>
