<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="channel-header-wrapper">
    <hw-tag :theme="theme" size="small">{{ label }}</hw-tag>
    <hw-dropdown :min-column-width="140">
      <hw-button variant="text" size="small" shape="square" :aria-label="`${label}批量操作`">
        <hw-icon name="ellipsis" />
      </hw-button>
      <hw-dropdown-menu>
        <hw-dropdown-item @click="$emit('batch', { action: 'add', type: type })">
          <hw-icon name="add" /> {{ $t('page.notify_subscription.batch_action_add') }}
        </hw-dropdown-item>
        <hw-dropdown-item @click="$emit('batch', { action: 'enable', type: type })">
          <hw-icon name="check-circle" /> {{ $t('page.notify_subscription.batch_action_enable') }}
        </hw-dropdown-item>
        <hw-dropdown-item @click="$emit('batch', { action: 'disable', type: type })">
          <hw-icon name="close-circle" /> {{ $t('page.notify_subscription.batch_action_disable') }}
        </hw-dropdown-item>
        <hw-dropdown-item @click="$emit('batch', { action: 'config', type: type })">
          <hw-icon name="setting" /> {{ $t('page.notify_subscription.batch_action_config') }}
        </hw-dropdown-item>
        <hw-dropdown-item @click="$emit('batch', { action: 'delete', type: type })">
          <hw-icon name="delete" /> {{ $t('page.notify_subscription.batch_action_delete') }}
        </hw-dropdown-item>
      </hw-dropdown-menu>
    </hw-dropdown>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';



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
</style>
