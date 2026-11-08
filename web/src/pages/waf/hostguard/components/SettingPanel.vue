<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="panel-container">
    <hw-loading :loading="loading" size="small">
      <!-- 总开关单独放在最上面，并且开启前必须二次确认：
           白名单配错会把管理员自己锁在服务器外面，这是这个功能最大的事故来源 -->
      <hw-card size="small" class="master-card">
        <div class="master-row">
          <div>
            <div class="master-title">{{ $t('page.hostguard.cfg_enabled') }}</div>
            <div class="desc">{{ $t('page.hostguard.cfg_enabled_desc') }}</div>
          </div>
          <hw-switch v-model="masterEnabled" :loading="masterLoading" @change="onMasterChange" size="large" />
        </div>
      </hw-card>

      <hw-form :data="form" ref="form" @submit="onSave" :labelWidth="190" class="cfg-form">
        <hw-divider align="left">{{ $t('page.hostguard.group_detect') }}</hw-divider>

        <hw-form-item :label="$t('page.hostguard.cfg_mode')">
          <hw-radio-group v-model="form.host_guard_mode">
            <hw-radio-button value="observe">{{ $t('page.hostguard.mode_observe') }}</hw-radio-button>
            <hw-radio-button value="block">{{ $t('page.hostguard.mode_block') }}</hw-radio-button>
          </hw-radio-group>
          <div class="desc">{{ $t('page.hostguard.cfg_mode_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_find_time')">
          <hw-input-number v-model="form.host_guard_find_time" :min="1" theme="column" :style="{ width: '200px' }" />
          <span class="unit">{{ $t('common.unit_minute') }}</span>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_max_retry')">
          <hw-input-number v-model="form.host_guard_max_retry" :min="1" theme="column" :style="{ width: '200px' }" />
          <div class="desc">{{ $t('page.hostguard.cfg_max_retry_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_offender_reset')">
          <hw-input-number
            v-model="form.host_guard_offender_reset_day"
            :min="0"
            theme="column"
            :style="{ width: '200px' }"
          />
          <span class="unit">{{ $t('page.hostguard.unit_day') }}</span>
          <div class="desc">{{ $t('page.hostguard.cfg_offender_reset_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_soft_fail')">
          <hw-switch v-model="form.host_guard_count_soft_fail" :customValue="['1', '0']" />
          <div class="desc">{{ $t('page.hostguard.cfg_soft_fail_desc') }}</div>
        </hw-form-item>

        <hw-divider align="left">{{ $t('page.hostguard.group_whitelist') }}</hw-divider>

        <hw-form-item :label="$t('page.hostguard.cfg_whitelist')">
          <hw-textarea
            v-model="form.host_guard_whitelist"
            :autosize="{ minRows: 2, maxRows: 5 }"
            :style="{ width: '520px' }"
            :placeholder="$t('page.hostguard.cfg_whitelist_ph')"
          />
          <div class="desc">{{ $t('page.hostguard.cfg_whitelist_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_auto_lan')">
          <hw-switch v-model="form.host_guard_auto_lan" :customValue="['1', '0']" />
          <div class="desc">{{ $t('page.hostguard.cfg_auto_lan_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.whitelist_test')">
          <hw-input
            v-model="testIP"
            :style="{ width: '260px' }"
            :placeholder="$t('page.hostguard.whitelist_test_ph')"
          />
          <hw-button theme="default" @click="onTestWhitelist" style="margin-left: 8px">
            {{ $t('page.hostguard.whitelist_test_btn') }}
          </hw-button>
          <div v-if="testResult" class="test-result">
            <hw-tag :theme="testResult.whitelisted ? 'success' : 'warning'" variant="light">
              {{ testResult.whitelisted ? $t('page.hostguard.test_exempt') : $t('page.hostguard.test_not_exempt') }}
            </hw-tag>
            <span v-if="testResult.reason" class="test-reason">{{ testResult.reason }}</span>
          </div>
        </hw-form-item>

        <hw-divider align="left">{{ $t('page.hostguard.group_source') }}</hw-divider>

        <hw-form-item :label="$t('page.hostguard.cfg_log_paths')">
          <hw-input v-model="form.host_guard_log_paths" :style="{ width: '520px' }" />
          <div class="desc">{{ $t('page.hostguard.cfg_log_paths_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_ssh_ports')">
          <hw-input v-model="form.host_guard_ssh_ports" :style="{ width: '200px' }" placeholder="22" />
          <span class="unit-text">{{ $t('page.hostguard.cfg_ports_detected') }}: {{ (status.ssh_ports || []).join(', ') || '-' }}</span>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_rdp_ports')">
          <hw-input v-model="form.host_guard_rdp_ports" :style="{ width: '200px' }" placeholder="3389" />
          <span class="unit-text">{{ $t('page.hostguard.cfg_ports_detected') }}: {{ (status.rdp_ports || []).join(', ') || '-' }}</span>
          <div class="desc">{{ $t('page.hostguard.cfg_ports_desc') }}</div>
        </hw-form-item>

        <hw-divider align="left">{{ $t('page.hostguard.group_exec') }}</hw-divider>

        <hw-form-item :label="$t('page.hostguard.cfg_port_scope')">
          <hw-radio-group v-model="form.host_guard_port_scope">
            <hw-radio-button value="all">{{ $t('page.hostguard.scope_all') }}</hw-radio-button>
            <!-- 平台不支持时禁用：选了也只会静默按全端口封，留着就是个假开关 -->
            <hw-radio-button value="detected" :disabled="!portScopeSupported">
              {{ $t('page.hostguard.scope_detected') }}
            </hw-radio-button>
          </hw-radio-group>
          <div class="desc">{{ $t('page.hostguard.cfg_port_scope_desc') }}</div>
          <div v-if="!portScopeSupported" class="desc warn-desc">
            {{ $t('page.hostguard.cfg_port_scope_unsupported') }}
          </div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_exec_mode')">
          <hw-select v-model="form.host_guard_exec_mode" :style="{ width: '200px' }">
            <hw-option key="auto" value="auto" :label="$t('page.hostguard.exec_auto')" />
            <hw-option key="ipset" value="ipset" :label="$t('page.hostguard.exec_ipset')" />
            <hw-option key="rule" value="rule" :label="$t('page.hostguard.exec_rule')" />
          </hw-select>
          <span class="unit-text">{{ $t('page.hostguard.cfg_exec_current') }}: {{ status.exec_mode || '-' }}</span>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_debounce')">
          <hw-input-number v-model="form.host_guard_debounce_sec" :min="1" theme="column" :style="{ width: '200px' }" />
          <span class="unit">{{ $t('page.hostguard.unit_second') }}</span>
          <div class="desc">{{ $t('page.hostguard.cfg_debounce_desc') }}</div>
        </hw-form-item>

        <hw-divider align="left">{{ $t('page.hostguard.group_flood') }}</hw-divider>

        <hw-form-item :label="$t('page.hostguard.cfg_max_entries')">
          <hw-input-number
            v-model="form.host_guard_max_ban_entries"
            :min="0"
            theme="column"
            :style="{ width: '200px' }"
          />
          <div class="desc">{{ $t('page.hostguard.cfg_max_entries_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_rate_limit')">
          <hw-input-number
            v-model="form.host_guard_ban_rate_limit"
            :min="0"
            theme="column"
            :style="{ width: '200px' }"
          />
          <div class="desc">{{ $t('page.hostguard.cfg_rate_limit_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_subnet')">
          <hw-switch v-model="form.host_guard_subnet_aggregate" :customValue="['1', '0']" />
          <hw-input-number
            v-model="form.host_guard_subnet_threshold"
            :min="2"
            theme="column"
            :style="{ width: '160px', marginLeft: '12px' }"
          />
          <div class="desc warn-desc">{{ $t('page.hostguard.cfg_subnet_desc') }}</div>
        </hw-form-item>

        <hw-form-item :label="$t('page.hostguard.cfg_notify')">
          <hw-switch v-model="form.host_guard_notify" :customValue="['1', '0']" />
        </hw-form-item>

        <hw-divider align="left">{{ $t('page.hostconn.group_conn') }}</hw-divider>

        <hw-form-item :label="$t('page.hostconn.cfg_enabled')">
          <hw-switch v-model="form.host_conn_enabled" :customValue="['1', '0']" />
        </hw-form-item>

        <hw-form-item :label="$t('page.hostconn.cfg_cache_sec')">
          <hw-input-number v-model="form.host_conn_cache_sec" :min="1" theme="column" :style="{ width: '200px' }" />
          <span class="unit">{{ $t('page.hostguard.unit_second') }}</span>
          <div class="desc">{{ $t('page.hostconn.cfg_cache_sec_desc') }}</div>
        </hw-form-item>

        <hw-form-item>
          <hw-button theme="primary" type="submit" :loading="saving">{{ $t('common.save') }}</hw-button>
        </hw-form-item>
      </hw-form>

      <!-- 封禁阶梯编辑器 -->
      <hw-divider align="left">{{ $t('page.hostguard.group_ladder') }}</hw-divider>
      <hw-alert theme="info" :close="false" class="tip-alert">
        <template #message>{{ $t('page.hostguard.ladder_tip') }}</template>
      </hw-alert>

      <hw-table :columns="ladderColumns" :data="ladders" rowKey="level" size="small" :hover="true">
        <template #ban_minutes="{ row, rowIndex }">
          <hw-input-number
            :value="row.ban_minutes"
            :min="0"
            theme="column"
            :style="{ width: '150px' }"
            @change="(v) => onLadderChange(rowIndex, 'ban_minutes', v)"
          />
          <span class="unit">{{ $t('common.unit_minute') }}</span>
          <hw-tag v-if="row.ban_minutes === 0" theme="danger" variant="light" size="small" style="margin-left: 8px">
            {{ $t('page.hostguard.permanent') }}
          </hw-tag>
        </template>
        <template #enable="{ row, rowIndex }">
          <hw-switch
            :value="row.enable"
            :customValue="[1, 0]"
            @change="(v) => onLadderChange(rowIndex, 'enable', v)"
          />
        </template>
        <template #remarks="{ row, rowIndex }">
          <hw-input
            :value="row.remarks"
            :style="{ width: '100%' }"
            @change="(v) => onLadderChange(rowIndex, 'remarks', v)"
          />
        </template>
        <template #op="{ rowIndex }">
          <a class="t-button-link" @click="removeLadder(rowIndex)">{{ $t('common.delete') }}</a>
        </template>
      </hw-table>

      <div class="ladder-ops">
        <hw-button variant="outline" @click="addLadder">{{ $t('page.hostguard.ladder_add') }}</hw-button>
        <hw-button theme="primary" @click="saveLadders" :loading="ladderSaving" style="margin-left: 8px">
          {{ $t('page.hostguard.ladder_save') }}
        </hw-button>
      </div>

      <!-- 自救说明：真出事的时候用户不会来翻文档，直接写在页面上 -->
      <hw-alert theme="warning" :close="false" class="rescue-alert">
        <template #message>
          <div class="rescue-title">{{ $t('page.hostguard.rescue_title') }}</div>
          <ol class="rescue-list">
            <li>{{ $t('page.hostguard.rescue_1') }}</li>
            <li>{{ $t('page.hostguard.rescue_2') }}</li>
            <li>{{ $t('page.hostguard.rescue_3') }}</li>
            <li>
              {{ $t('page.hostguard.rescue_4') }}
              <pre class="code-block">ipset del samwaf_hostguard &lt;IP&gt;
ipset flush samwaf_hostguard
netsh advfirewall firewall delete rule name=SamWAF_Set_samwaf_hostguard_0</pre>
            </li>
          </ol>
        </template>
      </hw-alert>
    </hw-loading>

    <hw-dialog
      :header="$t('page.hostguard.enable_confirm_title')"
      v-model:visible="enableConfirmVisible"
      :width="600"
      @confirm="doEnable"
      @close="cancelEnable"
    >
      <div>
        <p>{{ $t('page.hostguard.enable_confirm_body') }}</p>
        <ul class="exempt-list">
          <li>{{ $t('page.hostguard.enable_confirm_item1') }}</li>
          <li>{{ $t('page.hostguard.enable_confirm_item2') }}</li>
          <li>{{ $t('page.hostguard.enable_confirm_item3') }}</li>
        </ul>
        <p class="exempt-tip">{{ $t('page.hostguard.enable_confirm_tip') }}</p>
      </div>
    </hw-dialog>
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { get_detail_by_item_api, edit_system_config_by_item_api } from '@/apis/systemconfig';
import {
  wafHostGuardLadderListApi,
  wafHostGuardLadderSaveApi,
  wafHostGuardWhitelistTestApi,
} from '@/apis/hostguard';

const MASTER_ITEM = 'host_guard_enabled';

// 逐项读写的配置项清单。分成数字与字符串两类只是为了保存时转换，
// 后端 system_config 的 value 一律按字符串存。
const NUMBER_ITEMS = [
  'host_guard_find_time',
  'host_guard_max_retry',
  'host_guard_offender_reset_day',
  'host_guard_debounce_sec',
  'host_guard_max_ban_entries',
  'host_guard_ban_rate_limit',
  'host_guard_subnet_threshold',
  'host_conn_cache_sec',
];
const STRING_ITEMS = [
  'host_guard_mode',
  'host_guard_count_soft_fail',
  'host_guard_whitelist',
  'host_guard_auto_lan',
  'host_guard_log_paths',
  'host_guard_ssh_ports',
  'host_guard_rdp_ports',
  'host_guard_port_scope',
  'host_guard_exec_mode',
  'host_guard_subnet_aggregate',
  'host_guard_notify',
  'host_conn_enabled',
];

export default defineComponent({
  name: 'HostGuardSettingPanel',
  props: {
    status: { type: Object, default: () => ({}) },
  },
  computed: {
    // 后端明确回 false 才算不支持；status 还没拉回来时不要先把选项禁掉
    portScopeSupported(): boolean {
      return this.status.port_scope_supported !== false;
    },
  },
  data() {
    return {
      loading: false,
      saving: false,
      masterEnabled: false,
      masterLoading: false,
      enableConfirmVisible: false,
      form: {} as any,
      ladders: [],
      ladderSaving: false,
      testIP: '',
      testResult: null as any,
      ladderColumns: [
        { title: this.$t('page.hostguard.col_level'), colKey: 'level', width: 90 },
        { title: this.$t('page.hostguard.col_duration'), colKey: 'ban_minutes', width: 320 },
        { title: this.$t('page.hostguard.col_enable'), colKey: 'enable', width: 100 },
        { title: this.$t('common.remarks'), colKey: 'remarks' },
        { title: this.$t('common.op'), colKey: 'op', width: 90 },
      ],
    };
  },
  mounted() {
    this.refresh();
  },
  methods: {
    refresh() {
      this.loadConfig();
      this.loadLadders();
    },
    loadConfig() {
      this.loading = true;
      const items = [MASTER_ITEM, ...NUMBER_ITEMS, ...STRING_ITEMS];
      const tasks = items.map((item) =>
        get_detail_by_item_api({ item })
          .then((res) => {
            if (res.code === 0 && res.data) {
              return { item, value: res.data.value };
            }
            return null;
          })
          .catch(() => null),
      );
      Promise.all(tasks)
        .then((results) => {
          const form = {} as any;
          results.forEach((r: any) => {
            if (!r) return;
            if (r.item === MASTER_ITEM) {
              this.masterEnabled = String(r.value) === '1';
              return;
            }
            if (NUMBER_ITEMS.indexOf(r.item) >= 0) {
              form[r.item] = Number(r.value) || 0;
            } else {
              form[r.item] = String(r.value ?? '');
            }
          });
          this.form = form;
        })
        .finally(() => {
          this.loading = false;
        });
    },
    onMasterChange(val) {
      if (val) {
        // 开启是危险操作，先确认；先把开关拨回去，确认后再真正打开
        this.masterEnabled = false;
        this.enableConfirmVisible = true;
        return;
      }
      this.applyMaster(false);
    },
    doEnable() {
      this.enableConfirmVisible = false;
      this.applyMaster(true);
    },
    cancelEnable() {
      this.enableConfirmVisible = false;
      this.masterEnabled = false;
    },
    applyMaster(on) {
      this.masterLoading = true;
      edit_system_config_by_item_api({ item: MASTER_ITEM, value: on ? '1' : '0' })
        .then((res) => {
          if (res.code === 0) {
            this.masterEnabled = on;
            this.$message.success(res.msg);
            this.$emit('changed');
          } else {
            this.$message.warning(res.msg);
            // 失败要回读，别让界面和库不一致
            this.loadConfig();
          }
        })
        .catch(() => {
          this.loadConfig();
        })
        .finally(() => {
          this.masterLoading = false;
        });
    },
    onSave() {
      this.saving = true;
      const tasks = [];
      Object.keys(this.form).forEach((item) => {
        tasks.push(edit_system_config_by_item_api({ item, value: String(this.form[item]) }).catch(() => null));
      });
      Promise.all(tasks)
        .then(() => {
          this.$message.success(this.$t('page.hostguard.save_success'));
          this.$emit('changed');
        })
        .finally(() => {
          this.saving = false;
        });
    },
    loadLadders() {
      wafHostGuardLadderListApi({})
        .then((res) => {
          if (res.code === 0) {
            this.ladders = res.data || [];
          }
        })
        .catch(() => {});
    },
    onLadderChange(idx, field, value) {
      const list = [...this.ladders];
      list[idx] = { ...list[idx], [field]: value };
      this.ladders = list;
    },
    addLadder() {
      const nextLevel = this.ladders.length > 0 ? this.ladders[this.ladders.length - 1].level + 1 : 1;
      this.ladders = [...this.ladders, { level: nextLevel, ban_minutes: 60, enable: 1, remarks: '' }];
    },
    removeLadder(idx) {
      const list = [...this.ladders];
      list.splice(idx, 1);
      // 删完重新编号，保证级别连续
      this.ladders = list.map((x, i) => ({ ...x, level: i + 1 }));
    },
    saveLadders() {
      this.ladderSaving = true;
      wafHostGuardLadderSaveApi({ ladders: this.ladders })
        .then((res) => {
          if (res.code === 0) {
            this.$message.success(res.msg);
            this.loadLadders();
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch(() => {})
        .finally(() => {
          this.ladderSaving = false;
        });
    },
    onTestWhitelist() {
      if (!this.testIP) {
        this.$message.warning(this.$t('page.hostguard.whitelist_test_ph'));
        return;
      }
      wafHostGuardWhitelistTestApi({ ip: this.testIP })
        .then((res) => {
          if (res.code === 0) {
            this.testResult = res.data;
          } else {
            this.$message.warning(res.msg);
          }
        })
        .catch(() => {});
    },
  },
});
</script>

<style lang="less" scoped>
.panel-container {
  padding: 8px 0;
}
.master-card {
  margin-bottom: 20px;
}
.master-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.master-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}
.cfg-form {
  max-width: 900px;
}
// TDesign 的 .t-form__controls-content 是 flex row，说明文字会和控件抢同一行；
// 控件被压窄后，radio-button 组就换行竖排了（"封禁范围"最明显）。
// 让内容区允许换行、并把说明独占整行，控件才能拿到完整宽度保持并排。
:deep(.t-form__controls-content) {
  flex-wrap: wrap;
  align-items: center;
}
:deep(.t-radio-group) {
  flex-shrink: 0;
}
.desc {
  flex-basis: 100%;
  width: 100%;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  margin-top: 4px;
  line-height: 1.6;
  &.warn-desc {
    color: var(--td-warning-color-7);
  }
}
.unit {
  margin-left: 8px;
}
.unit-text {
  margin-left: 12px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
.test-result {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.test-reason {
  font-size: 13px;
  color: var(--td-text-color-secondary);
}
.tip-alert {
  margin-bottom: 16px;
}
.ladder-ops {
  margin-top: 16px;
}
.rescue-alert {
  margin-top: 24px;
}
.rescue-title {
  font-weight: 600;
  margin-bottom: 8px;
}
.rescue-list {
  margin: 0;
  padding-left: 20px;
  li {
    margin-bottom: 8px;
    font-size: 13px;
    line-height: 1.7;
  }
}
.code-block {
  background: var(--td-bg-color-component);
  border: 1px solid var(--td-border-level-1-color);
  border-radius: 3px;
  padding: 10px;
  margin: 6px 0 0 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 12px;
  line-height: 1.6;
  overflow-x: auto;
}
.exempt-list {
  padding-left: 20px;
  li {
    margin-bottom: 6px;
    font-size: 13px;
  }
}
.exempt-tip {
  color: var(--td-warning-color-7);
  font-size: 13px;
  margin-top: 8px;
}
</style>
