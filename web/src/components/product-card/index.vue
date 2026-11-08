<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-card theme="poster2">
    <template #avatar>
      <hw-avatar size="56px">
        <template #icon>
          <shop-icon v-if="product.type === 1" />
          <calendar-icon v-if="product.type === 2" />
          <service-icon v-if="product.type === 3" />
          <user-avatar-icon v-if="product.type === 4" />
          <laptop-icon v-if="product.type === 5" />
        </template>
      </hw-avatar>
    </template>
    <template #status>
      <hw-tag :theme="product.isSetup ? 'success' : 'default'" :disabled="!product.isSetup">{{
        product.isSetup ? '已启用' : '已停用'
      }}</hw-tag>
    </template>
    <template #content>
      <p class="list-card-item_detail--name">{{ product.name }}</p>
      <p class="list-card-item_detail--desc">{{ product.description }}</p>
    </template>
    <template #footer>
      <hw-avatar-group cascading="left-up" :max="2">
        <hw-avatar>{{ typeMap[product.type - 1] }}</hw-avatar>
        <hw-avatar
          ><template #icon>
            <add-icon />
          </template>
        </hw-avatar>
      </hw-avatar-group>
    </template>
    <template #actions>
      <hw-dropdown
        :disabled="!product.isSetup"
        trigger="click"
        :options="[
          {
            content: '管理',
            value: 'manage',
            onClick: () => handleManageProduct(product),
          },
          {
            content: '删除',
            value: 'delete',
            onClick: () => handleDeleteItem(product),
          },
        ]"
      >
        <hw-button theme="default" :disabled="!product.isSetup" shape="square" variant="text">
          <more-icon />
        </hw-button>
      </hw-dropdown>
    </template>
  </hw-card>
</template>
<script lang="ts">
import { ShopIcon, CalendarIcon, ServiceIcon, UserAvatarIcon, LaptopIcon, MoreIcon, AddIcon } from '@/ui/icons';

export default {
  name: 'ListCard',
  components: {
    ShopIcon,
    CalendarIcon,
    ServiceIcon,
    UserAvatarIcon,
    LaptopIcon,
    MoreIcon,
    AddIcon,
  },
  props: {
    product: {
      type: Object,
    },
  },
  data() {
    return { typeMap: ['A', 'B', 'C', 'D', 'E'] };
  },
  methods: {
    handleClickManage(product) {
      this.$emit('manage-product', product);
    },
    handleClickDelete(product) {
      this.$emit('delete-item', product);
    },
  },
};
</script>

<style lang="less" scoped>
@import '@/style/variables';

.list-card-item {
  display: flex;
  flex-direction: column;
  cursor: pointer;

  &_detail {
    min-height: 140px;

    &--name {
      margin-bottom: 8px;
      font-size: 16px;
      font-weight: 400;
      color: var(--td-text-color-primary);
    }

    &--desc {
      color: var(--td-text-color-secondary);
      font-size: 12px;
      line-height: 20px;
      overflow: hidden;
      text-overflow: ellipsis;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      height: 40px;
    }
  }
}
</style>
