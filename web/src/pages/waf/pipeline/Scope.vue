<template>
  <Fields :model="model" :definitions="supportsFields ? [...definitions, fieldException] : definitions" />
  <p v-if="!supportsFields && model.exclude_fields?.length" class="unused-fields">
    当前检测不使用已保存的参数名例外（{{ model.exclude_fields.join('、') }}）。
    <ElButton link @click="model.exclude_fields = []">清除未生效的参数名例外</ElButton>
  </p>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { ElButton } from 'element-plus';
import Fields from './Fields.vue';
import type { Field } from './model';
const props = defineProps({ model: { type: Object, required: true }, category: String });
const fieldDetectors = [
  'sqli', 'xss', 'upload', 'lfi', 'rce', 'java_inject', 'java_deser', 'php_deser', 'php_inject', 'asp_inject', 'ssti', 'ssrf',
];
const supportsFields = computed(() => fieldDetectors.includes(props.category || ''));
const definitions: Field[] = [
  { path:'hosts', label:'限定主机名',kind:'lines',help:'每行一个主机名；只在本监听端口内匹配，不会借 Host 切换到其他端口策略。留空为全部。' },
  { path:'paths', label:'限定路径前缀',kind:'lines',help:'每行一个 / 开头的路径；留空为全部。' },
  { path:'methods', label:'限定 HTTP 方法',kind:'lines',help:'每行一个大写方法；留空为全部。' },
  { path:'exclude_paths', label:'路径例外',kind:'lines',help:'命中这些路径前缀时跳过本条规则，其他规则照常。' },
];
const fieldException: Field = {
  path: 'exclude_fields', label: '参数名例外', kind: 'lines',
  help: '每行一个字段名。当前规则会跳过这些字段，其他字段继续检测。',
};
</script>
<style scoped>
.unused-fields {
  color: var(--hw-muted);
  overflow-wrap: anywhere;
}
</style>
