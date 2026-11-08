<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <hw-card>
    <common-table />
  </hw-card>
</template>
<script lang="ts">
import CommonTable from '../components/CommonTable.vue';

export default {
  name: 'ListFilter',
  components: {
    CommonTable,
  },
};
</script>
