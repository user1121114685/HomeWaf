<template>
  <ElDialog
    v-model="visible"
    title="高级配置项维护"
    width="min(820px,96vw)"
    :close-on-click-modal="false"
    :before-close="beforeClose"
  >
    <p>
      仅维护有明确读取方的扩展配置。新增记录不会自动增加功能；内置配置由程序管理，只能在原页面修改。扩展值请勿存放密码或令牌，密钥使用对应功能的专用配置。
    </p>
    <ElAlert v-if="error" ref="errorAlert" tabindex="-1" :title="error" type="error" :closable="false" />
    <ElForm label-position="top" class="advanced-config-form" @submit.prevent="create">
      <ElFormItem label="配置键"
        ><ElInput
          v-model="form.item"
          :disabled="busy"
          aria-label="新增配置键"
          placeholder="读取方明确要求的配置键"
          maxlength="100"
      /></ElFormItem>
      <ElFormItem label="分类"
        ><ElInput v-model="form.item_class" :disabled="busy" aria-label="新增配置分类" maxlength="100"
      /></ElFormItem>
      <ElFormItem label="配置值" class="wide"
        ><ElInput
          v-model="form.value"
          type="textarea"
          :rows="3"
          :disabled="busy"
          aria-label="新增配置值"
          maxlength="65536"
      /></ElFormItem>
      <ElFormItem label="用途 / 读取方" class="wide"
        ><ElInput v-model="form.remarks" :disabled="busy" aria-label="配置用途" maxlength="500"
      /></ElFormItem>
      <ElButton type="primary" :loading="busy" :disabled="loading" native-type="submit">新增配置项</ElButton>
    </ElForm>
    <h3>可删除的扩展项</h3>
    <ElTable :data="customRows" :empty-text="loading ? '正在重新读取…' : '没有可删除的扩展配置'" max-height="300"
      ><ElTableColumn prop="item" label="配置键" min-width="180" /><ElTableColumn
        prop="remarks"
        label="用途"
        min-width="180"
      /><ElTableColumn label="操作" width="100"
        ><template #default="{ row }"
          ><ElButton link type="danger" :disabled="busy || loading" @click="remove(row)">删除</ElButton></template
        ></ElTableColumn
      ></ElTable
    >
    <template #footer
      ><ElButton :disabled="busy" @click="beforeClose(() => (visible = false))">关闭</ElButton></template
    >
  </ElDialog>
</template>
<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch, type PropType } from 'vue';
import {
  ElAlert,
  ElButton,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElTable,
  ElTableColumn,
} from 'element-plus';
import { add_system_config_api, del_detail_api } from '@/apis/systemconfig';
const props = defineProps({ rows: { type: Array as PropType<any[]>, default: () => [] }, loading: Boolean });
const emit = defineEmits(['changed']);
const visible = ref(false),
  busy = ref(false),
  error = ref('');
const errorAlert = ref<any>();
watch(error, async (message) => {
  if (message) {
    await nextTick();
    errorAlert.value?.$el?.focus();
  }
});
const form = reactive({ item: '', item_class: 'extension', value: '', remarks: '', item_type: 'string' });
const customRows = computed(() =>
  props.rows.filter(
    (row) => row.is_system === '0' && !row.is_sensitive && !['last_run_version', 'max_run_version'].includes(row.item),
  ),
);
async function create() {
  if (busy.value || props.loading) return;
  error.value = '';
  if (!/^[a-z][a-z0-9_.-]{0,99}$/.test(form.item.trim()) || !form.remarks.trim()) {
    error.value = '请填写合法配置键以及用途/读取方。';
    return;
  }
  busy.value = true;
  try {
    const res = await add_system_config_api({ ...form, item: form.item.trim() });
    if (res.code !== 0) throw new Error(res.msg || '新增失败');
    Object.assign(form, { item: '', value: '', remarks: '' });
    emit('changed');
    ElMessage.success('扩展配置记录已保存，请由对应功能验证读取效果。');
  } catch (cause: any) {
    error.value = cause.message || '新增失败，输入已保留。';
  } finally {
    busy.value = false;
  }
}
async function remove(row: any) {
  if (busy.value || props.loading) return;
  busy.value = true;
  error.value = '';
  try {
    await ElMessageBox.confirm(
      `删除扩展配置「${row.item}」？读取它的功能将无法获取该记录，请先确认影响。`,
      '删除配置项',
      { type: 'warning', confirmButtonText: '删除此项', cancelButtonText: '取消' },
    );
    const res = await del_detail_api({ id: row.id });
    if (res.code !== 0) throw new Error(res.msg || '删除失败');
    emit('changed');
    ElMessage.success('扩展配置已删除');
  } catch (cause: any) {
    if (cause !== 'cancel' && cause !== 'close') error.value = cause.message || '删除失败，请重试。';
  } finally {
    busy.value = false;
  }
}
async function beforeClose(done: () => void) {
  if (busy.value) return;
  if (form.item || form.value || form.remarks) {
    try {
      await ElMessageBox.confirm('放弃尚未新增的配置项？', '未保存的输入');
    } catch {
      return;
    }
  }
  Object.assign(form, { item: '', value: '', remarks: '' });
  done();
}
function open() {
  error.value = '';
  visible.value = true;
}
defineExpose({ open });
</script>
<style scoped>
p {
  color: var(--hw-muted);
  line-height: 1.7;
}
h3 {
  font-size: 16px;
  margin-top: 28px;
}
.advanced-config-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 20px;
  margin-top: 20px;
}
.wide {
  grid-column: 1/-1;
}
@media (max-width: 600px) {
  .advanced-config-form {
    grid-template-columns: 1fr;
  }
}
</style>
