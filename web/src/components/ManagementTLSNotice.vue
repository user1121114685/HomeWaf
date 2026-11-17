<template>
  <ManagementTrustGuide
    v-if="info?.https_configured && isHttp"
    :ca-pem="info.ca_pem"
    :fingerprint="info.ca?.fingerprint"
    :https-url="httpsUrl"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import request from '@/utils/request';
import ManagementTrustGuide from './ManagementTrustGuide.vue';
const info = ref<any>(null);
const isHttp = window.location.protocol === 'http:';
const httpsUrl = computed(() => {
  if (!info.value?.host_covered) return '';
  const url = new URL(window.location.href);
  url.protocol = 'https:';
  return url.href;
});
onMounted(async () => {
  if (!isHttp) return;
  try {
    const response = await request({ url: 'public/managementTls', method: 'get', timeout: 5000, background: true });
    if (response.code === 0) info.value = response.data;
  } catch {
    /* The existing login form remains usable if TLS discovery is unavailable. */
  }
});
</script>
