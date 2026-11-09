<template><Screen :key="token" :shared-token="token" shared-view /></template>
<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import Screen from './index.vue';
const route = useRoute();
const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''));
</script>
