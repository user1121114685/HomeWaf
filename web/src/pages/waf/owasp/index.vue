<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-card class="list-card-container">
      <help-block :summary="$t('page.owasp.alert_message')" doc="guide/Owasp" />
      <hw-tabs v-model="activeTab" theme="card">
        <hw-tab-panel value="rules" :label="$t('page.owasp.tab_rules')">
          <rules-tab v-if="activeTab === 'rules'" :init-keyword="rulesInitKeyword" @ready="rulesInitKeyword = ''" @go-tuning="onGoTuning" />
        </hw-tab-panel>
        <hw-tab-panel value="tuning" :label="$t('page.owasp.tab_tuning')">
          <tuning-tab v-if="activeTab === 'tuning'" />
        </hw-tab-panel>
        <hw-tab-panel value="hit_stats" :label="$t('page.owasp.tab_hit_stats')">
          <hit-stats-tab v-if="activeTab === 'hit_stats'" @go-rule="onGoRule" />
        </hw-tab-panel>
        <hw-tab-panel value="upgrade" :label="$t('page.owasp.tab_upgrade')">
          <upgrade-tab v-if="activeTab === 'upgrade'" />
        </hw-tab-panel>
        <hw-tab-panel value="sandbox" :label="$t('page.owasp.tab_sandbox')">
          <sandbox-tab v-if="activeTab === 'sandbox'" @switch-tab="activeTab = $event" />
        </hw-tab-panel>
        <hw-tab-panel value="changelog" :label="$t('page.owasp.tab_changelog')">
          <change-log-tab v-if="activeTab === 'changelog'" @go-rule="onGoRule" />
        </hw-tab-panel>
        <hw-tab-panel value="usage" :label="$t('page.owasp.tab_usage')">
          <usage-tab v-if="activeTab === 'usage'" />
        </hw-tab-panel>
      </hw-tabs>
    </hw-card>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import RulesTab from './components/RulesTab.vue';
import TuningTab from './components/TuningTab.vue';
import UpgradeTab from './components/UpgradeTab.vue';
import SandboxTab from './components/SandboxTab.vue';
import ChangeLogTab from './components/ChangeLogTab.vue';
import UsageTab from './components/UsageTab.vue';
import HitStatsTab from './components/HitStatsTab.vue';

export default defineComponent({
  name: 'OwaspManage',
  components: { RulesTab, TuningTab, UpgradeTab, SandboxTab, ChangeLogTab, UsageTab, HitStatsTab },
  data() {
    return {
      activeTab: 'rules',
      rulesInitKeyword: '',
    };
  },
  mounted() {
    const tab = this.$route.query.tab as string;
    if (tab) {
      this.activeTab = tab;
    }
  },
  methods: {
    onGoRule(ruleId: number) {
      this.rulesInitKeyword = String(ruleId);
      this.activeTab = 'rules';
    },
    onGoTuning(_varName: string) {
      this.activeTab = 'tuning';
    },
  },
});
</script>

<style lang="less" scoped>
.list-card-container {
  padding-bottom: 16px;
}
</style>
