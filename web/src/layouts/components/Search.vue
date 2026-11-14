<template>
  <ElButton
    text
    class="search-trigger-btn"
    :aria-label="$t('common.search_page')"
    title="查找功能 · Ctrl K"
    @click="visible = true"
    ><Search
  /></ElButton>
  <ElDialog
    v-model="visible"
    title="查找功能"
    width="560px"
    class="console-search"
    @opened="input?.focus()"
    @closed="query = ''"
  >
    <label for="console-search-input" class="search-field-label">输入功能名称</label>
    <ElInput
      id="console-search-input"
      ref="input"
      v-model="query"
      placeholder="服务、证书、日志…"
      clearable
      @keydown.down.prevent="focusResult"
    />
    <nav ref="results" class="search-results" aria-label="搜索结果" @keydown="moveFocus">
      <RouterLink
        v-for="item in matches"
        :key="item.path"
        :to="item.path"
        :target="item.newTab ? '_blank' : undefined"
        :rel="item.newTab ? 'noopener noreferrer' : undefined"
        @click="visible = false"
        ><span
          >{{ item.title }}<small>{{ item.group }}</small></span
        ><TopRight v-if="item.newTab" /><ArrowRight v-else
      /></RouterLink>
      <p v-if="!matches.length" role="status">没有找到相关功能，请换一个关键词。</p>
    </nav>
    <p class="search-dialog-tip">方向键选择，Enter 打开，Esc 关闭。</p>
  </ElDialog>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { ElButton, ElDialog, ElInput } from 'element-plus';
import { ArrowRight, Search, TopRight } from '@element-plus/icons-vue';
import { navigationItems, filterNavigation } from '@/ui/navigation';
import store from '@/store';
const { t } = useI18n();
const visible = ref(false),
  query = ref(''),
  input = ref(),
  results = ref<HTMLElement>();
const matches = computed(() => filterNavigation(navigationItems(store.getters['permission/routers'], t), query.value));
function focusResult() {
  results.value?.querySelector<HTMLElement>('a')?.focus();
}
function moveFocus(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return;
  event.preventDefault();
  const links = [...results.value.querySelectorAll<HTMLElement>('a')];
  const index = links.indexOf(document.activeElement as HTMLElement) + (event.key === 'ArrowDown' ? 1 : -1);
  if (index < 0) input.value?.focus();
  else links[Math.min(index, links.length - 1)]?.focus();
}
function shortcut(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    visible.value = !visible.value;
  }
}
onMounted(() => window.addEventListener('keydown', shortcut));
onUnmounted(() => window.removeEventListener('keydown', shortcut));
</script>
<style scoped>
.search-trigger-btn {
  width: 40px;
  padding: 8px;
}
.search-trigger-btn svg {
  width: 18px;
  height: 18px;
}
.search-field-label {
  display: block;
  font-size: 13px;
  margin-bottom: 8px;
  color: var(--hw-muted);
}
.search-results {
  margin-top: 16px;
  max-height: 360px;
  overflow-y: auto;
}
.search-results a {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 12px;
  color: var(--hw-ink);
  border-radius: var(--hw-r-sm);
  min-height: 56px;
}
.search-results a:hover,
.search-results a:focus-visible {
  background: var(--hw-subtle);
}
.search-results small {
  display: block;
  font-size: 13px;
  color: var(--hw-muted);
  margin-top: 2px;
}
.search-results svg {
  width: 16px;
  height: 16px;
  color: var(--hw-muted);
}
.search-results p {
  padding: 32px 12px;
  text-align: center;
  color: var(--hw-muted);
}
.search-dialog-tip {
  margin-top: 16px;
  font-size: 13px;
  color: var(--hw-muted);
}
</style>
