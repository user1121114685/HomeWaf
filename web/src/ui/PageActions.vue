<template>
  <Teleport v-if="mounted && target && !inline" :to="target">
    <div class="page-action-group" v-bind="$attrs"><slot /></div>
  </Teleport>
  <div v-else class="page-action-group" v-bind="$attrs"><slot /></div>
</template>
<script setup lang="ts">
import { inject, onMounted, ref } from 'vue';
defineOptions({ inheritAttrs: false });
defineProps({ inline: Boolean });
const target = inject<string | undefined>('page-actions-target', undefined);
const mounted = ref(false);
onMounted(() => {
  mounted.value = true;
});
</script>
