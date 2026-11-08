<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization. -->

<template>
  <div class="sp-page">
    <!-- 查询条件 -->
    <hw-card class="sp-card">
      <div class="sp-bar">
        <span class="sp-lb">{{ $t('page.source_path.label_day') }}</span>
        <hw-date-picker v-model="day" :style="{ width: '150px' }" @change="reload" />
        <span class="sp-lb">{{ $t('page.source_path.label_host') }}</span>
        <hw-select v-model="hostCode" clearable filterable :style="{ width: '200px' }"
                  :placeholder="$t('page.source_path.all_hosts')" @change="reload">
          <hw-option v-for="item in hostOptions" :key="item.value" :value="item.value" :label="item.label" />
        </hw-select>
        <span class="sp-lb">Top</span>
        <hw-select v-model="limit" :style="{ width: '90px' }" @change="reload">
          <hw-option v-for="n in [20, 50, 100]" :key="n" :value="n" :label="String(n)" />
        </hw-select>
        <hw-button theme="primary" :loading="loading" @click="reload">{{ $t('common.search') }}</hw-button>
        <span class="sp-flex"></span>
        <span class="sp-tip">{{ $t('page.source_path.day_level_tip') }}</span>
      </div>
    </hw-card>

    <!-- 判定阈值：就地改就地存，与访问日志页的「日志配置」卡同款 -->
    <hw-card class="sp-card">
      <div class="sp-bar">
        <span class="sp-lb"><b>{{ $t('page.source_path.label_threshold') }}</b></span>
        <span class="sp-lb">{{ $t('page.source_path.th_scan_prefix') }}</span>
        <hw-input-number v-model="thScan" :min="1" :max="100000" theme="column" :style="{ width: '110px' }" />
        <span class="sp-lb">{{ $t('page.source_path.th_scan_suffix') }}</span>
        <span class="sp-gap"></span>
        <span class="sp-lb">{{ $t('page.source_path.th_ua_prefix') }}</span>
        <hw-input-number v-model="thUa" :min="1" :max="10000" theme="column" :style="{ width: '110px' }" />
        <span class="sp-lb">{{ $t('page.source_path.th_ua_suffix') }}</span>
        <hw-button variant="outline" size="small" :loading="thSaving" @click="saveThreshold">
          {{ $t('page.source_path.save_as_default') }}
        </hw-button>
        <span class="sp-flex"></span>
        <span class="sp-tip">{{ $t('page.source_path.threshold_tip') }}</span>
      </div>
    </hw-card>

    <!-- 摘要 -->
    <div class="sp-kpis">
      <hw-card class="sp-kpi">
        <div class="k">{{ $t('page.source_path.kpi_actor') }}</div>
        <div class="v">{{ actorResp.total_actor || 0 }}</div>
        <div class="d">{{ $t('page.source_path.kpi_actor_desc') }}</div>
      </hw-card>
      <hw-card class="sp-kpi">
        <div class="k">{{ $t('page.source_path.kpi_path') }}</div>
        <div class="v">{{ pathResp.total_path || 0 }}</div>
        <div class="d">{{ $t('page.source_path.kpi_path_desc') }}</div>
      </hw-card>
      <hw-card class="sp-kpi sp-clickable" :class="{ on: kpiFilter === 'scan' }" @click="toggleKpi('scan')">
        <div class="k">{{ $t('page.source_path.kpi_scan') }}
          <hw-tag size="small" theme="warning" variant="light">{{ $t('page.source_path.click_filter') }}</hw-tag>
        </div>
        <div class="v warn">{{ actorResp.scan_actor || 0 }}</div>
        <div class="d">{{ $t('page.source_path.kpi_scan_desc', { n: actorResp.scan_threshold || thScan }) }}</div>
      </hw-card>
      <hw-card class="sp-kpi sp-clickable" :class="{ on: kpiFilter === 'ua' }" @click="toggleKpi('ua')">
        <div class="k">{{ $t('page.source_path.kpi_ua') }}
          <hw-tag size="small" theme="warning" variant="light">{{ $t('page.source_path.click_filter') }}</hw-tag>
        </div>
        <div class="v warn">{{ actorResp.ua_actor || 0 }}</div>
        <div class="d">{{ $t('page.source_path.kpi_ua_desc', { n: actorResp.ua_threshold || thUa }) }}</div>
      </hw-card>
    </div>

    <!-- 行为视角 -->
    <hw-card class="sp-card" :title="$t('page.source_path.actor_title')" :subtitle="$t('page.source_path.actor_sub')">
      <hw-table :data="actorRows" :columns="actorColumns" row-key="actor_key" size="small"
               :loading="loading" :sort="actorSort" @sort-change="onActorSort" hover>
        <template #actor_key="{ row }">
          <hw-link theme="primary" hover="color" @click="openDrawer('actor', row.actor_key)">{{ row.actor_key }}</hw-link>
        </template>
        <template #distinct_path="{ row }">
          <span :class="{ 'sp-danger': row.distinct_path >= (actorResp.scan_threshold || thScan) }">{{ row.distinct_path }}</span>
        </template>
        <template #ua_cnt="{ row }">
          <span :class="{ 'sp-warn': row.ua_cnt >= (actorResp.ua_threshold || thUa) }">{{ row.ua_cnt }}</span>
        </template>
        <template #verdict="{ row }">
          <hw-tag v-if="row.distinct_path >= (actorResp.scan_threshold || thScan)" theme="danger" variant="light" size="small">
            {{ $t('page.source_path.verdict_scan') }}
          </hw-tag>
          <hw-tag v-else-if="row.ua_cnt >= (actorResp.ua_threshold || thUa)" theme="warning" variant="light" size="small">
            {{ $t('page.source_path.verdict_ua') }}
          </hw-tag>
          <hw-tag v-else-if="row.deny_cnt > 0" theme="warning" variant="light" size="small">
            {{ $t('page.source_path.verdict_denied') }}
          </hw-tag>
          <span v-else class="sp-muted">—</span>
        </template>
        <template #op="{ row }">
          <hw-space size="small">
            <hw-link theme="danger" hover="color" @click="disposal(row.actor_key, 'block')">{{ $t('page.source_path.op_block') }}</hw-link>
            <hw-link theme="primary" hover="color" @click="disposal(row.actor_key, 'allow')">{{ $t('page.source_path.op_allow') }}</hw-link>
          </hw-space>
        </template>
      </hw-table>
    </hw-card>

    <!-- 目标视角 -->
    <hw-card class="sp-card" :title="$t('page.source_path.path_title')" :subtitle="$t('page.source_path.path_sub')">
      <hw-table :data="pathResp.list || []" :columns="pathColumns" row-key="path_norm" size="small"
               :loading="loading" :sort="pathSort" @sort-change="onPathSort" hover>
        <template #path_norm="{ row }">
          <hw-link theme="primary" hover="color" @click="openDrawer('path', row.path_norm)">{{ row.path_norm }}</hw-link>
          <hw-tag v-if="row.path_norm === '{overflow}'" theme="warning" variant="light" size="small" style="margin-left:6px">
            {{ $t('page.source_path.overflow_tip') }}
          </hw-tag>
        </template>
        <template #top_rule="{ row }">
          <hw-tag v-if="row.top_rule" theme="danger" variant="light" size="small">{{ row.top_rule }}</hw-tag>
          <span v-else class="sp-muted">—</span>
        </template>
      </hw-table>
    </hw-card>

    <!-- 下钻抽屉 -->
    <hw-drawer v-model:visible="drawerVisible" :header="drawerTitle" size="640px" :footer="false">
      <div v-if="detail">
        <hw-descriptions :column="3" size="small" bordered>
          <hw-descriptions-item :label="$t('page.source_path.d_req')">{{ detail.req_cnt }}</hw-descriptions-item>
          <hw-descriptions-item :label="$t('page.source_path.d_deny')">{{ detail.deny_cnt }}</hw-descriptions-item>
          <hw-descriptions-item :label="$t('page.source_path.d_err4xx')">{{ detail.err4xx_cnt }}</hw-descriptions-item>
        </hw-descriptions>

        <template v-if="detail.kind === 'actor'">
          <div class="sp-sect">{{ $t('page.source_path.d_paths') }}</div>
          <hw-table :data="detail.paths || []" :columns="dPathCols" row-key="path_norm" size="small" />
          <div class="sp-sect">{{ $t('page.source_path.d_uas') }}</div>
          <hw-table :data="detail.uas || []" :columns="dUaCols" row-key="ua_hash" size="small" />
          <div class="sp-sect">{{ $t('page.source_path.d_hosts') }}
            <span class="sp-tip">{{ $t('page.source_path.d_hosts_tip') }}</span>
          </div>
          <hw-table :data="detail.hosts || []" :columns="dHostCols" row-key="host_code" size="small">
            <template #host_code="{ row }">{{ hostName(row.host_code) }}</template>
          </hw-table>

          <div class="sp-sect">{{ $t('page.source_path.d_actions') }}</div>
          <hw-space break-line>
            <hw-button theme="danger" @click="disposal(detail.key, 'block')">{{ $t('page.source_path.op_block') }}</hw-button>
            <hw-button variant="outline" @click="disposal(detail.key, 'allow')">{{ $t('page.source_path.op_allow') }}</hw-button>
            <hw-button variant="outline" @click="gotoLog(detail.key)">{{ $t('page.source_path.op_log') }}</hw-button>
          </hw-space>
        </template>

        <template v-else>
          <div class="sp-sect">{{ $t('page.source_path.d_actors') }}</div>
          <hw-table :data="detail.actors || []" :columns="dActorCols" row-key="actor_key" size="small" />
          <div class="sp-sect">{{ $t('page.source_path.d_rules') }}</div>
          <hw-table :data="detail.rules || []" :columns="dRuleCols" row-key="rule" size="small" />
        </template>
      </div>
    </hw-drawer>

    <ip-disposal v-model:visible="disposalVisible" :ip="disposalIp" :mode="disposalMode"
                 :host-candidates="disposalHosts" :prefer-host-code="hostCode"
                 :remarks="disposalRemarks" @success="reload" />
  </div>
</template>

<script lang="ts">
import {defineComponent} from 'vue';


import { analysisActorList, analysisPathList, analysisDetail } from '@/apis/analysis_view';
import { allhost } from '@/apis/host';
import { get_detail_by_item_api, edit_system_config_api } from '@/apis/systemconfig';
import IpDisposal from '@/components/ip-disposal/index.vue';

const TH_SCAN = 'analysis_scan_path_threshold';
const TH_UA = 'analysis_ua_threshold';

// 来源与路径分析（M5/G3）。
// 两张表都从 stats_actor_path_days 来，只是 GROUP BY 的维度不同：
// 按 actor_key 出「谁在打」，按 path_norm 出「打哪里」。
// 去重数是服务端 COUNT(DISTINCT) 现算的，库里没这两列。
export default defineComponent({
  name: 'SourcePathAnalysis',
  components: { IpDisposal },
  data() {
    return {
      loading: false,
      day: this.todayStr(),
      hostCode: '',
      limit: 20,
      hostOptions: [] as Array<{ value: string; label: string }>,
      hostDic: {} as Record<string, string>,

      thScan: 20,
      thUa: 5,
      thItems: {} as Record<string, any>,
      thSaving: false,

      actorResp: {} as any,
      pathResp: {} as any,
      actorSort: { sortBy: 'distinct_path', descending: true },
      pathSort: { sortBy: 'req_cnt', descending: true },
      kpiFilter: '',

      drawerVisible: false,
      detail: null as any,

      disposalVisible: false,
      disposalIp: '',
      disposalMode: 'block',
      disposalHosts: [] as any[],
      disposalRemarks: '',
    };
  },
  computed: {
    // KPI 点击后只是把当前页的行过滤一下，不再往服务端跑一趟
    actorRows(): any[] {
      const list = (this.actorResp.list || []) as any[];
      if (this.kpiFilter === 'scan') {
        return list.filter((r) => r.distinct_path >= (this.actorResp.scan_threshold || this.thScan));
      }
      if (this.kpiFilter === 'ua') {
        return list.filter((r) => r.ua_cnt >= (this.actorResp.ua_threshold || this.thUa));
      }
      return list;
    },
    drawerTitle(): string {
      if (!this.detail) return '';
      const t = this.detail.kind === 'actor'
        ? this.$t('page.source_path.drawer_actor')
        : this.$t('page.source_path.drawer_path');
      return `${t} · ${this.detail.key}`;
    },
    actorColumns(): any[] {
      const t = (k: string) => this.$t(`page.source_path.col_${k}`);
      return [
        { colKey: 'actor_key', title: t('actor'), width: 190 },
        { colKey: 'distinct_path', title: t('distinct_path'), width: 120, sorter: true },
        { colKey: 'req_cnt', title: t('req'), width: 100, sorter: true },
        { colKey: 'deny_cnt', title: t('deny'), width: 100, sorter: true },
        { colKey: 'err4xx_cnt', title: t('err4xx'), width: 110, sorter: true },
        { colKey: 'ua_cnt', title: t('ua'), width: 90 },
        { colKey: 'verdict', title: t('verdict'), width: 130 },
        { colKey: 'op', title: t('op'), width: 130 },
      ];
    },
    pathColumns(): any[] {
      const t = (k: string) => this.$t(`page.source_path.col_${k}`);
      return [
        { colKey: 'path_norm', title: t('path'), minWidth: 260 },
        { colKey: 'req_cnt', title: t('req'), width: 110, sorter: true },
        { colKey: 'deny_cnt', title: t('deny'), width: 110, sorter: true },
        { colKey: 'err4xx_cnt', title: t('err4xx'), width: 120, sorter: true },
        { colKey: 'distinct_actor', title: t('distinct_actor'), width: 120, sorter: true },
        { colKey: 'top_rule', title: t('top_rule'), width: 170 },
      ];
    },
    dPathCols(): any[] {
      const t = (k: string) => this.$t(`page.source_path.col_${k}`);
      return [
        { colKey: 'path_norm', title: t('path'), minWidth: 220 },
        { colKey: 'req_cnt', title: t('req'), width: 80 },
        { colKey: 'deny_cnt', title: t('deny'), width: 80 },
        { colKey: 'err4xx_cnt', title: t('err4xx'), width: 90 },
      ];
    },
    dActorCols(): any[] {
      const t = (k: string) => this.$t(`page.source_path.col_${k}`);
      return [
        { colKey: 'actor_key', title: t('actor'), minWidth: 200 },
        { colKey: 'req_cnt', title: t('req'), width: 80 },
        { colKey: 'deny_cnt', title: t('deny'), width: 80 },
        { colKey: 'err4xx_cnt', title: t('err4xx'), width: 90 },
      ];
    },
    dUaCols(): any[] {
      return [
        { colKey: 'ua_hash', title: this.$t('page.source_path.col_ua_hash'), minWidth: 200 },
        { colKey: 'cnt', title: this.$t('page.source_path.col_req'), width: 90 },
      ];
    },
    dHostCols(): any[] {
      const t = (k: string) => this.$t(`page.source_path.col_${k}`);
      return [
        { colKey: 'host_code', title: t('host'), minWidth: 200 },
        { colKey: 'req_cnt', title: t('req'), width: 90 },
        { colKey: 'deny_cnt', title: t('deny'), width: 90 },
      ];
    },
    dRuleCols(): any[] {
      return [
        { colKey: 'rule', title: this.$t('page.source_path.col_rule'), minWidth: 200 },
        { colKey: 'cnt', title: this.$t('page.source_path.col_req'), width: 90 },
      ];
    },
  },
  created() {
    this.loadHosts();
    this.loadThreshold();
    this.reload();
  },
  methods: {
    todayStr(): string {
      const d = new Date();
      const p = (n: number) => String(n).padStart(2, '0');
      return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
    },
    // 后端按 int 的 yyyymmdd 存 day，界面用日期控件，这里做一次换算
    dayInt(): number {
      return parseInt(String(this.day).replace(/-/g, ''), 10) || 0;
    },
    hostName(code: string): string {
      return this.hostDic[code] || code;
    },
    baseParams(): any {
      return { day: this.dayInt(), host_code: this.hostCode || '', limit: this.limit };
    },
    reload() {
      this.loading = true;
      const actorReq = analysisActorList({
        ...this.baseParams(),
        sort_by: this.actorSort.sortBy,
      }).then((res) => {
        if (res.code === 0) this.actorResp = res.data || {};
      });
      const pathReq = analysisPathList({
        ...this.baseParams(),
        sort_by: this.pathSort.sortBy,
      }).then((res) => {
        if (res.code === 0) this.pathResp = res.data || {};
      });
      Promise.all([actorReq, pathReq])
        .catch(() => {})
        .finally(() => {
          this.loading = false;
        });
    },
    // 排序走服务端：客户端排只能排当前这一页，Top N 就没意义了
    onActorSort(val: any) {
      if (!val) return;
      this.actorSort = { sortBy: val.sortBy, descending: true };
      this.reload();
    },
    onPathSort(val: any) {
      if (!val) return;
      this.pathSort = { sortBy: val.sortBy, descending: true };
      this.reload();
    },
    toggleKpi(k: string) {
      this.kpiFilter = this.kpiFilter === k ? '' : k;
    },
    loadHosts() {
      allhost()
        .then((res) => {
          if (res.code === 0) {
            // allhost 的 json 字段就叫 value / label（model/response/all_host.go 的 tag），
            // 不是 code / host——取错的话下拉全是空白项
            const list = (res.data || []).filter((h: any) => h && h.value);
            this.hostOptions = list.map((h: any) => ({ value: h.value, label: h.label || h.value }));
            const dic: Record<string, string> = {};
            list.forEach((h: any) => {
              dic[h.value] = h.label || h.value;
            });
            this.hostDic = dic;
          }
        })
        .catch(() => {});
    },
    loadThreshold() {
      [TH_SCAN, TH_UA].forEach((key) => {
        get_detail_by_item_api({ item: key })
          .then((res) => {
            if (res.code === 0 && res.data) {
              this.thItems[key] = res.data;
              const v = parseInt(res.data.value, 10);
              if (!Number.isNaN(v)) {
                if (key === TH_SCAN) this.thScan = v;
                else this.thUa = v;
              }
            }
          })
          .catch(() => {});
      });
    },
    saveThreshold() {
      this.thSaving = true;
      const save = (key: string, value: number) => {
        const item = this.thItems[key];
        if (!item) return Promise.resolve(undefined);
        return edit_system_config_api({
          id: item.id,
          category: item.category,
          item: item.item,
          value: String(value),
          type: item.type,
          title: item.title,
          options: item.options || '',
        });
      };
      Promise.all([save(TH_SCAN, this.thScan), save(TH_UA, this.thUa)])
        .then(() => {
          this.$message.success(this.$t('common.tips.save_success'));
          this.reload();
        })
        .catch(() => {
          this.$message.error(this.$t('page.source_path.save_failed'));
        })
        .finally(() => {
          this.thSaving = false;
        });
    },
    openDrawer(kind: string, key: string) {
      analysisDetail({ ...this.baseParams(), kind, key })
        .then((res) => {
          if (res.code === 0) {
            this.detail = res.data;
            this.drawerVisible = true;
          } else {
            this.$message.error(res.msg);
          }
        })
        .catch(() => {});
    },
    // actor_key 形如 ip:1.2.3.4，处置接口要的是裸 IP
    toIp(actorKey: string): string {
      return String(actorKey || '').replace(/^ip:/, '');
    },
    disposal(actorKey: string, mode: string) {
      this.disposalIp = this.toIp(actorKey);
      this.disposalMode = mode;
      // 站点候选优先用抽屉里查到的；直接从表格点的话回落到当前筛选
      const fromDetail = this.detail && this.detail.kind === 'actor' && this.detail.key === actorKey
        ? this.detail.hosts || []
        : [];
      this.disposalHosts = fromDetail;
      const row = (this.actorResp.list || []).find((r: any) => r.actor_key === actorKey);
      const why = row
        ? this.$t('page.source_path.remark_tpl', { day: this.day, path: row.distinct_path, req: row.req_cnt })
        : this.$t('page.source_path.remark_plain', { day: this.day });
      this.disposalRemarks = String(why);
      this.disposalVisible = true;
    },
    gotoLog(actorKey: string) {
      this.$router.push({ path: '/waf/wafattacklog', query: { src_ip: this.toIp(actorKey) } });
    },
  },
});
</script>

<style lang="less" scoped>
.sp-card {
  margin-bottom: 14px;
}
.sp-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.sp-lb {
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
.sp-tip {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
.sp-flex {
  flex: 1;
}
.sp-gap {
  width: 16px;
}
.sp-kpis {
  display: flex;
  gap: 14px;
  margin-bottom: 14px;
}
.sp-kpi {
  flex: 1;
  .k {
    color: var(--td-text-color-secondary);
    font-size: 13px;
  }
  .v {
    font-size: 26px;
    font-weight: 600;
    margin-top: 6px;
    line-height: 1.2;
    &.warn {
      color: var(--td-warning-color);
    }
  }
  .d {
    color: var(--td-text-color-placeholder);
    font-size: 12px;
    margin-top: 2px;
  }
}
.sp-clickable {
  cursor: pointer;
  &.on {
    outline: 1px solid var(--td-brand-color);
  }
}
.sp-sect {
  font-size: 13px;
  color: var(--td-text-color-secondary);
  margin: 16px 0 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--td-component-stroke);
}
.sp-danger {
  color: var(--td-error-color);
  font-weight: 600;
}
.sp-warn {
  color: var(--td-warning-color);
  font-weight: 600;
}
.sp-muted {
  color: var(--td-text-color-placeholder);
}
</style>
