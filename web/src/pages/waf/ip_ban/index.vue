<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <hw-card class="list-card-container resource-panel">
    <PageActions><ip-lookup /></PageActions>
    <ServicePunishments />
  </hw-card>
</template>

<script setup lang="ts">
import ServicePunishments from '@/pages/waf/pipeline/ServicePunishments.vue';
</script>
