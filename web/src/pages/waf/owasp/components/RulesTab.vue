<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div>
    <hw-form ref="form" :data="searchForm" layout="inline" :style="{ marginBottom: '8px' }">
      <hw-form-item :label="$t('page.owasp.rules.file')">
        <hw-input v-model="searchForm.file" :placeholder="$t('page.owasp.rules.file_ph')" clearable style="width:220px" />
      </hw-form-item>
      <hw-form-item :label="$t('page.owasp.rules.severity')">
        <hw-select v-model="searchForm.severity" clearable style="width:140px">
          <hw-option value="CRITICAL" label="CRITICAL" />
          <hw-option value="ERROR" label="ERROR" />
          <hw-option value="WARNING" label="WARNING" />
          <hw-option value="NOTICE" label="NOTICE" />
        </hw-select>
      </hw-form-item>
      <hw-form-item :label="$t('page.owasp.rules.paranoia')">
        <hw-select v-model="searchForm.paranoia" clearable style="width:100px">
          <hw-option :value="1" label="1" />
          <hw-option :value="2" label="2" />
          <hw-option :value="3" label="3" />
          <hw-option :value="4" label="4" />
        </hw-select>
      </hw-form-item>
      <hw-form-item :label="$t('page.owasp.rules.status')">
        <hw-select v-model="searchForm.status" clearable style="width:140px">
          <hw-option value="default" :label="$t('page.owasp.rules.status_default')" />
          <hw-option value="disabled" :label="$t('page.owasp.rules.status_disabled')" />
          <hw-option value="modified" :label="$t('page.owasp.rules.status_modified')" />
        </hw-select>
      </hw-form-item>
      <hw-form-item>
        <hw-input v-model="searchForm.keyword" :placeholder="$t('page.owasp.rules.keyword_ph')" clearable style="width:220px" />
      </hw-form-item>
      <hw-form-item>
        <hw-button theme="primary" @click="getList">{{ $t('common.search') }}</hw-button>
        <hw-button variant="outline" @click="resetSearch">{{ $t('common.reset') }}</hw-button>
        <hw-button variant="outline" theme="warning" @click="onReload">{{ $t('page.owasp.rules.reload') }}</hw-button>
      </hw-form-item>
    </hw-form>

    <!-- 我的改动快速筛选区 -->
    <div class="my-changes-bar">
      <span class="my-changes-label">{{ $t('page.owasp.rules.my_changes') }}：</span>
      <hw-button
        :variant="searchForm.status === 'disabled' ? 'base' : 'outline'"
        theme="danger"
        size="small"
        @click="quickFilter(searchForm.status === 'disabled' ? '' : 'disabled')"
      >
        {{ $t('page.owasp.rules.status_disabled') }}
        <hw-badge
          v-if="disabledCount > 0"
          :count="disabledCount"
          size="small"
          style="margin-left:4px"
        />
        <span v-else style="margin-left:4px;opacity:.7">0</span>
      </hw-button>
      <hw-button
        :variant="searchForm.status === 'modified' ? 'base' : 'outline'"
        theme="warning"
        size="small"
        style="margin-left:8px"
        @click="quickFilter(searchForm.status === 'modified' ? '' : 'modified')"
      >
        {{ $t('page.owasp.rules.status_modified') }}
        <hw-badge
          v-if="modifiedCount > 0"
          :count="modifiedCount"
          size="small"
          style="margin-left:4px"
        />
        <span v-else style="margin-left:4px;opacity:.7">0</span>
      </hw-button>
      <hw-button
        v-if="searchForm.status === 'disabled' || searchForm.status === 'modified'"
        variant="text"
        size="small"
        style="margin-left:8px"
        @click="quickFilter('')"
      >{{ $t('page.owasp.rules.show_all') }}</hw-button>
    </div>

    <hw-table
      :columns="columns"
      :data="data"
      rowKey="uid"
      :loading="dataLoading"
      :pagination="pagination"
      verticalAlign="top"
      hover
      @page-change="onPageChange"
    >
      <template #status="{ row }">
        <hw-tag v-if="row.disabled" theme="danger" variant="light">{{ $t('page.owasp.rules.status_disabled') }}</hw-tag>
        <hw-tag v-else-if="row.modified" theme="warning" variant="light">{{ $t('page.owasp.rules.status_modified') }}</hw-tag>
        <hw-tag v-else theme="success" variant="light">{{ $t('page.owasp.rules.status_default') }}</hw-tag>
      </template>
      <template #severity="{ row }">
        <hw-tag :theme="severityTheme(row.severity)" variant="light">{{ row.severity || '-' }}</hw-tag>
      </template>
      <template #op="{ row }">
        <a class="t-button-link" @click="onView(row)">{{ $t('page.owasp.rules.view') }}</a>
        <a class="t-button-link" @click="onToggle(row)">{{ row.disabled ? $t('page.owasp.rules.enable') : $t('page.owasp.rules.disable') }}</a>
        <a class="t-button-link" @click="onEdit(row)">{{ $t('page.owasp.rules.override') }}</a>
        <a v-if="row.modified || row.disabled" class="t-button-link" @click="onReset(row)">{{ $t('page.owasp.rules.reset') }}</a>
      </template>
    </hw-table>

    <!-- 查看 / 改写对话框 -->
    <hw-dialog :header="dialogTitle" v-model:visible="dialogVisible" :width="860" :footer="false">
      <template #body><div >
        <!-- 字段含义说明 -->
        <hw-collapse expand-icon-placement="right" class="rule-glossary">
          <hw-collapse-panel :header="$t('page.owasp.rules.glossary_title')" value="glossary">
            <ul class="glossary-list">
              <li><b>id</b> {{ $t('page.owasp.rules.glossary_id') }}</li>
              <li><b>phase</b> {{ $t('page.owasp.rules.glossary_phase') }}</li>
              <li><b>severity</b> {{ $t('page.owasp.rules.glossary_severity') }}</li>
              <li><b>paranoia</b> {{ $t('page.owasp.rules.glossary_paranoia') }}</li>
              <li><b>tags</b> {{ $t('page.owasp.rules.glossary_tags') }}</li>
              <li><b>msg</b> {{ $t('page.owasp.rules.glossary_msg') }}</li>
              <li><b>SecRule / SecAction</b> {{ $t('page.owasp.rules.glossary_directive') }}</li>
            </ul>
          </hw-collapse-panel>
        </hw-collapse>

        <!-- 元数据 -->
        <hw-descriptions :column="2" size="small" bordered class="rule-meta">
          <hw-descriptions-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.rule_id') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_id')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <b>{{ editForm.id }}</b>
          </hw-descriptions-item>
          <hw-descriptions-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.directive') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_directive')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            {{ editForm.directive || '-' }}
          </hw-descriptions-item>
          <hw-descriptions-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.phase_label') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_phase')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <hw-tag v-if="editForm.phase" theme="primary" variant="light">Phase {{ editForm.phase }} - {{ phaseName(editForm.phase) }}</hw-tag>
            <span v-else>-</span>
          </hw-descriptions-item>
          <hw-descriptions-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.severity') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_severity')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <hw-tag v-if="editForm.severity" :theme="severityTheme(editForm.severity)" variant="light">
              {{ editForm.severity }} (+{{ severityScore(editForm.severity) }})
            </hw-tag>
            <span v-else>-</span>
          </hw-descriptions-item>
          <hw-descriptions-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.paranoia') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_paranoia')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <hw-tag v-if="editForm.paranoia" theme="warning" variant="light">PL {{ editForm.paranoia }}</hw-tag>
            <span v-else>{{ $t('page.owasp.rules.paranoia_none') }}</span>
          </hw-descriptions-item>
          <hw-descriptions-item :label="$t('page.owasp.rules.file')">
            <span>{{ editForm.source_file }}</span>
          </hw-descriptions-item>
          <hw-descriptions-item :span="2" :label="$t('page.owasp.rules.message')">
            {{ editForm.message || '-' }}
          </hw-descriptions-item>
          <hw-descriptions-item :span="2">
            <template #label>
              <span>{{ $t('page.owasp.rules.tags') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.glossary_tags')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <template v-if="editForm.tags && editForm.tags.length">
              <hw-tag v-for="t in editForm.tags" :key="t" variant="light" size="small" style="margin:2px 4px 2px 0">{{ t }}</hw-tag>
            </template>
            <span v-else>-</span>
          </hw-descriptions-item>
        </hw-descriptions>

        <hw-form :data="editForm" @submit="onSubmitOverride" :labelWidth="100" class="rule-form">
          <!-- CRS tx.* 变量检测提示 -->
          <div v-if="detectedTxVars.length" class="tx-vars-hint">
            <hw-alert theme="warning" style="margin-bottom:8px">
              <template #message>
                <span>该规则引用了以下 <b>CRS 事务变量</b>，建议在「<b>调参 → CRS 变量参数</b>」中修改，而非直接改写规则内容（改写规则升级后会被覆盖，变量配置不会）：</span>
                <div class="tx-vars-list">
                  <hw-tag
                    v-for="v in detectedTxVars"
                    :key="v"
                    theme="warning"
                    variant="light"
                    size="small"
                    style="margin:2px 4px 2px 0;cursor:pointer;font-family:monospace"
                    @click="goToTuningVar(v)"
                  >tx.{{ v }} ↗</hw-tag>
                </div>
              </template>
            </hw-alert>
          </div>
          <hw-form-item>
            <template #label>
              <span>{{ $t('page.owasp.rules.content') }}</span>
              <hw-tooltip :content="$t('page.owasp.rules.content_tip')" placement="top-left">
                <hw-icon name="help-circle" class="label-icon" />
              </hw-tooltip>
            </template>
            <hw-textarea v-model="editForm.content" :autosize="{ minRows: 8, maxRows: 20 }" :readonly="editMode === 'view'" />
          </hw-form-item>
          <hw-form-item style="float: right">
            <hw-button variant="outline" @click="dialogVisible = false">{{ $t('common.close') }}</hw-button>
            <hw-button variant="outline" theme="default" @click="onAiAnalyze">{{ $t('page.owasp.rules.ai_analyze') }}</hw-button>
            <hw-button v-if="editMode === 'edit'" theme="primary" type="submit">{{ $t('common.confirm') }}</hw-button>
          </hw-form-item>
        </hw-form>
      </div></template>
    </hw-dialog>

    <!-- AI 分析预览确认对话框 -->
    <hw-dialog
      :header="$t('page.owasp.rules.ai_analyze')"
      v-model:visible="aiDialogVisible"
      width="680px"
      :onConfirm="confirmSendAi"
      :onClose="() => { aiDialogVisible = false }"
    >
      <hw-alert theme="info" :message="$t('page.owasp.rules.ai_analyze_tip')" style="margin-bottom:10px" />
      <hw-textarea v-model="aiPrompt" :autosize="{ minRows: 8, maxRows: 18 }" />
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import {
  owaspRulesListApi,
  owaspRuleDetailApi,
  owaspRuleToggleApi,
  owaspRuleOverrideApi,
  owaspRuleResetApi,
  owaspReloadApi,
} from '@/apis/owasp';

export default defineComponent({
  name: 'OwaspRulesTab',
  props: {
    initKeyword: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      dataLoading: false,
      data: [],
      disabledCount: 0,
      modifiedCount: 0,
      pagination: {
        current: 1,
        pageSize: 20,
        total: 0,
        showJumper: true,
        pageSizeOptions: [20, 50, 100, 200],
      },
      searchForm: {
        file: '',
        severity: '',
        paranoia: undefined,
        status: '',
        keyword: '',
      },
      columns: [
        { title: this.$t('page.owasp.rules.rule_id'), colKey: 'id', width: 100 },
        { title: this.$t('page.owasp.rules.status'), colKey: 'status', width: 110 },
        { title: this.$t('page.owasp.rules.severity'), colKey: 'severity', width: 110 },
        { title: this.$t('page.owasp.rules.paranoia'), colKey: 'paranoia', width: 80 },
        { title: this.$t('page.owasp.rules.message'), colKey: 'message', ellipsis: true },
        { title: this.$t('page.owasp.rules.file'), colKey: 'file', width: 260, ellipsis: true },
        { title: this.$t('common.op'), colKey: 'op', width: 260, fixed: 'right' },
      ],
      dialogVisible: false,
      dialogTitle: '',
      editMode: 'view', // view | edit
      aiDialogVisible: false,
      aiPrompt: '',
      editForm: {
        id: 0,
        source_file: '',
        content: '',
        directive: '',
        phase: 0,
        severity: '',
        paranoia: 0,
        message: '',
        tags: [] as string[],
      },
    };
  },
  mounted() {
    const ruleId = this.$route.query.rule_id as string;
    if (ruleId) {
      this.searchForm.keyword = ruleId;
    } else if (this.initKeyword) {
      this.searchForm.keyword = this.initKeyword;
    }
    this.getList();
    this.$emit('ready');
  },
  computed: {
    // 从当前对话框规则内容中提取 %{tx.VAR} 变量（排除纯评分变量）
    detectedTxVars(): string[] {
      const content = this.editForm.content || '';
      const scoreVars = new Set([
        'inbound_anomaly_score', 'detection_inbound_anomaly_score',
        'blocking_inbound_anomaly_score', 'outbound_anomaly_score',
        'critical_anomaly_score', 'error_anomaly_score', 'warning_anomaly_score', 'notice_anomaly_score',
        'inbound_anomaly_score_pl1', 'inbound_anomaly_score_pl2',
        'inbound_anomaly_score_pl3', 'inbound_anomaly_score_pl4',
      ]);
      const matches = content.match(/%\{tx\.([a-zA-Z_][a-zA-Z0-9_]*)\}/g) || [];
      const vars = new Set<string>();
      for (const m of matches) {
        const name = m.replace('%{tx.', '').replace('}', '');
        if (!scoreVars.has(name)) vars.add(name);
      }
      return Array.from(vars);
    },
  },
  methods: {
    severityTheme(sev: string) {
      const s = (sev || '').toUpperCase();
      if (s === 'CRITICAL') return 'danger';
      if (s === 'ERROR') return 'warning';
      if (s === 'WARNING') return 'primary';
      return 'default';
    },
    severityScore(sev: string): number {
      // CRS 默认: CRITICAL=5, ERROR=4, WARNING=3, NOTICE=2
      const s = (sev || '').toUpperCase();
      if (s === 'CRITICAL') return 5;
      if (s === 'ERROR') return 4;
      if (s === 'WARNING') return 3;
      if (s === 'NOTICE') return 2;
      return 0;
    },
    phaseName(p: number): string {
      switch (p) {
        case 1: return this.$t('page.owasp.rules.phase_1');
        case 2: return this.$t('page.owasp.rules.phase_2');
        case 3: return this.$t('page.owasp.rules.phase_3');
        case 4: return this.$t('page.owasp.rules.phase_4');
        case 5: return this.$t('page.owasp.rules.phase_5');
        default: return '-';
      }
    },
    getList() {
      this.dataLoading = true;
      const params: any = {
        page_index: this.pagination.current,
        page_size: this.pagination.pageSize,
      };
      if (this.searchForm.file) params.file = this.searchForm.file;
      if (this.searchForm.severity) params.severity = this.searchForm.severity;
      if (this.searchForm.paranoia) params.paranoia = this.searchForm.paranoia;
      if (this.searchForm.status) params.status = this.searchForm.status;
      if (this.searchForm.keyword) params.keyword = this.searchForm.keyword;
      owaspRulesListApi(params)
        .then((res) => {
          if (res.code === 0) {
            const list = res.data.list || [];
            // 同一 ID 的规则可能来自多个文件（chained 或分散声明），这里给每行加 uid 保证 rowKey 唯一。
            this.data = list.map((row, idx) => ({
              ...row,
              uid: `${row.id}-${row.file}-${row.line_start}-${idx}`,
            }));
            this.pagination.total = res.data.total || 0;
            // 仅在无 status 过滤时更新全局计数（保证徽标始终准确）
            if (!this.searchForm.status) {
              this.disabledCount = res.data.disabled_count || 0;
              this.modifiedCount = res.data.modified_count || 0;
            }
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch((e) => console.log(e))
        .finally(() => (this.dataLoading = false));
    },
    resetSearch() {
      this.searchForm = { file: '', severity: '', paranoia: undefined, status: '', keyword: '' };
      this.pagination.current = 1;
      this.getList();
    },
    quickFilter(status: string) {
      this.searchForm.status = status;
      this.pagination.current = 1;
      this.getList();
    },
    onPageChange(p) {
      this.pagination.current = p.current;
      this.pagination.pageSize = p.pageSize;
      this.getList();
    },
    onReload() {
      owaspReloadApi().then((res) => {
        if (res.code === 0) {
          this.$message.success(res.msg);
          this.getList();
        } else {
          this.$message.warning(res.msg);
        }
      });
    },
    onToggle(row) {
      owaspRuleToggleApi({ id: row.id, disabled: !row.disabled, source_file: row.file }).then((res) => {
        if (res.code === 0) {
          this.$message.success(res.msg);
          this.getList();
        } else {
          this.$message.warning(res.msg);
        }
      });
    },
    onReset(row) {
      owaspRuleResetApi({ id: row.id }).then((res) => {
        if (res.code === 0) {
          this.$message.success(res.msg);
          this.getList();
        } else {
          this.$message.warning(res.msg);
        }
      });
    },
    fillForm(rule, override, mode: 'view' | 'edit') {
      this.editMode = mode;
      this.editForm = {
        id: rule.id,
        source_file: rule.file,
        content: override && override.custom_content ? override.custom_content : rule.raw,
        directive: rule.directive || '',
        phase: rule.phase || 0,
        severity: rule.severity || '',
        paranoia: rule.paranoia || 0,
        message: rule.message || '',
        tags: rule.tags || [],
      };
      const titleKey = mode === 'edit' ? 'page.owasp.rules.override_title' : 'page.owasp.rules.detail_title';
      this.dialogTitle = this.$t(titleKey) + ` (${rule.id})`;
      this.dialogVisible = true;
    },
    onView(row) {
      owaspRuleDetailApi({ id: row.id }).then((res) => {
        if (res.code !== 0) {
          this.$message.warning(res.msg);
          return;
        }
        this.fillForm(res.data.rule, res.data.override, 'view');
      });
    },
    onEdit(row) {
      owaspRuleDetailApi({ id: row.id }).then((res) => {
        if (res.code !== 0) {
          this.$message.warning(res.msg);
          return;
        }
        this.fillForm(res.data.rule, res.data.override, 'edit');
      });
    },
    onAiAnalyze() {
      const f = this.editForm as any;
      const phaseText = f.phase ? `Phase ${f.phase} - ${this.phaseName(f.phase)}` : '-';
      const plText = f.paranoia ? `PL${f.paranoia}` : '未指定（任意 PL 都启用）';
      const tagsText = (f.tags && f.tags.length) ? f.tags.join(', ') : '-';
      // 构建结构化提示词，告知 AI 这是一条 OWASP CRS 规则
      this.aiPrompt = [
        '以下是一条来自 OWASP 核心规则集（CRS, Core Rule Set）的 WAF 规则，请用中文解释：',
        '1. 该规则的防护意图是什么？',
        '2. 它在什么场景/条件下会被触发？',
        '3. 可能导致哪些误报场景，管理员应如何调优？',
        '',
        `规则 ID：${f.id}`,
        `指令类型：${f.directive || 'SecRule'}`,
        `执行阶段：${phaseText}`,
        `严重级别：${f.severity || '-'}`,
        `偏执级别：${plText}`,
        `描述（msg）：${f.message || '-'}`,
        `标签：${tagsText}`,
        `所属文件：${f.source_file || '-'}`,
        '',
        '规则原文：',
        f.content || '-',
      ].join('\n');
      this.aiDialogVisible = true;
    },
    confirmSendAi() {
      // 规则解读走 OWASP 规则提示词，不套风险评分格式
      this.$bus.$emit('sendAi', { q: this.aiPrompt, scene: 'owasp_rule' });
      this.aiDialogVisible = false;
    },
    goToTuningVar(varName: string) {
      // 关闭当前弹窗，切换到调参 tab（父组件通过 $parent 或 event bus 处理）
      this.dialogVisible = false;
      this.$emit('go-tuning', varName);
    },
    onSubmitOverride() {
      owaspRuleOverrideApi({
        id: this.editForm.id,
        source_file: this.editForm.source_file,
        content: this.editForm.content,
      }).then((res) => {
        if (res.code === 0) {
          this.$message.success(res.msg);
          this.dialogVisible = false;
          this.getList();
        } else {
          this.$message.warning(res.msg);
        }
      });
    },
  },
});
</script>

<style lang="less" scoped>
.t-button-link + .t-button-link {
  margin-left: 8px;
}
.my-changes-bar {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
  padding: 8px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--td-radius-medium);
  border: 1px solid var(--td-component-border);
}
.my-changes-label {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin-right: 8px;
  white-space: nowrap;
}
.tx-vars-hint {
  margin-bottom: 4px;
}
.tx-vars-list {
  margin-top: 6px;
}
.label-icon {
  margin-left: 4px;
  color: var(--td-text-color-placeholder);
  cursor: help;
  vertical-align: -2px;
}
.rule-glossary {
  margin-bottom: 12px;
}
.glossary-list {
  margin: 0;
  padding-left: 20px;
  line-height: 1.9;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.rule-meta {
  margin-bottom: 16px;
}
.rule-form {
  margin-top: 8px;
}
</style>
