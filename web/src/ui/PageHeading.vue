<template>
  <header class="page-heading">
    <div class="page-heading-copy">
      <div v-if="eyebrow" class="page-eyebrow">{{ eyebrow }}</div>
      <h1>{{ title }}</h1>
      <p v-if="description">{{ description }}</p>
    </div>
    <div v-if="$slots.default" class="page-heading-actions"><slot /></div>
  </header>
</template>
<script setup lang="ts">
defineProps({ title: { type: String, required: true }, description: String, eyebrow: String });
</script>
