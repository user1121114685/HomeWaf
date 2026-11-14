<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="doc-container">
    <hw-card class="doc-card">
      <p class="doc-title">{{ $t('menu.oplatform.doc_title') }}</p>
      <!-- 不内嵌 iframe：点击才跳转，且不带 Referer -->
      <hw-link theme="primary" :href="docUrl" target="_blank" rel="noopener noreferrer">
        {{ docUrl }}
      </hw-link>
    </hw-card>
  </div>
</template>

<script>
export default {
  name: 'OPlatformDoc',
  computed: {
    docUrl() {
      return '/#/help'
    }
  }
}
</script>

<style scoped>
.doc-container {
  padding: 16px;
}
.doc-card {
  max-width: 640px;
}
.doc-title {
  font-size: 16px;
  font-weight: 500;
  margin-bottom: 12px;
}
</style>
