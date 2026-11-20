<!-- HomeWaf modification (2026-11-09): independent distribution; see NOTICE. -->

<template>
  <div class="visit-log-page" data-testid="visit-log-page">
    <hw-card class="list-card-container visit-log-card resource-panel">
      <hw-form
        ref="form"
        :data="searchformData"
        class="visit-filter-form"
        @submit="getList('all')"
        @reset="resetSearch"
      >
        <div class="visit-filter-grid">
          <hw-form-item :label="text('服务', 'Service')" name="host_code">
            <hw-select
              v-model="searchformData.host_code"
              clearable
              filterable
              :loading="hostLoading"
              :placeholder="text('全部服务', 'All services')"
              data-testid="log-service-filter"
            >
              <hw-option v-for="item in serviceOptions" :key="item.value" :value="item.value" :label="item.label" />
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="text('时间范围', 'Time range')">
            <hw-select v-model="timePreset" :disabled="isUuidLookup" @change="changeTimePreset">
              <hw-option value="today" :label="text('今天', 'Today')" />
              <hw-option value="24h" :label="text('最近 24 小时', 'Last 24 hours')" />
              <hw-option value="7d" :label="text('最近 7 天', 'Last 7 days')" />
              <hw-option value="custom" :label="text('自定义时间', 'Custom range')" />
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="text('访问情况', 'Result')" name="request_outcome">
            <hw-select
              :model-value="isUuidLookup ? '' : searchformData.request_outcome"
              @update:model-value="searchformData.request_outcome = $event"
              :disabled="isUuidLookup"
              :placeholder="text('全部访问', 'All requests')"
            >
              <hw-option value="" :label="text('全部访问', 'All requests')" />
              <hw-option value="normal" :label="text('正常', 'Normal')" />
              <hw-option value="observed" :label="text('仅观察', 'Observed')" />
              <hw-option value="abnormal" :label="text('异常', 'Abnormal')" />
              <hw-option value="blocked" :label="text('已拦截', 'Blocked')" />
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="text('来源 IP', 'Source IP')" name="src_ip">
            <hw-input v-model="searchformData.src_ip" clearable :placeholder="text('输入 IP 地址', 'IP address')" />
          </hw-form-item>
          <hw-form-item :label="text('访问路径', 'Request path')" name="url_keyword">
            <hw-input
              v-model="searchformData.url_keyword"
              clearable
              :maxlength="2048"
              :placeholder="text('例如 /photos', 'For example /photos')"
            />
          </hw-form-item>
        </div>
        <div v-if="timePreset === 'custom'" class="visit-custom-range">
          <hw-form-item :label="text('起止时间', 'Start and end time')">
            <hw-date-range-picker
              v-model="dateControl.range1"
              :presets="dateControl.presets"
              :enable-time-picker="true"
              :disabled="isUuidLookup"
              value-type="YYYY-MM-DD HH:mm:ss"
            />
          </hw-form-item>
        </div>
        <div v-show="advancedVisible" id="visit-advanced-filters" class="visit-filter-grid visit-advanced">
          <hw-form-item :label="text('命中规则', 'Matched rule')" name="rule"
            ><hw-input v-model="searchformData.rule" clearable
          /></hw-form-item>
          <hw-form-item :label="text('请求编号', 'Request ID')" name="req_uuid"
            ><hw-input v-model="searchformData.req_uuid" clearable :maxlength="64"
          /></hw-form-item>
          <hw-form-item :label="text('防护处置', 'Protection action')" name="action"
            ><hw-select v-model="searchformData.action" :options="action_options"
          /></hw-form-item>
          <hw-form-item :label="text('HTTP 响应码', 'HTTP status')" name="status_code"
            ><hw-input v-model="searchformData.status_code" clearable placeholder="200 / 403 / 500"
          /></hw-form-item>
          <hw-form-item :label="text('请求方法', 'Method')" name="method"
            ><hw-select v-model="searchformData.method" :options="method_options"
          /></hw-form-item>
          <hw-form-item :label="text('日志范围', 'Log source')" name="view_type">
            <hw-select v-model="searchformData.view_type" @change="onViewTypeChange">
              <hw-option value="access" :label="text('全部访问记录', 'All access records')" />
              <hw-option value="event" :label="text('仅安全事件', 'Security events only')" />
            </hw-select>
          </hw-form-item>
          <hw-form-item :label="text('归档分区', 'Archive')" name="current_db_name">
            <hw-select v-model="searchformData.current_db_name" :disabled="isUuidLookup">
              <hw-option value="auto" :label="$t('page.visit_log.shard_auto')" />
              <hw-option-group v-for="g in shardGroups" :key="g.label" :label="g.label"
                ><hw-option
                  v-for="it in g.options"
                  :key="it.value"
                  :value="it.value"
                  :label="it.label"
                  :disabled="it.disabled"
              /></hw-option-group>
            </hw-select>
          </hw-form-item>
          <div class="visit-support">
            <ip-lookup ref="ipLookup" />
            <hw-button variant="text" @click="handleIPExtractIssue">{{
              text('排查来源 IP', 'Diagnose source IP')
            }}</hw-button>
          </div>
        </div>
        <div class="visit-search-actions">
          <hw-button theme="primary" type="submit" :loading="dataLoading">{{ $t('common.search') }}</hw-button>
          <hw-button type="button" variant="outline" @click="resetSearch">{{ $t('common.reset') }}</hw-button>
          <hw-button
            variant="text"
            :aria-expanded="advancedVisible"
            aria-controls="visit-advanced-filters"
            @click="advancedVisible = !advancedVisible"
          >
            {{ advancedVisible ? text('收起高级筛选', 'Hide advanced filters') : text('高级筛选', 'Advanced filters')
            }}<span v-if="advancedFilterCount"> ({{ advancedFilterCount }})</span>
          </hw-button>
        </div>
      </hw-form>
      <hw-alert v-if="hostError" theme="warning" :message="hostError" class="visit-notice"
        ><hw-button variant="text" @click="loadHostList">{{
          text('重新读取服务', 'Reload services')
        }}</hw-button></hw-alert
      >
      <div class="visit-results-heading">
        <div>
          <h2>
            {{ text('访问记录', 'Access records') }} <span class="visit-result-count">{{ pagination.total }}</span>
          </h2>
          <p>
            {{
              text(
                '仅观察表示记录了检测线索且请求已放行，不代表确认攻击。拦截、非观察规则命中或响应失败归为异常。',
                'Observed requests were allowed with detection records; this does not confirm an attack. Blocks, enforced matches and failed responses are abnormal.',
              )
            }}
          </p>
        </div>
        <PageActions :inline="attack_ip !== ''"><div class="visit-result-actions">
          <hw-button
            v-if="attack_ip === '' && $route.name === 'WafvisitLog'"
            variant="outline"
            @click="$refs.logExport.open()"
            >{{ $t('logExport.title') }}</hw-button
          >
          <hw-button variant="outline" :loading="columnConfigPending" @click="openColumnDialog">{{ text('显示列', 'Columns') }}</hw-button>
          <hw-button v-if="attack_ip === ''" variant="text" @click="logConfigVisible = true">{{
            text('日志设置', 'Log settings')
          }}</hw-button>
        </div></PageActions>
      </div>
      <div class="table-container">
        <hw-alert v-if="queryError" theme="error" :message="queryError" class="visit-notice"
          ><hw-button variant="text" @click="getList('')">{{ text('重试', 'Retry') }}</hw-button></hw-alert
        >
        <hw-alert
          v-if="isUuidLookup"
          theme="info"
          style="margin-bottom: 12px"
          :message="$t('page.visit_log.uuid_filters_tip')"
        />
        <!-- 查询超出后端预算：给出「能怎么办」，而不是让人对着不完整的数字猜 -->
        <hw-alert v-if="queryMeta.partial" theme="warning" style="margin-bottom: 12px">
          <template #message>
            {{ $t('page.visit_log.query_partial', { ms: queryMeta.took_ms }) }}
            <button type="button" class="t-button-link" style="margin-left: 8px" @click="narrowRange">
              {{ $t('page.visit_log.query_partial_narrow') }}
            </button>
          </template>
        </hw-alert>

        <!-- 分区登记还在、存储却不在了（或文件里一行都没有）：说清楚是哪个，给出处理入口；
             缺失分区一多整屏都是告警，超过 5 条默认折叠成一行概括，要看再展开 -->
        <hw-alert v-if="queryMeta.issues && queryMeta.issues.length > 0" theme="warning" style="margin-bottom: 12px">
          <template #message>
            <template v-if="queryMeta.issues.length > 5 && !issExpanded">
              {{ $t('page.visit_log.shard_issue_collapsed', { n: queryMeta.issues.length }) }}
              <button type="button" class="t-button-link" @click="issExpanded = true">
                {{ $t('page.visit_log.shard_issue_expand') }}
              </button>
            </template>
            <template v-else>
              <div>{{ $t('page.visit_log.shard_issue_title') }}</div>
              <div v-for="iss in queryMeta.issues" :key="iss.name" :title="iss.name" style="margin-top: 2px">
                {{ shardTitle(iss.name) }}：{{
                  iss.kind === 'missing'
                    ? $t('page.visit_log.shard_issue_missing', { n: iss.registered })
                    : $t('page.visit_log.shard_issue_empty', { n: iss.registered })
                }}
              </div>
              <button
                type="button"
                v-if="queryMeta.issues.length > 5"
                class="t-button-link"
                @click="issExpanded = false"
              >
                {{ $t('page.visit_log.shard_cover_fold') }}
              </button>
            </template>
            <button type="button" v-if="attack_ip == ''" class="t-button-link" @click="openShardManage">
              {{ $t('page.visit_log.shard_issue_manage') }}
            </button>
          </template>
        </hw-alert>

        <!-- 这次查了哪些分区：自动模式下用户没选分区，界面得说明数据从哪来 -->
        <hw-alert
          v-if="queryMeta.uuid_lookup && queryMeta.found_in"
          theme="success"
          style="margin-bottom: 12px"
          :message="$t('page.visit_log.uuid_found', { shard: shardTitle(queryMeta.found_in), n: queryMeta.scanned })"
        />
        <hw-alert v-else-if="queryMeta.uuid_lookup" theme="warning" style="margin-bottom: 12px">
          <template #message>
            <div>{{ $t('page.visit_log.uuid_miss', { n: queryMeta.scanned }) }}</div>
            <ul style="margin: 6px 0 0 18px; padding: 0">
              <li>{{ $t('page.visit_log.uuid_miss_r1') }}</li>
              <li>{{ $t('page.visit_log.uuid_miss_r2') }}</li>
              <li>{{ $t('page.visit_log.uuid_miss_r3') }}</li>
            </ul>
          </template>
        </hw-alert>
        <hw-alert v-else-if="queryMeta.shards && queryMeta.shards.length > 1" theme="info" style="margin-bottom: 12px">
          <template #message>
            <!-- 分区多的时候全列出来就是一大堆，默认只给一行概括，要看再展开 -->
            {{ $t('page.visit_log.shard_cover', { n: queryMeta.shards.length }) }}
            <span style="color: var(--hw-muted)">{{ shardCoverSummary }}</span>
            <button type="button" class="t-button-link" @click="covExpanded = !covExpanded">
              {{ covExpanded ? $t('page.visit_log.shard_cover_fold') : $t('page.visit_log.shard_cover_detail') }}
            </button>
            <span v-if="queryMeta.sort_forced_time" style="color: var(--hw-muted)">
              {{ $t('page.visit_log.sort_forced_time') }}
            </span>
            <div v-show="covExpanded" style="margin-top: 6px">
              <span
                v-for="sh in queryMeta.shards"
                :key="sh.name"
                :title="sh.name"
                style="
                  display: inline-block;
                  margin: 2px 6px 2px 0;
                  padding: 0 8px;
                  background: var(--hw-subtle);
                  border-radius: 2px;
                "
              >
                {{ shardTitle(sh.name) }} · {{ sh.count }}
              </span>
            </div>
          </template>
        </hw-alert>

        <hw-table
          ref="logTable"
          :columns="responsiveColumns"
          :data="data"
          size="small"
          :rowKey="rowKey"
          :verticalAlign="verticalAlign"
          :display-columns="compactView ? ['summary'] : displayColumns"
          :show-header="!compactView"
          :pagination="pagination"
          :selected-row-keys="selectedRowKeys"
          :loading="dataLoading"
          @page-change="rehandlePageChange"
          :sort="sorts"
          @change="rehandleChange"
          @select-change="rehandleSelectChange"
          @sort-change="onSortChange"
          @filter-change="onFilterChange"
          :max-height="compactView ? '' : 640"
        >
          <template #summary="{ row }">
            <dl class="visit-compact-record">
              <template v-for="key in compactColumnKeys" :key="key">
                <dt>{{ columnLabel(key) }}</dt>
                <dd v-if="key === 'outcome'">
                  <hw-tag :theme="outcomeTheme(row)">{{
                    outcomeLabel(row)
                  }}</hw-tag
                  ><span v-if="outcome(row).abnormal || outcome(row).observed" class="visit-compact-reason">{{ outcomeReason(row) }}</span>
                </dd>
                <dd v-else-if="key === 'src_ip'">
                  <button type="button" class="ipl-link" @click="openIpLookup(row.src_ip)">{{ row.src_ip }}</button>
                </dd>
                <dd v-else>{{ compactValue(row, key) }}</dd>
              </template>
            </dl>
            <button type="button" class="t-button-link" @click="handleClickDetail({ row })">
              {{ text('查看详情', 'View details') }}
            </button>
          </template>

          <template #empty>
            <div class="table-empty-state">
              <strong>{{
                dataLoading
                  ? text('正在查询…', 'Loading…')
                  : queryError
                    ? text('暂时无法读取日志', 'Logs unavailable')
                    : text('没有符合条件的访问', 'No matching requests')
              }}</strong>
              <p v-if="!dataLoading && !queryError">
                {{
                  text(
                    '调整时间或筛选条件；新服务收到访问后才会产生记录。',
                    'Adjust the time range or filters. New services need traffic before logs appear.',
                  )
                }}
              </p>
            </div>
          </template>
          <template #outcome="{ row }">
            <div class="visit-outcome">
              <hw-tag :theme="outcomeTheme(row)">{{
                outcomeLabel(row)
              }}</hw-tag>
              <span v-if="outcome(row).abnormal || outcome(row).observed">{{ outcomeReason(row) }}</span>
            </div>
          </template>
          <template #service="{ row }"
            ><span :title="row.host">{{
              host_nickname_dic[row.host_code] ||
              host_dic[row.host_code] ||
              row.host ||
              text('未归属服务', 'Unassigned')
            }}</span></template
          >
          <template #url="{ row }"
            ><div class="visit-request-path" :title="row.url">
              <span>{{ row.method }}</span
              ><span>{{ row.url || '/' }}</span>
            </div></template
          >
          <template #status_code="{ row }"
            ><span :class="{ 'visit-response-error': Number(row.status_code) >= 400 }">{{
              row.status_code || '—'
            }}</span></template
          >
          <template #risk_level="{ row }">{{ riskLabel(row.risk_level) }}</template>
          <template #rule="{ row }">
            <hw-tag v-if="row.rule" shape="round" theme="primary" variant="outline">{{ row.rule }}</hw-tag>
            <span v-else>—</span>
          </template>
          <template #log_only_mode="{ row }">
            <hw-tag :theme="row.log_only_mode == '1' ? 'primary' : 'default'" variant="light-outline">
              {{
                row.log_only_mode == '1'
                  ? $t('page.visit_log.log_only_mode_on')
                  : $t('page.visit_log.log_only_mode_off')
              }}
            </hw-tag>
          </template>
          <template #host_nickname="{ row }">
            <span>{{ host_nickname_dic[row.host_code] || '-' }}</span>
          </template>
          <template #src_ip="{ row }">
            <hw-tooltip :content="$t('common.ip_lookup.click_tip')">
              <button type="button" class="ipl-link" @click="openIpLookup(row.src_ip)">{{ row.src_ip }}</button>
            </hw-tooltip>
          </template>
          <template #op="slotProps"
            ><button type="button" class="t-button-link" @click="handleClickDetail(slotProps)">
              {{ $t('common.details') }}
            </button></template
          >
        </hw-table>
      </div>
    </hw-card>
    <hw-drawer
      v-model:visible="logConfigVisible"
      :header="text('日志设置', 'Log settings')"
      width="min(760px, 100vw)"
      class="visit-log-settings"
    >
      <p class="visit-settings-intro">
        {{ text('设置哪些访问会被记录，以及日志保留多久。', 'Choose what to record and how long to keep logs.') }}
      </p>
      <hw-alert v-if="logConfigError" theme="warning" :message="logConfigError"
        ><hw-button variant="text" @click="loadLogConfig">{{ text('重新读取', 'Reload') }}</hw-button></hw-alert
      >
      <hw-loading :loading="logConfigLoading">
        <hw-form :data="logConfig" :label-width="200" class="log-config-form">
          <hw-row :gutter="16">
            <hw-col :span="6">
              <hw-form-item label="是否记录响应报文" name="record_resp">
                <hw-select v-model="logConfig.record_resp" style="width: 100%">
                  <hw-option value="1" label="是" />
                  <hw-option value="0" label="否" />
                </hw-select>
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="记录原始请求BODY报文" name="record_all_src_byte_info">
                <hw-select v-model="logConfig.record_all_src_byte_info" style="width: 100%">
                  <hw-option value="1" label="启动" />
                  <hw-option value="0" label="关闭" />
                </hw-select>
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="日志记录类型" name="record_log_type">
                <hw-select v-model="logConfig.record_log_type" style="width: 100%">
                  <hw-option value="all" label="全部" />
                  <hw-option value="abnormal" label="非正常" />
                </hw-select>
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="记录请求最大报文(字节)" name="record_max_req_body_length">
                <hw-input-number v-model="logConfig.record_max_req_body_length" style="width: 100%" :min="0" />
              </hw-form-item>
            </hw-col>
          </hw-row>

          <hw-row :gutter="16">
            <hw-col :span="6">
              <hw-form-item label="记录响应最大报文(字节)" name="record_max_res_body_length">
                <hw-input-number v-model="logConfig.record_max_res_body_length" style="width: 100%" :min="0" />
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="删除历史日志(天)" name="delete_history_log_day">
                <hw-input-number v-model="logConfig.delete_history_log_day" style="width: 100%" :min="1" />
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="日志归档最大记录数" name="log_db_size">
                <hw-input-number v-model="logConfig.log_db_size" style="width: 100%" :min="0" />
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="日志归档最大文件(MB)" name="db_file_size">
                <hw-input-number v-model="logConfig.db_file_size" style="width: 100%" :min="0" />
              </hw-form-item>
            </hw-col>
          </hw-row>

          <hw-row :gutter="16">
            <hw-col :span="6">
              <hw-form-item label="是否开启日志持久化" name="log_persist_enable">
                <hw-select v-model="logConfig.log_persist_enable" style="width: 100%">
                  <hw-option value="1" label="开启" />
                  <hw-option value="0" label="关闭" />
                </hw-select>
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="数据库批量插入数量" name="batch_insert">
                <hw-input-number v-model="logConfig.batch_insert" style="width: 100%" :min="1" />
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="IP Tag 存放位置" name="ip_tag_db">
                <hw-select v-model="logConfig.ip_tag_db" style="width: 100%">
                  <hw-option value="0" label="主库" />
                  <hw-option value="1" label="读取 stat库" />
                </hw-select>
              </hw-form-item>
            </hw-col>
          </hw-row>

          <hw-row :gutter="16">
            <hw-col :span="6">
              <hw-form-item label="访问日志档位" name="access_log_mode">
                <hw-select v-model="logConfig.access_log_mode" style="width: 100%">
                  <hw-option value="db" label="全部入库" />
                  <hw-option value="sample" label="采样入库" />
                  <hw-option value="off" label="仅安全事件" />
                </hw-select>
                <div v-if="logConfig.access_log_mode === 'off'" class="log-config-hint">
                  「仅安全事件」会失去 CC 阈值推荐与异常 IP 的正常行为回溯
                </div>
              </hw-form-item>
            </hw-col>

            <hw-col :span="6">
              <hw-form-item label="访问日志保留天数" name="access_log_retention_days">
                <hw-input-number v-model="logConfig.access_log_retention_days" style="width: 100%" :min="1" />
              </hw-form-item>
            </hw-col>
          </hw-row>

          <hw-row :gutter="16">
            <hw-col :span="12">
              <hw-form-item label="全局排除记录日志的IP" name="exclude_ip_log">
                <hw-textarea
                  v-model="logConfig.exclude_ip_log"
                  :placeholder="'单IP/CIDR/通配符/区间，或 group:组短码 引用IP组；逗号或换行分隔，#开头为注释。只静音正常请求，安全事件照常记录；站点级清单在网站编辑里'"
                  :autosize="{ minRows: 2, maxRows: 6 }"
                  style="width: 100%"
                />
              </hw-form-item>
            </hw-col>
          </hw-row>
        </hw-form>
      </hw-loading>
      <div class="visit-settings-tools">
        <span>{{
          $t('page.visit_log.retention_line', {
            d: logConfig.delete_history_log_day || '—',
            a: logConfig.access_log_retention_days || '—',
          })
        }}</span>
        <hw-button variant="outline" @click="openShardManage">{{ $t('page.visit_log.shard_manage') }}</hw-button>
      </div>
      <template #footer>
        <hw-button variant="outline" @click="logConfigVisible = false">{{ $t('common.close') }}</hw-button>
        <hw-button
          theme="primary"
          :disabled="logConfigLoading || !!logConfigError || !logConfigDirty"
          :loading="logConfigSaving"
          @click="saveLogConfig"
          >{{ text('保存设置', 'Save settings') }}</hw-button
        >
      </template>
    </hw-drawer>
    <!-- 分区管理 -->
    <log-export-dialog v-if="attack_ip === '' && $route.name === 'WafvisitLog'" ref="logExport" />
    <hw-dialog
      :header="$t('page.visit_log.shard_manage')"
      v-model:visible="shardManageVisible"
      width="820px"
      :footer="false"
      :onClose="
        () => {
          this.shardManageVisible = false;
        }
      "
    >
      <hw-alert theme="info" :message="$t('page.visit_log.shard_manage_tip')" style="margin-bottom: 12px" />
      <hw-table :data="shardRows" :columns="shardManageColumns" row-key="file_name" size="small" max-height="420">
        <template #label="{ row }">
          <span :title="row.file_name">{{ shardTitle(row.file_name) }}</span>
        </template>
        <template #tiers="{ row }">
          <span v-if="row.is_current" style="color: var(--hw-muted)">—</span>
          <hw-tag v-else-if="row.missing" size="small" theme="danger" variant="light">
            {{ $t('page.visit_log.shard_missing') }}
          </hw-tag>
          <span v-else-if="!row.tiers || row.tiers.length === 0" style="color: var(--hw-muted)">
            {{ $t('page.visit_log.shard_tier_unknown') }}
          </span>
          <hw-tag
            v-else
            v-for="t in row.tiers"
            :key="t"
            size="small"
            variant="light"
            :theme="t === 'access_log' ? 'primary' : 'default'"
            style="margin-right: 4px"
          >
            {{ tierLabel(t) }}
          </hw-tag>
        </template>
        <template #op="{ row }">
          <span v-if="row.is_current" style="color: var(--hw-muted)">{{ $t('page.visit_log.shard_live') }}</span>
          <button type="button" v-else class="t-button-link" style="color: #d54941" @click="confirmDeleteShard(row)">
            {{ $t('common.delete') }}
          </button>
        </template>
      </hw-table>
    </hw-dialog>

    <hw-dialog
      :header="$t('page.visit_log.pop_detail_header')"
      v-model:visible="visitDetailVisible"
      width="clamp(640px, 80vw, 1180px)"
      :confirmOnEnter="true"
      :onConfirm="
        () => {
          this.visitDetailVisible = false;
        }
      "
      :onClose="
        () => {
          this.visitDetailVisible = false;
        }
      "
    >
      <visit-detail-page
        :prop_current_db="searchformData.current_db_name"
        :prop_req_uuid="visitDetailUid"
      ></visit-detail-page>
    </hw-dialog>

    <VisitLogColumnDialog ref="columnDialog" :columns="columns" @save="saveColumnConfig" />
    <hw-dialog
      :header="text('排查来源 IP', 'Diagnose source IP')"
      v-model:visible="ipExtractDialogVisible"
      :footer="false"
    >
      <template #body>
        <p>{{ text('选择接收请求的服务，检查其可信代理与来源 IP 请求头。', 'Choose the service that receives the request to check its trusted proxies and client IP header.') }}</p>
        <p v-if="hostError" role="alert">{{ hostError }} <hw-button @click="loadHostList">{{ text('重试', 'Retry') }}</hw-button></p>
        <p v-else-if="!hostLoading && !serviceOptions.length">{{ text('尚未配置受保护服务。', 'No protected services configured.') }}</p>
        <hw-select v-else v-model="ipExtractHostCode" filterable :loading="hostLoading"
          :aria-label="text('受保护服务', 'Protected service')" :placeholder="text('选择服务', 'Select a service')">
          <hw-option v-for="item in serviceOptions" :key="item.value" :value="item.value" :label="item.label" />
        </hw-select>
        <hw-button theme="primary" :disabled="!serviceByCode[ipExtractHostCode]" @click="gotoHostIpSource">
          {{ text('打开代理配置', 'Open proxy settings') }}
        </hw-button>
      </template>
    </hw-dialog>
  </div>
</template>
<script lang="ts">
import { defineComponent } from 'vue';
import {
  recommendedLogColumns,
  normalizeLogColumns,
  logColumns,
  requestOutcome,
  logDateRange,
  logTimeBounds,
} from './logPresentation';

import { SearchIcon } from '@/ui/icons';
import Trend from '@/components/trend/index.vue';
import { prefix } from '@/config/global';
import { allsharedblist, delsharedb } from '@/apis/waflog/attacklog';
import LogExportDialog from './components/LogExportDialog.vue';
import VisitLogColumnDialog from './components/VisitLogColumnDialog.vue';
import VisitDetailPage from './detail/index.vue';

import { NowDate, ConvertStringToUnix, ConvertDateToString, ConvertUnixToDate } from '@/utils/date';
import { listServices } from '@/apis/services';
import { get_detail_by_item_api, edit_system_config_api } from '@/apis/systemconfig';
import { get_ui_preference_api, save_ui_preference_api } from '@/apis/uipreference';

// 列配置持久化：服务端（按登录账号）为准，localStorage 只作首屏秒开缓存 + 接口不可用兜底。
const COLUMN_CONFIG_KEY = 'attack_table_display_columns';
const COLUMN_CONFIG_VERSION = 3;
const COLUMN_PREF_NAME = 'visit_log_columns'; // 服务端偏好名（后端白名单内）

// 可由外部路由 query 预设的筛选字段（必须是 searchformData 的合法键，防止任意 query 注入）。
// 不含 unix_add_time_begin/end（由日期控件驱动）和 current_db_name（由 loadShareDbList 异步定值）
const ROUTE_FILTER_QUERY_KEYS = [
  'action',
  'src_ip',
  'host_code',
  'url_keyword',
  'rule',
  'req_uuid',
  'status_code',
  'method',
  'log_only_mode',
  'view_type',
  'request_outcome',
];
// 外部页面可用这两个 query 指定日期区间（形如 2026-07-01 00:00:00），不传则用"今天"
const ROUTE_DATE_QUERY_KEYS = ['date_begin', 'date_end'];

export default defineComponent({
  name: 'WebLogList',
  components: {
    LogExportDialog,
    VisitLogColumnDialog,
    SearchIcon,
    Trend,
    VisitDetailPage,
  },
  props: {
    attack_ip: {
      type: String,
      default: '',
    },
  },
  data() {
    return {
      // 这次查询覆盖了哪些分区（后端给）：自动模式与识别码直查都靠它向用户解释数据来源
      queryMeta: {
        shards: [],
        found_in: '',
        scanned: 0,
        uuid_lookup: false,
        sort_forced_time: false,
        partial: false,
        took_ms: 0,
        issues: [],
      },
      shardGroups: [],
      covExpanded: false,
      issExpanded: false,
      shardManageVisible: false,
      shardRows: [],
      quickRange: '30',
      dateControl: {
        presets: {
          [this.$t('page.visit_log.date_range_last_300_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 299)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_90_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 89)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_60_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 59)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_30_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 29)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_15_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 14)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_7_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 6)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_last_3_days')]: [
            ConvertDateToString(new Date(+new Date() - 86400000 * 2)) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
          [this.$t('page.visit_log.date_range_today')]: [
            ConvertDateToString(new Date()) + ' 00:00:00',
            ConvertDateToString(new Date()) + ' 23:59:59',
          ],
        },
        range1: ['2023-11-01 00:00:00', '2023-11-16 23:59:59'],
      },
      action_options: [
        {
          label: this.$t('common.defense_status.all'),
          value: '',
        },
        {
          label: this.$t('common.defense_status.stop'),
          value: '阻止',
        },
        {
          label: this.$t('common.defense_status.pass'),
          value: '放行',
        },
        {
          label: this.$t('common.defense_status.forbid'),
          value: '禁止',
        },
      ],
      method_options: [
        {
          label: this.$t('common.all'),
          value: '',
        },
        {
          label: 'POST',
          value: 'POST',
        },
        {
          label: 'GET',
          value: 'GET',
        },
        {
          label: 'CONNECT',
          value: 'CONNECT',
        },
        {
          label: 'HEAD',
          value: 'HEAD',
        },
        {
          label: 'OPTIONS',
          value: 'OPTIONS',
        },
        {
          label: 'PUT',
          value: 'PUT',
        },
        { label: 'PATCH', value: 'PATCH' },
        { label: 'DELETE', value: 'DELETE' },
        {
          label: 'PRI',
          value: 'PRI',
        },
      ],
      prefix,
      dataLoading: false,
      compactView: window.innerWidth <= 1023,
      queryError: '',
      querySequence: 0,
      hostLoading: false,
      hostError: '',
      serviceOptions: [],
      serviceByCode: {},
      advancedVisible: false,
      timePreset: 'today',
      columnConfigPending: false,
      data: [],
      selectedRowKeys: [],
      value: 'first',
      customText: false,
      displayColumns: [...recommendedLogColumns],
      allColumns: logColumns((zh, en) => this.text(zh, en)),
      rowKey: 'req_uuid',
      tableLayout: 'auto',
      verticalAlign: 'top',
      hover: true,
      rowClassName: (rowKey: string) => `${rowKey}-class`,
      // 与pagination对齐
      pagination: {
        total: 0,
        current: 1,
        pageSize: 10,
      },
      searchValue: '',
      confirmVisible: false,
      deleteIdx: -1,
      //顶部搜索
      searchformData: {
        request_outcome: '',
        url_keyword: '',
        rule: '',
        req_uuid: '',
        action: '',
        src_ip: '',
        host_code: '',
        status_code: '',
        method: '',
        unix_add_time_begin: '',
        unix_add_time_end: '',
        current_db_name: 'auto',
        log_only_mode: '',
        view_type: 'access', // access=访问日志(全量窄行) event=安全事件
      },
      //table 字段
      table: {
        multipleSort: true,
      },
      //排序字段
      sorts: {
        sortBy: 'unix_add_time',
        descending: true,
      },
      //筛选字段
      filters: {
        filter_by: '',
        filter_value: '',
      },
      //主机字典
      host_dic: {},
      ipExtractHostCode: '', // 选中的受保护服务
      //主机昵称字典 host_code -> 纯昵称
      host_nickname_dic: {},
      //日志存档字典
      share_db_dic: {},
      visitDetailVisible: false, //访问详情弹窗
      visitDetailUid: '', //访问详情id

      // 日志配置相关
      logConfigVisible: false, // 日志配置区域是否展开
      logConfigLoading: false,
      logConfigError: '',
      savedLogConfig: {},
      logConfigSaving: false, // 保存配置按钮加载状态
      logConfig: {
        record_log_type: 'all',
        record_max_req_body_length: '0',
        record_max_res_body_length: '0',
        record_resp: '0',
        record_all_src_byte_info: '0',
        delete_history_log_day: '7',
        log_db_size: '0',
        db_file_size: '0',
        log_persist_enable: '0',
        batch_insert: '0',
        ip_tag_db: '0',
        exclude_ip_log: '',
        access_log_mode: 'db',
        access_log_retention_days: '30',
      },
      logConfigItems: {}, // 存储配置项的完整信息（包含ID等）

      // IP提取配置相关
      ipExtractDialogVisible: false,
    };
  },
  computed: {
    defaultOutcome() {
      return this.$route.name === 'WafvisitLog' && !this.attack_ip ? 'abnormal' : '';
    },
    responsiveColumns() {
      return this.compactView ? [{ colKey: 'summary', title: '', minWidth: 220 }] : this.columns;
    },
    offsetTop() {
      return this.$store.state.setting.isUseTabsRouter ? 48 : 0;
    },
    // 当前视图可见的列：访问日志视图没有 header 列（报文已搬进报文表，UA/Referer 顶替它的筛选位）
    columns() {
      if (this.searchformData.view_type !== 'access') {
        return this.allColumns;
      }
      return this.allColumns.filter((c) => c.colKey !== 'header');
    },
    // 内嵌在风险日志详情里时，访问日志视图展示的是该 IP 的全部行为
    shardManageColumns() {
      return [
        { colKey: 'label', title: this.$t('page.visit_log.log_archive_db'), width: 230 },
        { colKey: 'cnt', title: this.$t('page.visit_log.shard_rows'), width: 110 },
        { colKey: 'tiers', title: this.$t('page.visit_log.shard_tiers'), width: 280 },
        { colKey: 'op', title: this.$t('common.operation'), width: 90 },
      ];
    },
    // 覆盖概括：分区一多就不该把它们全铺在提示条里，先给「跨度 + 总条数」
    shardCoverSummary() {
      const list = this.queryMeta.shards || [];
      if (list.length < 2) return '';
      let total = 0;
      list.forEach((x) => {
        total += Number(x.count) || 0;
      });
      // shards 由后端按时间从新到旧给出
      const newest = this.shardTitle(list[0].name).split(' ·')[0];
      const oldest = this.shardTitle(list[list.length - 1].name).split(' ·')[0];
      return ' · ' + oldest + ' ~ ' + newest + (total ? this.fmtCnt(total) : '');
    },
    // 填了访问识别码 = 点查，时间与分区都不该再要求用户选
    isUuidLookup() {
      return !!(this.searchformData.req_uuid && this.searchformData.req_uuid.trim());
    },
    accessViewLabel() {
      return this.attack_ip !== '' ? this.$t('page.visit_log.view_access_all') : this.$t('page.visit_log.view_access');
    },
    compactColumnKeys() {
      return this.displayColumns.filter((key) => key !== 'op' && this.columns.some((column) => column.colKey === key));
    },
    advancedFilterCount() {
      return (
        ['rule', 'req_uuid', 'action', 'status_code', 'method', 'log_only_mode'].filter(
          (key) => this.searchformData[key],
        ).length +
        (this.searchformData.current_db_name !== 'auto' ? 1 : 0) +
        (this.searchformData.view_type === 'event' ? 1 : 0) +
        (this.filters.filter_by ? 1 : 0)
      );
    },
    logConfigDirty() {
      return Object.keys(this.savedLogConfig).some(
        (key) => String(this.logConfig[key]) !== String(this.savedLogConfig[key]),
      );
    },
  },
  created() {
    console.log(NowDate);
    this.dateControl.range1[0] = NowDate + ' 00:00:00';
    this.dateControl.range1[1] = NowDate + ' 23:59:59';
    this.searchformData.unix_add_time_begin = ConvertStringToUnix(this.dateControl.range1[0]).toString();
    this.searchformData.unix_add_time_end = ConvertStringToUnix(this.dateControl.range1[1]).toString();
    console.log(this.range1);
  },
  mounted() {
    window.addEventListener('resize', this.onViewportChange);
    console.log('attack list mounted ');
    // 先用本地缓存的列配置渲染首屏
    this.loadColumnConfig();
    // 加载日志配置
    this.loadLogConfig();

    // 作为弹窗子组件内嵌时（风险日志 / IP失败页），$route 是宿主页面的路由，不参与列配置同步与 query 预设
    const isOwnRoute = this.$route.name === 'WafvisitLog';
    if (isOwnRoute) {
      // 服务端为准，异步覆盖本地缓存结果
      this.loadColumnConfigFromServer();
    }

    const routeFilter = isOwnRoute ? this.collectRouteFilterQuery() : {};
    if (Object.keys(routeFilter).length > 0) {
      // 【issue #893 问题2】外部页面（首页卡片 / IP排行）带着筛选条件跳进来时必须以 query 为准：
      // 跳过 vuex 里上次离开时的搜索条件恢复，否则 searchformData 会被整体覆盖，
      // 预设筛选丢失（表现为"刷新一次才生效"）
      this.applyRouteFilterQuery(routeFilter);
      // 换了筛选条件，停在第 N 页没有意义；只沿用用户的每页条数偏好
      this.pagination.current = 1;
      const cached = this.$store.state.attacklog.msgData;
      if (cached && cached.pagesize) {
        this.pagination.pageSize = cached.pagesize;
      }
      // 日期区间保持 created() 里设置的"今天"，与首页卡片的"今日"口径一致
    } else if (isOwnRoute && this.$store.state.attacklog.msgData) {
      // 判断 vuex 中是否有保存的搜索参数
      const attack = this.$store.state.attacklog;
      this.pagination.current = attack.msgData.currentpage;
      this.pagination.pageSize = attack.msgData.pagesize;
      // 浅拷贝 + 当前默认对象打底：直接引用 store 里的对象会让后续表单编辑绕过 mutation 改 state，
      // 也会让弹窗内嵌实例把 src_ip 写脏主列表页缓存；历史缓存可能缺字段，打底避免 v-model 绑到 undefined
      this.searchformData = { ...this.searchformData, request_outcome: this.defaultOutcome, ...attack.msgData.searchData };
      console.log('daysrc', attack.msgData.searchData);
      let newrange = Array();
      newrange[0] = ConvertUnixToDate(parseInt(attack.msgData.searchData.unix_add_time_begin));
      newrange[1] = ConvertUnixToDate(parseInt(attack.msgData.searchData.unix_add_time_end));
      //console.log(this.dateControl.range1)
      this.$set(this.dateControl, 'range1', newrange);
    } else {
      this.searchformData.request_outcome = this.defaultOutcome;
    }

    this.loadShareDbList();
    this.loadHostList();
    if (this.attack_ip) this.updateSearchFormAttackPage();
    else {
      if (Object.keys(routeFilter).length || this.$store.state.attacklog.msgData) this.timePreset = 'custom';
      this.advancedVisible = this.advancedFilterCount > 0;
      this.getList('');
    }
  },
  watch: {
    // 同一路由内 query 变化（浏览器前进/后退等）；首次进入由 mounted 处理，此处不触发
    '$route.query': {
      deep: true,
      handler(newQuery, oldQuery) {
        if (this.$route.name !== 'WafvisitLog') return;
        if (JSON.stringify(newQuery) === JSON.stringify(oldQuery)) return;
        const picked = this.collectRouteFilterQuery();
        // 未在 query 中出现的筛选字段一律清空，避免上一次的筛选残留；
        // query 全空（如从带筛选的 URL 点侧边栏菜单回来）同样按"清空重查"处理
        ROUTE_FILTER_QUERY_KEYS.forEach((key) => {
          this.$set(this.searchformData, key, Object.prototype.hasOwnProperty.call(picked, key) ? picked[key] : '');
        });
        if (!Object.keys(picked).length) this.searchformData.request_outcome = this.defaultOutcome;
        // 日期区间与分页复位，口径与 mounted 分支一致
        if (!this.searchformData.view_type) this.searchformData.view_type = 'access';
        this.applyRouteDateQuery(picked);
        this.timePreset = picked.date_begin || picked.date_end ? 'custom' : 'today';
        this.advancedVisible = this.advancedFilterCount > 0;
        this.pagination.current = 1;
        this.getList('');
      },
    },
    attack_ip(newVal) {
      if (newVal !== '') {
        this.updateSearchFormAttackPage(); // 当 attack_ip 变化时，更新表单数据
      }
    },
  },
  beforeRouteLeave(to, from, next) {
    console.log('attack list beforeRouteLeave ');
    // vuex 存储操作
    const bounds = logTimeBounds(this.dateControl.range1);
    if (bounds) Object.assign(this.searchformData, bounds);
    this.querySequence += 1;
    this.$store.dispatch('attacklog/setAttackMsgData', {
      //query: this.queryParam,
      pagesize: this.pagination.pageSize,
      currentpage: this.pagination.current,
      searchData: { ...this.searchformData }, // 存快照，避免 store 持有活引用被后续编辑写脏
    });
    next(); // 继续后续的导航解析过程
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.onViewportChange);
    this.querySequence += 1;
  },
  methods: {
    onViewportChange() {
      this.compactView = window.innerWidth <= 1023;
    },
    compactValue(row, key) {
      if (key === 'service') return this.host_nickname_dic[row.host_code] || row.host || '—';
      if (key === 'url') return [row.method, row.url || '/'].filter(Boolean).join(' ');
      if (key === 'log_only_mode')
        return Number(row.log_only_mode) === 1 ? this.text('仅记录', 'Observe only') : this.text('关闭', 'Off');
      if (key === 'risk_level') return this.riskLabel(row.risk_level);
      return row[key] === '' || row[key] == null ? '—' : row[key];
    },
    text(zh, en) {
      return this.$i18n.locale === 'en_US' ? en : zh;
    },
    outcome: requestOutcome,
    outcomeLabel(row) {
      const state = requestOutcome(row);
      if (state.observed) return this.text('仅观察', 'Observed');
      return state.abnormal ? this.text('异常', 'Abnormal') : this.text('正常', 'Normal');
    },
    outcomeTheme(row) {
      const state = requestOutcome(row);
      if (state.observed) return 'primary';
      return state.abnormal ? (state.blocked ? 'danger' : 'warning') : 'success';
    },
    outcomeReason(row) {
      const state = requestOutcome(row);
      if (state.blocked) return this.text('已拦截', 'Blocked');
      if (state.unknown) return this.text('状态待确认', 'Unknown action');
      if (state.failed) return this.text('响应失败', 'Failed response');
      if (state.observed) return this.text('已放行，保留检测记录', 'Allowed; detection recorded');
      if (state.matched) return this.text('命中规则，未拦截', 'Matched, not blocked');
      return this.text('响应失败', 'Failed response');
    },
    riskLabel(value) {
      const n = Number(value);
      return n >= 4
        ? this.text('严重', 'Critical')
        : n >= 3
          ? this.text('高', 'High')
          : n >= 2
            ? this.text('中', 'Medium')
            : n >= 1
              ? this.text('低', 'Low')
              : '—';
    },
    changeTimePreset() {
      if (this.timePreset !== 'custom') this.dateControl.range1 = logDateRange(this.timePreset);
    },
    resetSearch() {
      Object.assign(this.searchformData, {
        request_outcome: this.defaultOutcome,
        host_code: '',
        src_ip: this.attack_ip || '',
        url_keyword: '',
        rule: '',
        req_uuid: '',
        action: '',
        status_code: '',
        method: '',
        log_only_mode: '',
        current_db_name: 'auto',
        view_type: 'access',
      });
      this.timePreset = 'today';
      this.dateControl.range1 = logDateRange('today');
      this.filters = { filter_by: '', filter_value: '' };
      this.sorts = { sortBy: 'unix_add_time', descending: true };
      this.pagination.current = 1;
      this.$refs.logTable?.clearFilter();
      this.getList('all');
    },
    columnLabel(key) {
      return this.allColumns.find((column) => column.colKey === key)?.title || key;
    },
    // 点日志里的 IP 直接开归属查询弹窗，省得用户复制粘贴
    openIpLookup(ip) {
      if (!ip) return;
      this.$refs.ipLookup && this.$refs.ipLookup.open(ip);
    },
    // 收集路由 query 中的显式筛选意图。
    // 用 hasOwnProperty 判定"存在"：?action= 得到 ''、?action 得到 null，
    // 这两种都属于"调用方明确要求把该字段设成空"，用 != null 判定会漏掉后者
    collectRouteFilterQuery() {
      const query = (this.$route && this.$route.query) || {};
      const picked = {};
      ROUTE_FILTER_QUERY_KEYS.concat(ROUTE_DATE_QUERY_KEYS).forEach((key) => {
        if (!Object.prototype.hasOwnProperty.call(query, key)) return;
        const v = query[key];
        // vue-router 对重复 key 会给数组，取第一个；null（?key 无值）归一成空串
        picked[key] = Array.isArray(v) ? (v[0] == null ? '' : String(v[0])) : v == null ? '' : String(v);
      });
      return picked;
    },

    // 把 query 预设应用到当前（created 已初始化为今天）的搜索表单上
    applyRouteFilterQuery(picked) {
      Object.keys(picked).forEach((key) => {
        if (ROUTE_DATE_QUERY_KEYS.includes(key)) return; // 日期区间单独处理
        this.$set(this.searchformData, key, picked[key]);
      });
      this.applyRouteDateQuery(picked);
    },

    // 应用路由带来的日期区间；未指定则复位成"今天"，与首页"今日"类入口口径一致
    applyRouteDateQuery(picked) {
      const begin = picked.date_begin;
      const end = picked.date_end;
      const newRange = begin && end ? [begin, end] : [NowDate + ' 00:00:00', NowDate + ' 23:59:59'];
      this.$set(this.dateControl, 'range1', newRange);
      this.searchformData.unix_add_time_begin = ConvertStringToUnix(newRange[0]).toString();
      this.searchformData.unix_add_time_end = ConvertStringToUnix(newRange[1]).toString();
    },

    // 切换列配置弹窗
    openColumnDialog() {
      if (this.columnConfigPending) return;
      this.$refs.columnDialog.open(this.displayColumns);
    },

    // Keep the current account's selected columns for the initial render.
    persistColumnConfigLocalSafe(columns) {
      try {
        localStorage.setItem(COLUMN_CONFIG_KEY, JSON.stringify({
          v: COLUMN_CONFIG_VERSION,
          account: localStorage.getItem('current_account') || '',
          columns: Array.from(new Set(columns)),
        }));
      } catch (error) {
        console.warn('写入本地列配置缓存失败:', error);
      }
    },

    mergeColumnConfig(columns) {
      return normalizeLogColumns(columns, this.allColumns.map((column) => column.colKey));
    },

    loadColumnConfig() {
      try {
        const raw = localStorage.getItem(COLUMN_CONFIG_KEY);
        if (!raw) return;
        const parsed = JSON.parse(raw);
        const account = localStorage.getItem('current_account') || '';
        if (parsed?.v === COLUMN_CONFIG_VERSION && parsed.account === account && Array.isArray(parsed.columns)) {
          this.displayColumns = this.mergeColumnConfig(parsed.columns);
        }
      } catch (error) {
        console.warn(this.$t('common.column_config_load_failed'), error);
      }
    },

    // 从服务端加载列配置（异步，服务端为准，覆盖本地缓存结果）
    loadColumnConfigFromServer() {
      this.columnConfigPending = true;
      get_ui_preference_api({ pref_name: COLUMN_PREF_NAME })
        .then((res) => {
          if (res.code !== 0) {
            console.warn('获取服务端列配置失败:', res.msg);
            return;
          }
          if (res.data && res.data.id && res.data.pref_json) {
            const payload = JSON.parse(res.data.pref_json);
            if (payload?.v === COLUMN_CONFIG_VERSION && Array.isArray(payload.columns) && payload.columns.length > 0) {
              const merged = this.mergeColumnConfig(payload.columns);
              this.displayColumns = merged;
              this.persistColumnConfigLocalSafe(merged);
            }
          }
        })
        .catch((err) => {
          console.warn('获取服务端列配置异常:', err);
        })
        .finally(() => {
          this.columnConfigPending = false;
        });
    },

    // 把当前列配置同步到服务端
    syncColumnConfigToServer() {
      const payload = {
        v: COLUMN_CONFIG_VERSION,
        columns: Array.from(new Set(this.displayColumns)),
      };
      return save_ui_preference_api({
        pref_name: COLUMN_PREF_NAME,
        pref_json: JSON.stringify(payload),
      }).then((res) => {
        if (res.code !== 0) {
          return Promise.reject(new Error(res.msg || 'save failed'));
        }
        return res;
      });
    },

    // 保存列配置（本地缓存 + 服务端）
    saveColumnConfig(columns) {
      if (this.columnConfigPending) return;
      this.columnConfigPending = true;
      this.displayColumns = columns;
      this.persistColumnConfigLocalSafe(this.displayColumns);
      // 成功提示等服务端返回后再弹，避免与调用方的提示重复/乱序
      this.syncColumnConfigToServer()
        .then(() => {
          this.$message.success(this.$t('common.column_config_saved'));
        })
        .catch((error) => {
          console.warn('同步列配置到服务端失败:', error);
          this.$message.warning(this.$t('common.column_config_sync_failed'));
        })
        .finally(() => {
          this.columnConfigPending = false;
        });
    },

    updateSearchFormAttackPage() {
      console.log('updateSearchFormAttackPage');
      if (this.attack_ip != '') {
        console.log('attack ip index', this.attack_ip);
        this.searchformData.src_ip = this.attack_ip;
        // 风险详情默认落在安全事件视图（命中明细）；旁边的「全部行为」页签是该 IP 的访问日志视图
        this.timePreset = 'custom';
        this.searchformData.view_type = 'event';
        this.dateControl.range1[0] = '2022-01-01 00:00:00';
        this.dateControl.range1[1] = NowDate + ' 23:59:59';
        this.searchformData.unix_add_time_begin = ConvertStringToUnix(this.dateControl.range1[0]).toString();
        this.searchformData.unix_add_time_end = ConvertStringToUnix(this.dateControl.range1[1]).toString();
        this.pagination.current = 1;
        this.getList('');
      }
    },
    // 条数写成人能读的样子：13.5 万条 而不是 135140
    fmtCnt(n) {
      if (n === undefined || n === null || n === '') return '';
      const v = Number(n);
      if (isNaN(v)) return '';
      if (v >= 10000) return ' · ' + (v / 10000).toFixed(1) + ' 万条';
      return ' · ' + v + ' 条';
    },
    // 归档分片的显示名：实时单独标出，按月分区显示「年-月」，

    shardLabel(item) {
      const cnt = this.fmtCnt(item.cnt);
      if (item.is_current) {
        return this.$t('page.visit_log.shard_live') + cnt;
      }
      // 按层过期之后，窄行那一层可能已经被回收，只剩安全事件与报文——
      // 下拉里得标出来，否则选中它再切到访问日志视图只会得到一句生硬的报错
      // 登记还在、存储已不在：照样列出来（好让人知道有这么个分区、去分区管理删登记），但选不了
      const note = item.missing
        ? ' · ' + this.$t('page.visit_log.shard_missing')
        : item.tiers && item.tiers.length > 0 && item.tiers.indexOf('access_log') < 0
          ? ' · ' + this.$t('page.visit_log.shard_only_event')
          : '';
      const period = item.period_key;
      if (!period) return item.file_name + cnt + note;
      const sequence = /_([0-9]{2,})(?:\.db)?$/.exec(item.file_name);
      const extra = sequence && sequence[1] !== period ? ' (' + sequence[1] + ')' : '';
      return period.slice(0, 4) + '-' + period.slice(4) + extra + cnt + note;
    },
    // 识别码直查模式：填了访问识别码就忽略时间与分区，后端按主键跨分区找
    applyQuickRange() {
      const d = (t) => {
        const p = (n) => (n < 10 ? '0' + n : '' + n);
        return t.getFullYear() + '-' + p(t.getMonth() + 1) + '-' + p(t.getDate());
      };
      const days = parseInt(this.quickRange, 10);
      const end = d(new Date()) + ' 23:59:59';
      const start = days > 0 ? d(new Date(+new Date() - 86400000 * (days - 1))) + ' 00:00:00' : '1970-01-02 00:00:00';
      this.dateControl.range1 = [start, end];
    },
    // 分区管理：打开前刷一次，免得删完别处看到的还是旧列表
    openShardManage() {
      this.loadShareDbList();
      this.shardManageVisible = true;
    },
    tierLabel(t) {
      const m = {
        access_log: this.$t('page.visit_log.view_access'),
        security_event: this.$t('page.visit_log.view_event'),
        event_payload: this.$t('logExport.payload'),

      };
      return m[t] || t;
    },
    confirmDeleteShard(row) {
      const that = this;
      const name = this.shardTitle(row.file_name);
      const confirmDia = this.$dialog.confirm({
        header: this.$t('page.visit_log.shard_del_title'),
        body: this.$t('page.visit_log.shard_del_body', { name: name, raw: row.file_name }),
        theme: 'warning',
        confirmBtn: { content: this.$t('common.delete'), theme: 'danger' },
        onConfirm: ({ e }) => {
          delsharedb({ file_name: row.file_name })
            .then((res) => {
              if (res.code === 0) {
                confirmDia.destroy();
                that.$message.success(res.msg || that.$t('page.visit_log.shard_del_done'));
                that.loadShareDbList();
                // 删掉的可能正好是当前锁定的分区，回到自动模式免得查了个不存在的东西
                if (that.searchformData.current_db_name === row.file_name) {
                  that.searchformData.current_db_name = 'auto';
                }
                that.getList('all');
              } else {
                confirmDia.hide();
                that.$message.error(res.msg || that.$t('page.visit_log.shard_del_fail'));
              }
            })
            .catch(() => {
              confirmDia.hide();
              that.$message.error(that.$t('page.visit_log.shard_del_fail'));
            });
        },
      });
    },
    // 空态里点「IP归属查询」：带上当前筛的来源IP，直接看它是不是被排除清单命中了
    openIpLookupForHint() {
      const ip = (this.searchformData.src_ip || '').trim();
      this.openIpLookup(ip);
    },
    // 展开「日志配置」卡并滚到顶：保留天数就在那里改
    openRetentionSetting() {
      this.logConfigVisible = true;
      this.$nextTick(() => {
        const el = document.querySelector('.log-config-card');
        if (el && el.scrollIntoView) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    },
    // 超预算时的一键补救：把时间范围收到最近 7 天再查
    narrowRange() {
      this.timePreset = '7d';
      this.changeTimePreset();
      this.getList('all');
    },
    // 分区的显示名：优先用下拉里已经算好的标签，取不到就退回原始标识
    shardTitle(name) {
      if (!name) return '';
      return this.share_db_dic[name] || name;
    },
    loadShareDbList() {
      let that = this;
      allsharedblist('')
        .then((res) => {
          let resdata = res;
          console.log('loadShareDbList', resdata);
          if (resdata.code === 0) {
            let share_options = resdata.data;

            that.shardRows = share_options;
            const gLive = { label: that.$t('page.visit_log.shard_group_live'), options: [] };
            const gPeriod = { label: that.$t('page.visit_log.shard_group_period'), options: [] };

            for (let i = 0; i < share_options.length; i++) {
              const it = share_options[i];
              // 下拉里显示「时间段」而不是文件名/表名：归档分区按月切，用户要找的是某段时间的日志
              const label = that.shardLabel(it);
              that.share_db_dic[it.file_name] = label;
              const opt = { value: it.file_name, label: label, disabled: !!it.missing };

              if (it.is_current) {
                gLive.options.push(opt);
              } else {
                gPeriod.options.push(opt);
              }
            }
            that.shardGroups = [gLive, gPeriod].filter((g) => g.options.length > 0);
            // 默认停在「自动（按时间范围）」：分区由时间算出来，不再让用户自己猜

          }
        })
        .catch((e: Error) => {
          console.log(e);
        });
    },
    async loadHostList() {
      this.hostLoading = true;
      this.hostError = '';
      try {
        const res = await listServices();
        if (res.code !== 0 || !Array.isArray(res.data))
          throw new Error(res.msg || this.text('服务列表暂时无法读取', 'Services unavailable'));
        this.serviceOptions = res.data;
        this.host_dic = Object.fromEntries(res.data.map((item) => [item.value, item.label]));
        this.host_nickname_dic = Object.fromEntries(res.data.map((item) => [item.value, item.nickname || item.label]));
        this.serviceByCode = Object.fromEntries(res.data.map((item) => [item.value, item]));
      } catch (error) {
        this.hostError = error.message || this.text('服务列表读取失败，请重试。', 'Could not load services. Retry.');
      } finally {
        this.hostLoading = false;
      }
    },
    async getList(keyword) {
      if (keyword === 'all') this.pagination.current = 1;
      if (this.timePreset !== 'custom' && !this.isUuidLookup) this.dateControl.range1 = logDateRange(this.timePreset);
      const bounds = logTimeBounds(this.dateControl.range1);
      const sequence = ++this.querySequence;
      if (!this.isUuidLookup && !bounds) {
        this.queryError = this.text(
          '请选择有效的起止时间，开始时间不能晚于结束时间。',
          'Choose a valid start and end time.',
        );
        this.data = [];
        this.pagination.total = 0;
        this.dataLoading = false;
        return;
      }
      if (bounds) Object.assign(this.searchformData, bounds);
      const requestData: Record<string, any> = {
        ...this.getFilteredSearchData(),
        ...bounds,
        pageSize: this.pagination.pageSize,
        pageIndex: this.pagination.current,
        sort_by: this.sorts.sortBy,
        sort_descending: this.sorts.descending ? 'desc' : 'asc',
        filter_by: this.filters.filter_by,
        filter_value: this.filters.filter_value,
      };
      if (requestData.req_uuid) {
        requestData.current_db_name = 'auto';
        delete requestData.request_outcome;
        delete requestData.unix_add_time_begin;
        delete requestData.unix_add_time_end;
      }
      this.queryError = '';
      this.dataLoading = true;
      try {
        const res = await this.$request.post('/waflog/attack/list', requestData);
        if (sequence !== this.querySequence) return;
        if (res.code !== 0 || !Array.isArray(res.data?.list))
          throw new Error(res.msg || this.text('读取日志失败', 'Could not load logs'));
        this.data = res.data.list;
        this.pagination.total = Number(res.data.total) || 0;
        this.queryMeta = {
          shards: res.data.shards || [],
          found_in: res.data.found_in || '',
          scanned: res.data.scanned || 0,
          uuid_lookup: !!res.data.uuid_lookup,
          sort_forced_time: !!res.data.sort_forced_time,
          partial: !!res.data.partial,
          took_ms: res.data.took_ms || 0,
          issues: res.data.issues || [],
        };
        if (res.data.sort_forced_time) this.sorts = { sortBy: 'unix_add_time', descending: this.sorts.descending };
      } catch (error) {
        if (sequence !== this.querySequence) return;
        this.data = [];
        this.pagination.total = 0;
        this.queryMeta = {
          shards: [],
          found_in: '',
          scanned: 0,
          uuid_lookup: false,
          sort_forced_time: false,
          partial: false,
          took_ms: 0,
          issues: [],
        };
        this.queryError = error.message || this.text('日志读取失败，请重试。', 'Could not load logs. Retry.');
      } finally {
        if (sequence === this.querySequence) this.dataLoading = false;
      }
    },
    getContainer() {
      return document.querySelector('.tdesign-starter-layout');
    },
    rehandlePageChange(curr, pageInfo) {
      this.pagination.current = curr.current;
      if (this.pagination.pageSize != curr.pageSize) {
        this.pagination.current = 1;
        this.pagination.pageSize = curr.pageSize;
      }
      this.getList('');
    },
    rehandleSelectChange(selectedRowKeys: number[]) {
      this.selectedRowKeys = selectedRowKeys;
    },
    rehandleChange(changeParams, triggerAndData) {},
    handleClickDetail(e) {
      const { req_uuid } = e.row;
      // 行自带来源分区（扇出时后端回填）：带上它详情就能直达，不必再逐个分区找
      const shard = e.row.shard_name || this.searchformData.current_db_name;

      if (this.attack_ip == '') {
        this.$router.push({
          path: '/waf/wafattacklogdetail',
          query: {
            req_uuid: req_uuid + '#' + shard,
          },
        });
      } else {
        this.visitDetailUid = req_uuid;
        this.visitDetailVisible = true;
      }
    },
    handleClickIPDetail(e) {
      console.log(e);
      const { src_ip } = e.row;
      this.searchformData.src_ip = src_ip;
      this.getList('all');
    },
    handleClickDelete(row: { rowIndex: any }) {
      this.deleteIdx = row.rowIndex;
      this.confirmVisible = true;
    },
    onConfirmDelete() {
      this.data.splice(this.deleteIdx, 1);
      this.pagination.total = this.data.length;
      const selectedIdx = this.selectedRowKeys.indexOf(this.deleteIdx);
      if (selectedIdx > -1) {
        this.selectedRowKeys.splice(selectedIdx, 1);
      }
      this.confirmVisible = false;
      this.$message.success('删除成功');
      this.resetIdx();
    },
    onCancel() {
      this.resetIdx();
    },
    resetIdx() {
      this.deleteIdx = -1;
    },
    //Jump Url
    /**
     * table 排序
     */
    onSortChange(sorter) {
      this.pagination.current = 1;
      let that = this;

      if (sorter != undefined) {
        this.sorts.sortBy = sorter.sortBy;
        that.sorts.descending = sorter.descending;
      } else {
        that.sorts.sortBy = 'unix_add_time';
        that.sorts.descending = true;
      }
      this.getList('');
    },
    // 切换访问日志/安全事件视图。「请求」全文筛选只在安全事件视图有，切走时摘掉，否则后端会拒
    onViewTypeChange() {
      if (this.searchformData.view_type !== 'event' && this.filters.filter_by.indexOf('header') >= 0) {
        this.filters.filter_by = '';
        this.filters.filter_value = '';
      }
      this.pagination.current = 1;
      this.getList('');
    },
    /**
     * 筛选结果
     */
    onFilterChange(e) {
      this.pagination.current = 1;
      this.filters.filter_by = '';
      this.filters.filter_value = '';
      //请求ID
      if (e.req_uuid != undefined && e.req_uuid != '') {
        this.filters.filter_by = 'req_uuid';
        this.filters.filter_value = e.req_uuid;
      }
      //header
      if (e.header != undefined && e.header != '') {
        if (this.filters.filter_by == '') {
          this.filters.filter_by = 'header';
          this.filters.filter_value = e.header;
        } else {
          this.filters.filter_by = this.filters.filter_by + '|header';
          this.filters.filter_value = this.filters.filter_value + '|' + e.header;
        }
      }
      //User-Agent / Referer（窄行列，两个视图都可用）
      if (e.user_agent != undefined && e.user_agent != '') {
        if (this.filters.filter_by == '') {
          this.filters.filter_by = 'user_agent';
          this.filters.filter_value = e.user_agent;
        } else {
          this.filters.filter_by = this.filters.filter_by + '|user_agent';
          this.filters.filter_value = this.filters.filter_value + '|' + e.user_agent;
        }
      }
      if (e.referer != undefined && e.referer != '') {
        if (this.filters.filter_by == '') {
          this.filters.filter_by = 'referer';
          this.filters.filter_value = e.referer;
        } else {
          this.filters.filter_by = this.filters.filter_by + '|referer';
          this.filters.filter_value = this.filters.filter_value + '|' + e.referer;
        }
      }

      this.getList('');
    },
    resetState() {
      this.resetSearch();
    },

    // 获取过滤后的搜索数据
    getFilteredSearchData() {
      const filteredData = {};
      Object.keys(this.searchformData).forEach((key) => {
        const value = this.searchformData[key];
        if (typeof value === 'string' && key !== 'url_keyword') {
          filteredData[key] = value.trim();
        } else {
          filteredData[key] = value;
        }
      });
      return filteredData;
    },

    // 切换日志配置区域显示/隐藏
    toggleLogConfig() {
      this.logConfigVisible = !this.logConfigVisible;
    },

    // 加载日志配置
    loadLogConfig() {
      this.logConfigLoading = true;
      this.logConfigError = '';
      const configKeys = [
        'record_log_type',
        'record_max_req_body_length',
        'record_max_res_body_length',
        'record_resp',
        'record_all_src_byte_info',
        'delete_history_log_day',
        'log_db_size',
        'db_file_size',
        'log_persist_enable',
        'batch_insert',
        'ip_tag_db',
        'access_log_mode',
        'access_log_retention_days',
        'exclude_ip_log',
      ];

      // 使用 Promise.all 并行获取所有配置项
      const promises = configKeys.map((key) => {
        return get_detail_by_item_api({ item: key })
          .then((res) => {
            if (res.code === 0 && res.data) {
              this.logConfigItems[key] = res.data;
              const numeric = [
                'record_max_req_body_length',
                'record_max_res_body_length',
                'delete_history_log_day',
                'log_db_size',
                'db_file_size',
                'batch_insert',
                'access_log_retention_days',
              ];
              this.logConfig[key] = numeric.includes(key) ? Number(res.data.value) : res.data.value;
            }
            return { key, success: res.code === 0 && !!res.data };
          })
          .catch((err) => {
            console.error(`加载配置项 ${key} 失败:`, err);
            return { key, success: false };
          });
      });

      Promise.all(promises).then((results) => {
        this.logConfigLoading = false;
        const failedItems = results.filter((r) => !r.success);
        if (failedItems.length)
          this.logConfigError = this.text(
            '部分设置加载失败，请重新读取后再保存。',
            'Some settings failed to load. Reload before saving.',
          );
        this.savedLogConfig = { ...this.logConfig };
        if (failedItems.length === 0) {
          console.log('日志配置加载成功', this.logConfig);
        } else {
          console.warn('部分配置项加载失败:', failedItems);
        }
      });
    },

    // 保存日志配置
    async saveLogConfig() {
      if (this.logConfigLoading || this.logConfigError || !this.logConfigDirty) return;
      this.logConfigSaving = true;
      const changes = Object.keys(this.savedLogConfig)
        .filter((key) => String(this.savedLogConfig[key]) !== String(this.logConfig[key]))
        .map((key) => ({ key, value: String(this.logConfig[key]), item: this.logConfigItems[key] }));
      const results = await Promise.allSettled(
        changes.map(async ({ key, value, item }) => {
          if (!item) throw new Error(key);
          const res = await edit_system_config_api({
            id: item.id,
            category: item.category,
            item: item.item,
            value,
            type: item.type,
            title: item.title,
            options: item.options || '',
          });
          if (res.code !== 0) throw new Error(res.msg || key);
          this.savedLogConfig[key] = value;
        }),
      );
      this.logConfigSaving = false;
      if (results.some((result) => result.status === 'rejected')) {
        this.$message.error(this.text('部分设置未保存，请重试。', 'Some settings were not saved. Retry.'));
      } else {
        this.$message.success(this.$t('common.tips.save_success'));
        this.getList('');
      }
    },

    handleIPExtractIssue() {
      this.ipExtractHostCode = this.searchformData.host_code || '';
      if (this.serviceByCode[this.ipExtractHostCode]) this.gotoHostIpSource();
      else this.ipExtractDialogVisible = true;
    },
    gotoHostIpSource() {
      const service = this.serviceByCode[this.ipExtractHostCode];
      if (!service) return;
      this.ipExtractDialogVisible = false;
      this.$router.push({ name: 'WafPipeline', query: { edit: String(service.port), tab: 'network' } });
    },
  },
});
</script>

<style lang="less" scoped>
@import '@/style/variables';

.t-button + .t-button {
  margin-left: @spacer;
}

.ipl-link {
  color: var(--td-brand-color);
  cursor: pointer;
}

.ipl-link:hover {
  color: var(--td-brand-color-hover);
  text-decoration: underline;
}

.log-config-hint {
  margin-top: 4px;
  font-size: 13px;
  color: var(--td-warning-color);
  line-height: 1.4;
}

.visit-filter-form {
  margin: 0;
}
.visit-filter-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 16px;
}
.visit-filter-grid :deep(.el-form-item) {
  margin: 0;
  min-width: 0;
}
.visit-filter-grid :deep(.el-form-item__content > .el-input),
.visit-filter-grid :deep(.el-form-item__content > .el-select) {
  width: 100%;
  min-width: 0;
}
.visit-search-actions,
.visit-result-actions,
.visit-support,
.visit-settings-tools {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.visit-search-actions {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--hw-line);
}
.visit-search-actions :deep(.el-button),
.visit-result-actions :deep(.el-button) {
  margin: 0;
}
.visit-custom-range {
  margin-top: 16px;
}
.visit-custom-range :deep(.el-form-item) {
  margin: 0;
}
.visit-custom-range :deep(.el-date-editor) {
  width: min(100%, 480px);
}
.visit-advanced {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--hw-line);
}
.visit-support {
  grid-column: 1 / -1;
}
.visit-results-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin: 24px 0 16px;
}
.visit-results-heading h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
.visit-results-heading p {
  margin: 6px 0 0;
  color: var(--hw-muted);
  font-size: 13px;
}
.visit-result-count {
  margin-left: 8px;
  color: var(--hw-muted);
  font-size: 14px;
  font-weight: 400;
}
.visit-notice {
  margin-top: 16px;
}
.visit-outcome {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 4px;
}
.visit-outcome > span:last-child:not(.el-tag) {
  font-size: 12px;
  color: var(--hw-muted);
}
.visit-request-path {
  display: flex;
  gap: 8px;
  align-items: center;
  min-width: 0;
}
.visit-request-path > span:first-child {
  flex: none;
  color: var(--hw-muted);
  font-size: 12px;
}
.visit-request-path > span:last-child {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.visit-response-error {
  color: var(--el-color-danger);
}
.visit-compact-record {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 6px 12px;
  margin: 4px 0;
  font-size: 13px;
}
.visit-compact-record dt {
  color: var(--hw-muted);
}
.visit-compact-record dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}
.visit-compact-reason {
  display: block;
  margin-top: 4px;
  color: var(--hw-muted);
}
.visit-settings-intro {
  color: var(--hw-muted);
  margin: 0 0 12px;
}
.visit-settings-tools {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--hw-line);
  color: var(--hw-muted);
  font-size: 13px;
}
.log-config-form :deep(.el-row) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin: 0 !important;
  gap: 16px;
}
.log-config-form :deep(.el-col) {
  max-width: 100%;
  width: 100%;
  padding: 0 !important;
}
.log-config-form :deep(.el-form-item__label) {
  font-size: 13px;
}
.log-config-form :deep(.el-row:has(textarea)) {
  grid-template-columns: 1fr;
}
.visit-log-card :deep(.el-table td.el-table__cell) {
  padding: 10px 0;
}
.visit-log-card :deep(.el-table .cell) {
  line-height: 1.5;
}
@media (max-width: 1180px) {
  .visit-filter-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 1023px) {
  .visit-filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .visit-results-heading {
    align-items: flex-start;
  }
}
@media (max-width: 520px) {
  .visit-filter-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .visit-filter-grid > :first-child { grid-column: 1 / -1; }
  .log-config-form :deep(.el-row) {
    grid-template-columns: 1fr;
  }
  .visit-search-actions,
  .visit-result-actions {
    width: 100%;
  }
  .visit-search-actions :deep(.el-button:not(.is-text):not(.is-link)) {
    flex: 1;
  }
}
</style>
