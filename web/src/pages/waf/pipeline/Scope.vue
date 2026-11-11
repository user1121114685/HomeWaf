<template><Fields :model="model" :definitions="definitions" /></template>
<script setup lang="ts">
import Fields from './Fields.vue';
import type { Field } from './model';
defineProps({model:{type:Object,required:true}});
const definitions: Field[] = [
  { path:'hosts', label:'限定主机名',kind:'lines',help:'每行一个主机名；只在本监听端口内匹配，不会借 Host 切换到其他端口策略。留空为全部。' },
  { path:'paths', label:'限定路径前缀',kind:'lines',help:'每行一个 / 开头的路径；留空为全部。' },
  { path:'methods', label:'限定 HTTP 方法',kind:'lines',help:'每行一个大写方法；留空为全部。' },
  { path:'exclude_paths', label:'路径例外',kind:'lines',help:'命中这些路径前缀时跳过本条规则，其他规则照常。' },
  { path:'exclude_fields', label:'参数名例外',kind:'lines',help:'按字段名排除，例如富文本正文 content；不要把整个站点设为trusted来处理单字段误报。CRS 不支持字段例外。' },
];
</script>
