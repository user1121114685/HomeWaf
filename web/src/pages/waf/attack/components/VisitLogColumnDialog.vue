<template>
  <hw-dialog
    v-model:visible="visible"
    :header="text('显示列与顺序', 'Columns and order')"
    width="min(760px, calc(100vw - 32px))"
    :confirm-btn="text('保存', 'Save')"
    :cancel-btn="text('取消', 'Cancel')"
    @confirm="save"
  >
    <div class="column-settings">
      <section>
        <h3>{{ text('显示顺序', 'Display order') }}</h3>
        <ol class="column-order">
          <li
            v-for="(key, index) in visibleKeys"
            :key="key"
            :class="{
              dragging: dragged === key,
              'drop-before': target === key && !afterTarget,
              'drop-after': target === key && afterTarget,
            }"
            @dragover="dragOver($event, key)"
            @dragleave="leaveRow($event, key)"
            @drop="drop($event, key)"
          >
            <span
              class="column-grip"
              draggable="true"
              aria-hidden="true"
              :title="text('拖动排序', 'Drag to reorder')"
              @dragstart="startDrag($event, key)"
              @dragend="clearDrag"
              ><Rank
            /></span>
            <span class="column-name">{{ labels.get(key) }}</span>
            <select
              :aria-label="text(labels.get(key) + '的位置', 'Position of ' + labels.get(key))"
              :value="index"
              @change="selectPosition($event, key)"
            >
              <option v-for="(other, position) in visibleKeys" :key="other" :value="position">
                {{ text('第 ' + (position + 1) + ' 列', 'Col. ' + (position + 1))
                }}{{ other === key ? '' : ' · ' + labels.get(other) }}
              </option>
            </select>
          </li>
        </ol>
      </section>
      <section>
        <h3>{{ text('选择显示列', 'Visible columns') }}</h3>
        <div class="column-choices">
          <hw-checkbox
            v-for="column in columns"
            :key="column.colKey"
            :model-value="draft.includes(column.colKey)"
            :disabled="column.colKey === 'op'"
            @change="toggle(column.colKey, $event)"
            >{{ column.title }}</hw-checkbox
          >
        </div>
        <hw-button variant="text" @click="draft = [...recommendedLogColumns]">
          {{ text('恢复推荐列', 'Restore recommended columns') }}
        </hw-button>
      </section>
    </div>
  </hw-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, type PropType } from 'vue';
import { useI18n } from 'vue-i18n';
import { Rank } from '@element-plus/icons-vue';
import { recommendedLogColumns } from '../logPresentation';

const props = defineProps({
  columns: { type: Array as PropType<{ colKey: string; title: string }[]>, required: true },
});
const emit = defineEmits(['save']);
const { locale } = useI18n();
const text = (zh: string, en: string) => (locale.value === 'en_US' ? en : zh);
const visible = ref(false);
const draft = ref<string[]>([]);
const labels = computed(() => new Map(props.columns.map((col) => [col.colKey, col.title])));
const visibleKeys = computed(() => draft.value.filter((key) => labels.value.has(key)));
const dragged = ref('');
const target = ref('');
const afterTarget = ref(false);

function open(selected: string[]) {
  draft.value = [...selected];
  clearDrag();
  visible.value = true;
}
defineExpose({ open });

function toggle(key: string, checked: boolean) {
  if (checked) {
    const end = draft.value.at(-1) === 'op' ? draft.value.length - 1 : draft.value.length;
    draft.value.splice(end, 0, key);
  } else {
    draft.value = draft.value.filter((item) => item !== key);
  }
}

function move(key: string, position: number) {
  const from = draft.value.indexOf(key);
  const to = draft.value.indexOf(visibleKeys.value[position]);
  if (from < 0 || to < 0 || from === to) return;
  draft.value.splice(from, 1);
  draft.value.splice(to, 0, key);
}

async function selectPosition(event: Event, key: string) {
  const select = event.currentTarget as HTMLSelectElement;
  move(key, Number(select.value));
  await nextTick();
  select.focus();
}

function startDrag(event: DragEvent, key: string) {
  if (!event.dataTransfer) return;
  dragged.value = key;
  event.dataTransfer.effectAllowed = 'move';
  event.dataTransfer.setData('text/plain', key);
  const row = (event.currentTarget as HTMLElement).closest('li');
  if (row) event.dataTransfer.setDragImage(row, 16, row.clientHeight / 2);
}

function dragOver(event: DragEvent, key: string) {
  if (!dragged.value) return;
  event.preventDefault();
  const row = (event.currentTarget as HTMLElement).getBoundingClientRect();
  target.value = key === dragged.value ? '' : key;
  afterTarget.value = event.clientY >= row.top + row.height / 2;
}

function leaveRow(event: DragEvent, key: string) {
  if (target.value === key && !(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node | null)) {
    target.value = '';
  }
}

function drop(event: DragEvent, key: string) {
  if (!dragged.value) return;
  dragOver(event, key);
  const boundary = visibleKeys.value.indexOf(key) + Number(afterTarget.value);
  const from = visibleKeys.value.indexOf(dragged.value);
  move(dragged.value, boundary - Number(from < boundary));
  clearDrag();
}

function clearDrag() {
  dragged.value = '';
  target.value = '';
}

function save() {
  emit('save', [...draft.value]);
  visible.value = false;
}
</script>

<style scoped>
.column-settings {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 28px;
}
.column-settings h3 {
  margin: 0 0 10px;
  font-size: 15px;
  font-weight: 600;
}
.column-order {
  margin: 0;
  padding: 0;
  list-style: none;
}
.column-order li {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  border-bottom: 1px solid var(--hw-line);
}
.column-grip {
  display: grid;
  place-items: center;
  width: 28px;
  align-self: stretch;
  color: var(--hw-muted);
  cursor: grab;
  user-select: none;
}
.column-grip svg {
  width: 16px;
  height: 16px;
}
.column-order .dragging {
  background: var(--hw-subtle);
}
.drop-before {
  box-shadow: inset 0 2px var(--hw-accent);
}
.drop-after {
  box-shadow: inset 0 -2px var(--hw-accent);
}
.column-name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.column-order select {
  width: 84px;
  min-height: 32px;
  padding-inline: 6px;
  background: var(--hw-panel);
  border: 1px solid var(--hw-line);
  border-radius: var(--hw-r-sm);
  color: var(--hw-ink);
  font: inherit;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}
.column-order select:focus-visible {
  outline: 2px solid var(--hw-accent);
  outline-offset: 1px;
}
.column-choices {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 8px;
  margin-bottom: 12px;
}
.column-choices :deep(.el-checkbox) {
  margin: 0;
  height: auto;
  min-height: 38px;
  white-space: normal;
}
@media (max-width: 660px) {
  .column-settings {
    grid-template-columns: minmax(0, 1fr);
    gap: 24px;
  }
  .column-order select,
  .column-choices :deep(.el-checkbox) {
    min-height: 44px;
  }
}
</style>
