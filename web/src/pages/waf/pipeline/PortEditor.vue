<template>
  <div class="port-editor" :class="{ 'is-new': isNew }" data-testid="port-editor" :aria-busy="busy">
    <ElAlert v-if="error" :title="error" type="error" :closable="false" class="space" />
    <GuidedSetup
      v-if="isNew"
      :config="config"
      :templates="templates"
      @saved="emit('saved', $event)"
      @busy="emit('busy', $event)"
    />
    <template v-else>
      <div class="editor-top">
        <div>
          <strong>{{ draft.name || '未命名服务' }}</strong
          ><code>{{ draft.protocol.toUpperCase() }} :{{ draft.port }}</code>
        </div>
        <span>配置版本 {{ draft.revision }}</span
        ><ElTag>{{ draft.listen_enabled ? '监听配置：启用' : '监听配置：停用' }}</ElTag>
      </div>
      <EditorTabs
        v-model="activeTab"
        class="port-sections"
        :inert="busy"
        label="服务配置分类"
        :items="[
          { value: 'common', label: '连接与访问' },
          { value: 'access', label: '监听入口' },
          { value: 'gate', label: '认证与账号' },
          { value: 'ip_group', label: '访问名单' },
          { value: 'ruleset', label: '检测规则' },
          { value: 'defense', label: '异常访问处置' },
          { value: 'hit_policy', label: '自动处罚' },
          { value: 'network', label: '代理与证书' },
          { value: 'explain', label: '请求诊断' },
          { value: 'templates', label: '模板与高级' },
        ]"
        ><ElTabPane name="common" label="连接与访问"
          ><template #label
            ><span class="editor-tab-label">连接与访问<small>名称、上游与允许来源</small></span></template
          ><EditorSection
            title="服务连接与访问范围"
            description="修改名称、现有地址和允许来源。只有保存后才会更新当前服务。"
          >
            <section class="common-settings">
              <Fields
                :model="draft"
                :definitions="[
                  { path: 'name', label: '服务名称', help: '为这个服务取一个容易识别的名称。' },
                  { path: 'upstream', label: '现有服务地址', help: '填写完整的 HTTP 或 HTTPS 地址。' },
                ]"
              />
              <AccessRestrictions
                :policy="draft.policy"
                :disabled="busy"
                @invalid="invalid.accessRestrictions = $event"
              />
              <div class="common-setting-row">
                <div>
                  <h3>允许业务访问</h3>
                  <p>关闭后暂停这个服务的业务请求，管理控制台仍可使用。</p>
                </div>
                <ElSwitch v-model="draft.policy.access.enabled" aria-label="允许业务访问" />
              </div>
            </section> </EditorSection
        ></ElTabPane>
        <ElTabPane name="access" label="监听入口"
          ><template #label
            ><span class="editor-tab-label">监听入口<small>协议、域名与关闭方式</small></span></template
          ><EditorSection title="监听入口" description="端口号保持不变；更改监听相关参数前，按服务端要求停止监听。"
            ><Fields
              :model="draft"
              :definitions="
                basicFields.filter((field) => ['protocol', 'host_names'].includes(field.path))
              " /></EditorSection
          ><EditorSection title="关闭业务访问时"
            ><Fields
              :model="draft.policy.access"
              :definitions="fields.access.filter((field) => field.path !== 'enabled')"
            />
            <p class="muted">
              最近访问开关变更：{{ timeText(config.access_changed_at) }} ·
              {{ config.access_changed_by || '未记录操作人' }}
            </p></EditorSection
          ></ElTabPane
        >
        <ElTabPane name="gate" label="认证与账号"
          ><template #label
            ><span class="editor-tab-label">认证与账号<small>登录方式、账号和会话</small></span></template
          >
          <EditorSection title="认证方式"
            ><Fields
              :model="draft.policy.gate"
              :definitions="
                fields.gate.filter((field) =>
                  ['enabled', 'require_token', 'mode', 'title', 'hint'].includes(field.path),
                )
              "
          /></EditorSection>
          <details class="editor-advanced" :open="!!invalid.gate">
            <summary>会话时限、免登录范围与认证限制</summary>
            <Fields
              :model="draft.policy.gate"
              :definitions="
                fields.gate.filter(
                  (field) => !['enabled', 'require_token', 'mode', 'title', 'hint'].includes(field.path),
                )
              "
              @invalid="invalid.gate = $event"
            />
          </details>
          <label class="space"
            >登录页Logo（PNG/JPEG，64KiB以内，宽高不超过512像素）<input
              type="file"
              accept="image/png,image/jpeg"
              aria-label="登录页Logo"
              @change="setGateLogo" /></label
          ><img
            v-if="draft.policy.gate.logo_data"
            :src="draft.policy.gate.logo_data"
            alt="登录页Logo预览"
            style="display: block; max-width: 96px; max-height: 96px"
          /><ElButton v-if="draft.policy.gate.logo_data" link @click="draft.policy.gate.logo_data = ''"
            >清除Logo</ElButton
          >
          <p class="muted">Logo嵌入登录页，不从外部地址加载，也不展示上游信息。</p>
          <ElAlert
            v-if="draft.policy.gate.mode === 'basic'"
            title="Basic占用Authorization；若业务也使用Basic，请改用表单、trusted网段、免登录路径或独立服务令牌。"
            type="warning"
            :closable="false"
            class="space"
          />
          <ServiceIdentityPanel
            :port="draft.port"
            :revision="draft.revision"
            @revision="draft.revision = $event"
            @busy="busy = $event"
          />
        </ElTabPane>
        <ElTabPane name="ip_group" label="访问名单"
          ><template #label
            ><span class="editor-tab-label">访问名单<small>手动名单与匹配策略</small></span></template
          ><EditorSection
            ><template #title>名单规则</template
            ><Fields :model="draft.policy.ip_group" :definitions="fields.ip_group" /><ElAlert
              title="手动 deny 优先于 trusted。trusted 免检和免自动处罚；区域白名单默认 allow_only，仍需登录并经过规则。地区和共享 IP 组在常用设置中配置；所有黑名单先于 trusted。"
              :closable="false"
              class="space" /><ElAlert
              v-if="hasMixedLists"
              title="同时存在 deny 与 trusted 条目：若范围重叠，deny 优先。请在请求解释中验证家人的真实来源，避免误封。"
              type="warning"
              :closable="false"
              class="space"
          /></EditorSection>
          <EditorSection
            ><template #title>手动名单 <ElButton size="small" @click="addIP">添加条目</ElButton></template
            ><ElTable :data="draft.policy.ip_group.entries || []" size="small">
              <ElTableColumn label="IP / CIDR / 范围" min-width="170"
                ><template #default="{ row }"
                  ><ElInput v-model="row.value" placeholder="192.168.1.0/24" aria-label="名单IP" /></template
              ></ElTableColumn>
              <ElTableColumn label="类型" width="125"
                ><template #default="{ row }"
                  ><ElSelect v-model="row.type" aria-label="名单类型"
                    ><ElOption label="白名单" value="allow" /><ElOption
                      label="黑名单"
                      value="deny" /></ElSelect></template
              ></ElTableColumn>
              <ElTableColumn label="信任级别" width="160"
                ><template #default="{ row }"
                  ><ElSelect v-model="row.trust" :disabled="row.type === 'deny'" aria-label="信任级别"
                    ><ElOption label="trusted · 可信免检" value="trusted" /><ElOption
                      label="allow_only · 继续检测"
                      value="allow_only" /></ElSelect></template
              ></ElTableColumn>
              <ElTableColumn label="到期时间" width="195"
                ><template #default="{ row }"
                  ><input
                    type="datetime-local"
                    :value="localDate(row.expires_at)"
                    aria-label="名单到期时间"
                    @change="row.expires_at = epoch(($event.target as HTMLInputElement).value)" /></template
              ></ElTableColumn>
              <ElTableColumn label="备注" min-width="130"
                ><template #default="{ row }"><ElInput v-model="row.remark" aria-label="名单备注" /></template
              ></ElTableColumn>
              <ElTableColumn width="70"
                ><template #default="{ $index }"
                  ><ElButton link type="danger" @click="draft.policy.ip_group.entries.splice($index, 1)"
                    >移除</ElButton
                  ></template
                ></ElTableColumn
              > </ElTable
            ><ElButton class="space" @click="activeTab = 'common'">配置地区与共享 IP 组</ElButton></EditorSection
          ></ElTabPane
        >
        <ElTabPane name="ruleset" label="检测规则"
          ><template #label
            ><span class="editor-tab-label">检测规则<small>协议限速与攻击检测</small></span></template
          >
          <ElCollapse v-model="rulePanels">
            <ElCollapseItem title="协议 / QPS / CC / BOT（先执行）" name="controls"
              ><Fields :model="draft.policy.ruleset" :definitions="fields.ruleset" @invalid="invalid.ruleset = $event"
            /></ElCollapseItem>

            <ElCollapseItem title="攻击检测（登录成功后继续执行）" name="attacks">
              <ElAlert
                title="先观察命中原因和正常业务，再逐条收紧。观察模式不阻断，也不会进入自动处罚。"
                :closable="false"
                class="space"
              />
              <Fields
                :model="draft.policy.ruleset"
                :definitions="[
                  {
                    path: 'plugins',
                    label: '签名插件策略',
                    kind: 'json',
                    nullable: true,
                    help: 'mode为off/observe/block；ids为插件ID数组（最多4个）；timeout_ms为共享检测预算内的RPC时限（0使用剩余预算，最大1000毫秒）；scope可限定路径。每端口独立进程，异常只观察并放行；dry-run不运行插件。',
                  },
                ]"
                @invalid="invalid.plugins = $event"
              />
              <h3>加密插件启动配置</h3>
              <p class="muted">
                JSON对象以插件ID为键，值包含binary（已签名的本地可执行文件路径）和params（配置对象）。只更新填写的ID，留空保留；删除策略中的ID会清除对应配置。配置加密保存，不回显、不随模板导出。插件在登录门之后执行，仅接收请求元数据；不要在插件参数中启用未经配置的联网行为。
              </p>
              <ElInput
                v-model="pluginProfilesJSON"
                type="textarea"
                :rows="4"
                aria-label="加密插件配置JSON"
                autocomplete="off"
              /><ElButton v-if="draft.policy.ruleset.plugins" class="space" @click="reloadPlugins"
                >保存时重新加载插件</ElButton
              >
              <Fields
                :model="draft.policy.ruleset"
                :definitions="[
                  {
                    path: 'scripts',
                    label: '自定义GRL规则JSON',
                    kind: 'json-array',
                    help: '每项id、source、mode和severity。只求值条件；登录后按优先级仲裁。Ban产生异步命中事件，观察不处罚。GetIPFailureCount使用端口独立的内存窗口。',
                  },
                  {
                    path: 'urls',
                    label: 'URL黑白名单JSON',
                    kind: 'json-array',
                    help: '每项id、match（exact/prefix/suffix/contains）、value、action（allow/deny）和mode。黑名单优先；白名单只跳过后续攻击检测，仍须登录。',
                  },
                ]"
                @invalid="invalid.extensions = $event"
              />
              <Fields
                :model="draft"
                :definitions="[
                  {
                    path: 'script_groups',
                    label: '自定义规则IP组快照JSON',
                    kind: 'json',
                    help: '组名或组code映射到IP/CIDR数组；只影响本端口，与原共享IP组脱钩。',
                  },
                ]"
                @invalid="invalid.scriptGroups = $event"
              />
              <ElTable :data="draft.policy.ruleset.rules" size="small"
                ><ElTableColumn label="规则" min-width="180"
                  ><template #default="{ row }"
                    ><strong>{{ ruleNames[row.category] || row.category }}</strong
                    ><small class="rule-help">{{ ruleHelp[row.category] }}</small></template
                  ></ElTableColumn
                ><ElTableColumn label="模式" width="130"
                  ><template #default="{ row }"
                    ><ElSelect v-model="row.mode" :aria-label="`${ruleNames[row.category]}模式`"
                      ><ElOption
                        v-for="m in modes"
                        :key="m[0]"
                        :label="m[1]"
                        :value="m[0]" /></ElSelect></template></ElTableColumn
                ><ElTableColumn label="严重度" width="130"
                  ><template #default="{ row }"
                    ><ElSelect v-model="row.severity" aria-label="规则严重度"
                      ><ElOption
                        v-for="severity in ['low', 'medium', 'high', 'critical']"
                        :key="severity"
                        :label="severity"
                        :value="severity" /></ElSelect></template></ElTableColumn
                ><ElTableColumn label="范围 / 例外" width="120"
                  ><template #default="{ row }"
                    ><ElButton link @click="openRule(row)">配置范围</ElButton></template
                  ></ElTableColumn
                ></ElTable
              >
              <h4>CRS 作用范围</h4>
              <Scope :model="draft.policy.ruleset.owasp_scope" />
            </ElCollapseItem>
          </ElCollapse>
          <EditorSection title="从观察调整为拦截"
            ><p class="muted">
              先从处罚列表和trace确认不是正常业务，再选择具体规则。此向导不自动打开自动封禁或登录门。
            </p>
            <ElSelect v-model="tightenRules" multiple placeholder="选择已验证的观察规则" aria-label="渐进收紧规则"
              ><ElOption
                v-for="rule in draft.policy.ruleset.rules.filter((r) => r.mode === 'observe')"
                :key="rule.id"
                :label="ruleNames[rule.category] || rule.id"
                :value="rule.id" /></ElSelect
            ><ElButton class="space" @click="tighten">将所选规则设为拦截</ElButton>
          </EditorSection></ElTabPane
        >
        <ElTabPane name="defense" label="异常访问处置">
          <DefenseSettings :config="draft" :original="config" :busy="busy" :dirty="dirty" @invalid="invalid.defense = $event" @migration="defenseMigrationConfirmed = $event" @committed="emit('saved', $event)" />
        </ElTabPane>
        <ElTabPane name="hit_policy" label="自动处罚"
          ><template #label
            ><span class="editor-tab-label">自动处罚<small>阈值、封禁与自保</small></span></template
          ><EditorSection
            ><template #title>自动处置</template
            ><ElAlert
              :title="draft.policy.defense ? '以下设置管理单来源封禁，trusted 和受保护来源保留豁免。整个端口的阈值与恢复时间在异常访问处置中配置。' : '第4步是异步旁路：只改变后续请求。当前请求动作由第3步决定。trusted和受保护来源只告警，不被自动处罚。'"
              :closable="false"
              class="space" /><Fields :model="draft.policy.hit_policy" :definitions="draft.policy.defense ? fields.hit_policy.filter(field => !field.path.startsWith('lockdown.') && field.path !== 'ban_scope') : fields.hit_policy"
          /></EditorSection>
          <EditorSection
            ><template #title>单IP累计阈值</template>
            <p class="muted">再次违规时长×4，上限7天，历史保留30天。以下时间单位均为秒。</p>
            <ElTable :data="thresholdRows" size="small"
              ><ElTableColumn prop="label" label="类别" /><ElTableColumn label="窗口"
                ><template #default="{ row }"
                  ><ElInputNumber
                    v-model="draft.policy.hit_policy[row.key].window_seconds"
                    :min="1"
                    :max="86400" /></template></ElTableColumn
              ><ElTableColumn label="命中次数"
                ><template #default="{ row }"
                  ><ElInputNumber
                    v-model="draft.policy.hit_policy[row.key].count"
                    :min="1"
                    :max="row.key === 'response_404' ? 128 : 100000" /></template></ElTableColumn
              ><ElTableColumn label="首次封禁时长"
                ><template #default="{ row }"
                  ><ElInputNumber
                    v-if="row.ban"
                    v-model="draft.policy.hit_policy[row.key].ban_seconds"
                    :min="1"
                    :max="604800"
                  /><span v-else>导入蜜罐，使用下方TTL</span></template
                ></ElTableColumn
              ></ElTable
            >
            <p class="muted">提示、命中后丢弃和请求归档已移到独立的蜜罐设置。</p>
            <ElButton @click="activeTab = 'defense'">配置异常访问处置</ElButton></EditorSection
          >
          <EditorSection
            ><template #title>可选业务登录观察（JSON）</template>
            <p class="muted">
              默认空列表。每条配置
              path、method、username_field、password_field、success_statuses、可选success_location_prefix/success_body_contains、body_prefix_bytes（1–8192）。响应正文只采样，不缓存整包。
            </p>
            <ElInput
              v-model="businessLoginJSON"
              type="textarea"
              :rows="6"
              aria-label="业务登录观察JSON"
              @change="parseBusinessLogin"
            />
            <p v-if="invalid.businessLogin" role="alert" class="error">
              业务登录观察必须是合法JSON数组。
            </p></EditorSection
          ></ElTabPane
        >
        <ElTabPane name="network" label="代理与证书"
          ><template #label
            ><span class="editor-tab-label">代理与证书<small>转发、TLS 和响应策略</small></span></template
          ><EditorSection
            ><template #title>连接设置</template
            ><ElAlert
              title="WebSocket、SSE、Range和大文件保持流式。上游TLS默认校验；私有CA可在下方显式配置。"
              :closable="false"
              class="space" /><EditorSection title="入口与代理信任"
              ><Fields
                :model="draft.policy.network"
                :definitions="
                  fields.network.filter((field) =>
                    [
                      'listen_ip_version',
                      'trusted_proxies',
                      'real_ip_header',
                      'preserve_host',
                      'disable_http2',
                      'max_proxy_hops',
                    ].includes(field.path),
                  )
                "
            /></EditorSection>
            <details class="editor-advanced" :open="!!invalid.network">
              <summary>连接限额、超时与响应高级参数</summary>
              <Fields
                :model="draft.policy.network"
                :definitions="
                  fields.network.filter(
                    (field) =>
                      ![
                        'listen_ip_version',
                        'trusted_proxies',
                        'real_ip_header',
                        'preserve_host',
                        'disable_http2',
                        'max_proxy_hops',
                      ].includes(field.path),
                  )
                "
                @invalid="invalid.network = $event"
              /></details
          ></EditorSection>
          <EditorSection
            ><template #title>网页防篡改基线</template>
            <p class="muted">
              响应策略的tamper数组配置id、path、hosts、mode、action（replace/alert）、ignore_query和max_bytes。每端口基线总额1MiB；只从完整200响应学习，基线加密且不随模板导出。重新学习将在保存后使用下一次合格响应，请先确认上游页面正确。
            </p>
            <ElTable :data="draft.policy.network.response?.tamper || []" size="small"
              ><ElTableColumn prop="id" label="规则" /><ElTableColumn prop="path" label="精确路径" /><ElTableColumn
                label="基线更新时间"
                ><template #default="{ row }">{{
                  row.learned_at ? timeText(row.learned_at) : '待学习'
                }}</template></ElTableColumn
              ><ElTableColumn label="操作"
                ><template #default="{ row }"
                  ><ElButton link @click="relearnTamper(row)">重新学习</ElButton></template
                ></ElTableColumn
              ></ElTable
            ><Fields
              :model="draft"
              :definitions="[
                {
                  path: 'upstream_dial_ip',
                  label: '默认上游固定连接IP',
                  help: '可选。仅改变连接目标，Host和TLS SNI仍使用上游URL主机名。',
                },
                {
                  path: 'routes',
                  label: '同端口业务路由JSON',
                  kind: 'json-array',
                  help: '每项包含hosts、可选path_prefix、upstream、可选dial_ip。所有路由共用本端口安全策略、账号和计数器；匹配顺序为主机名精度与最长路径前缀。',
                },
              ]"
              @invalid="invalid.routes = $event"
          /></EditorSection>
          <EditorSection
            ><template #title>证书与信任</template>
            <p class="muted">
              迁入证书继续跟随源站点证书夹续期；新握手热更新，不中断现有连接。手动提交新证书会解除源证书绑定；留空保留。HTTP-01仅响应迁入主机已登记的本地挑战，访问总开关仍有效。
            </p>
            <div class="two-columns">
              <label
                >TLS证书PEM（留空保留）<ElInput v-model="secrets.tls_certificate" type="textarea" :rows="4" /></label
              ><label
                >TLS私钥PEM（留空保留）<ElInput v-model="secrets.tls_private_key" type="textarea" :rows="4" /></label
              ><label
                >上游私有CA PEM（留空保留）<ElInput v-model="secrets.upstream_ca" type="textarea" :rows="4" /></label
              ><label
                ><ElCheckbox v-model="secrets.clear_upstream_ca">清除已保存的上游私有CA</ElCheckbox
                ><small>清除后只使用系统信任库。</small></label
              >
            </div>
            <ElButton class="space" @click="testUpstream">测试上游TCP连通性</ElButton></EditorSection
          >
          <EditorSection
            ><template #title>加密业务头</template>
            <p class="muted">
              默认配置引用：{{ draft.header_profile || '无' }}。路由可通过 header_profile
              引用独立配置。头值可能含有上游令牌，保存后不回显、不随模板导出；留空保留。修改示例：{"配置名称":{"request":[{"header_name":"Authorization","header_value":"Bearer
              新令牌"}],"response":[]}}。只更新填写的配置名称。
            </p>
            <ElInput
              v-model="businessHeadersJSON"
              type="textarea"
              :rows="4"
              aria-label="加密业务头JSON"
              autocomplete="off"
          /></EditorSection>
          <EditorSection
            ><template #title>响应隐私脱敏</template>
            <p class="muted">
              在响应策略的privacy对象设置mode及urls（match: exact/prefix/suffix/contains，value:
              URL）。复用本地DLP引擎；观察不修改正文，不输出原始识别结果。高级YAML配置加密保存、不回显、不随模板导出，留空保留；没有自定义配置时使用内置规则。
            </p>
            <ElInput
              v-model="secrets.privacy_engine_config"
              type="textarea"
              :rows="4"
              aria-label="隐私引擎配置"
              autocomplete="off"
            /><ElCheckbox v-model="secrets.clear_privacy_engine_config"
              >清除自定义隐私配置并使用内置规则</ElCheckbox
            ></EditorSection
          ></ElTabPane
        >
        <ElTabPane name="explain" label="请求诊断"
          ><template #label
            ><span class="editor-tab-label">请求诊断<small>检查当前草稿的判定</small></span></template
          ><EditorSection
            ><template #title>请求解释（不产生处罚）</template>
            <div class="two-columns">
              <label>来源IP<ElInput v-model="sample.client_ip" aria-label="解释来源IP" /></label
              ><label>方法<ElInput v-model="sample.method" /></label
              ><label>路径<ElInput v-model="sample.path" aria-label="解释路径" /></label
              ><label>请求头JSON<ElInput v-model="sampleHeaders" type="textarea" :rows="3" /></label
              ><label>样例正文<ElInput v-model="sample.body" type="textarea" :rows="4" /></label>
            </div>
            <ElButton class="space" :loading="busy" @click="explain">解释当前草稿</ElButton
            ><Trace :result="preview" /></EditorSection
        ></ElTabPane>
        <ElTabPane name="templates" label="模板与高级"
          ><template #label
            ><span class="editor-tab-label">模板与高级<small>差异应用与完整策略</small></span></template
          ><EditorSection
            ><template #title>应用模板 / 从模板更新</template>
            <p class="muted">
              来源：{{ draft.template?.id || '独立配置' }} · 版本
              {{ draft.template?.version || '—' }}。先查看差异，再只覆盖选中分区；名单条目、可信代理、凭据会保留。
            </p>
            <ElSelect v-model="selectedTemplate" aria-label="待应用模板"
              ><ElOption
                v-for="tpl in templates"
                :key="tpl.id"
                :label="`${tpl.name} · v${tpl.version}`"
                :value="tpl.id" /></ElSelect
            ><ElCheckboxGroup v-model="selectedSections" class="space"
              ><ElCheckbox v-for="section in sections" :key="section" :value="section">{{
                sectionNames[section]
              }}</ElCheckbox></ElCheckboxGroup
            ><ElButton @click="previewTemplate">查看模板差异</ElButton>
            <div v-if="templatePreview" class="space">
              <ElAlert :title="`有${templatePreview.diff?.length || 0}个分区与模板不同`" :closable="false" />
              <details v-for="diff in templatePreview.diff || []" :key="diff.section">
                <summary>{{ sectionNames[diff.section] }}</summary>
                <div class="two-columns">
                  <pre>{{ JSON.stringify(diff.current, null, 2) }}</pre>
                  <pre>{{ JSON.stringify(diff.template, null, 2) }}</pre>
                </div>
              </details>
              <ElButton type="primary" @click="acceptTemplate">确认应用选中分区到草稿</ElButton>
            </div>
            <details class="space">
              <summary>高级：编辑完整策略JSON（不含口令/私钥）</summary>
              <p v-if="draft.policy.defense" class="muted">蜜罐内容在独立页面管理；此处 honeypot 是只读迁移备份。已迁移服务不再启用旧多来源自保或跨端口自动封禁。</p>
              <p class="muted">
                可设置所有范围、参数和观察条件。服务端严格验证字段与数值；这里不会修改监听端口或凭据。
              </p>
              <ElButton size="small" @click="advancedJSON = JSON.stringify(draft.policy, null, 2)"
                >读取当前策略</ElButton
              ><ElInput v-model="advancedJSON" type="textarea" :rows="12" aria-label="完整策略JSON" /><ElButton
                class="space"
                @click="applyAdvanced"
                >校验JSON并写入草稿</ElButton
              >
            </details></EditorSection
          ></ElTabPane
        ></EditorTabs
      >
      <div class="editor-actions">
        <div>
          <span v-if="Object.values(invalid).some(Boolean)" class="field-error" role="alert"
            >有配置尚未通过校验。<ElButton text @click="locateInvalid">定位未完成的配置</ElButton></span
          ><span v-else>{{ dirty ? '有未保存的修改' : '当前配置已读取' }}</span
          ><small>保存只更新当前服务；账号和令牌操作单独即时生效。</small>
        </div>
        <ElButton :disabled="busy" @click="emit('close')">关闭</ElButton
        ><ElButton type="primary" :loading="busy" :disabled="Object.values(invalid).some(Boolean)" @click="save"
          >保存端口配置</ElButton
        >
      </div>
    </template>

    <ElDialog v-model="ruleVisible" :title="`规则范围：${activeRule?.id || ''}`" width="min(850px,94vw)" append-to-body>
      <template v-if="activeRule"
        ><Scope :model="activeRule.scope" /><Fields
          v-if="activeRule.category === 'scanner'"
          :model="activeRule.params"
          :definitions="[
            {
              path: 'probe_paths',
              label: '本站不存在的探测路径',
              kind: 'lines',
              help: '每行一个路径；存在的管理页、WordPress登录页不要填入。',
            },
          ]" /></template
      ><template #footer><ElButton type="primary" @click="ruleVisible = false">返回配置</ElButton></template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue';
import { v4 as uuidv4 } from 'uuid';
import type { PropType } from 'vue';
import { ElAlert, ElButton, ElCheckbox, ElCheckboxGroup, ElCollapse, ElCollapseItem, ElDialog, ElInput, ElInputNumber, ElMessage, ElMessageBox, ElOption, ElSelect, ElSwitch, ElTable, ElTableColumn, ElTabPane, ElTag } from 'element-plus';
import { pipelinePost } from '@/apis/pipeline';
import { clone, fields, modes, ruleHelp, ruleNames, sectionNames, sections, type Field, type PolicyTemplate, type PortConfig } from './model';
import GuidedSetup from './GuidedSetup.vue';
import AccessRestrictions from './AccessRestrictions.vue';
import DefenseSettings from './DefenseSettings.vue';
import Fields from './Fields.vue';
import Scope from './Scope.vue';
import Trace from './Trace.vue';
import ServiceIdentityPanel from './ServiceIdentityPanel.vue';
const props = defineProps({
  config: {
    type: Object as PropType<PortConfig & { access_changed_at?: number; access_changed_by?: string }>,
    required: true,
  },
  templates: { type: Array as PropType<PolicyTemplate[]>, required: true },
  isNew: Boolean,
  initialTab: { type: String, default: 'common' },
  initialPanels: { type: Array as PropType<string[]>, default: () => [] },
});
const emit = defineEmits(['saved', 'close', 'busy']);
const draft = ref<PortConfig>(clone(props.config));
const defenseMigrationConfirmed = ref(false);
const busy = ref(false),
  error = ref(''),
  activeTab = ref(props.initialTab === 'honeypot' ? 'defense' : props.initialTab);
function openAdvanced(tab: string, panels?: string[]) {
  activeTab.value = panels?.length === 1 && panels[0] === 'gate' ? 'gate' : tab;
  if (panels) rulePanels.value = panels.filter((panel) => panel !== 'gate');
}
function locateInvalid() {
  const key = Object.keys(invalid).find((key) => invalid[key]);
  if (key === 'defense') {
    activeTab.value = 'defense';
    return;
  }
  if (key === 'accessRestrictions') {
    activeTab.value = 'common';
    return;
  }
  if (key === 'gate') {
    activeTab.value = 'gate';
    return;
  }
  openAdvanced(
    key === 'businessLogin' ? 'hit_policy' : ['network', 'routes'].includes(key || '') ? 'network' : 'ruleset',
    ['controls', 'attacks'],
  );
}
watch(busy, (value) => emit('busy', value), { flush: 'sync' });
const secrets = reactive({
  initial_username: '',
  initial_password: '',
  tls_certificate: '',
  tls_private_key: '',
  upstream_ca: '',
  clear_upstream_ca: false,
  privacy_engine_config: '',
  clear_privacy_engine_config: false,
});
const businessHeadersJSON = ref('');
const pluginProfilesJSON = ref('');
function reloadPlugins() {
  const plugins = draft.value.policy.ruleset.plugins;
  if (plugins) {
    plugins.generation = (plugins.generation || 0) + 1;
    ElMessage.info('已标记重新加载，请保存端口配置');
  }
}
async function setGateLogo(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  await run(async () => {
    if (file.size > 65536 || !['image/png', 'image/jpeg'].includes(file.type))
      throw new Error('请选择64KiB以内的PNG/JPEG');
    const data = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(new Error('Logo读取失败'));
      reader.readAsDataURL(file);
    });
    draft.value.policy.gate.logo_data = data;
    input.value = '';
  });
}
const selectedTemplate = ref(draft.value.template?.id || ''),
  selectedSections = ref([...sections]),
  templatePreview = ref<any>(null),
  preview = ref<any>(null),
  previewKey = ref('');
const invalid = reactive<Record<string, boolean>>({}),
  rulePanels = ref(props.initialPanels.length ? [...props.initialPanels] : ['controls', 'attacks']),
  activeRule = ref<any>(null),
  ruleVisible = ref(false);
const sample = reactive({ client_ip: '198.51.100.25', method: 'GET', path: '/', body: '' }),
  sampleHeaders = ref('{"Accept":"text/html"}');
const tightenRules = ref<string[]>([]),
  advancedJSON = ref(''),
  businessLoginJSON = ref('[]');
const templatePreviewKey = ref('');
const thresholdRows = [
  { key: 'critical', label: 'critical', ban: true },
  { key: 'high', label: 'high', ban: true },
  { key: 'medium', label: 'medium', ban: true },
  { key: 'login_ban', label: '持续登录失败', ban: true },
  { key: 'response_404', label: '不同路径404', ban: false },
  { key: 'response_auth', label: '401/403异常', ban: false },
  { key: 'response_5xx', label: '可归因5xx', ban: false },
];
const basicFields: Field[] = [
  {
    path: 'port',
    label: '监听端口',
    kind: 'number',
    min: 1,
    max: 65535,
    help: '每个端口独立配置，不能占用管理端口或已有站点端口。',
  },
  { path: 'name', label: '端口名称', help: '例如家庭相册、NAS文件。' },
  {
    path: 'protocol',
    label: '监听协议',
    kind: 'select',
    options: [
      ['http', 'HTTP'],
      ['https', 'HTTPS'],
    ],
    help: 'HTTPS需要证书与私钥。',
  },
  { path: 'upstream', label: '上游地址', help: '明确填写http(s)://主机:端口，不能在URL中夹带账号密码。' },
  {
    path: 'host_names',
    label: '允许的主机名',
    kind: 'lines',
    help: '每行一个，不含端口；留空接受所有Host。同一端口的所有域名共用安全策略。',
  },
];
function normalize() {
  const p = draft.value.policy;
  p.ip_group.entries ||= [];
  p.ip_group.regions ||= [];
  p.ruleset.owasp_scope ||= {};
  for (const r of p.ruleset.rules || []) {
    r.scope ||= {};
    r.params ||= {};
  }
  const defaults = {
    login_ban: [600, 20, 1800],
    response_404: [60, 20, 0],
    response_auth: [300, 10, 0],
    response_5xx: [300, 5, 0],
  };
  for (const [key, v] of Object.entries(defaults)) {
    if (!p.hit_policy[key]?.count) p.hit_policy[key] = { window_seconds: v[0], count: v[1], ban_seconds: v[2] };
  }
  businessLoginJSON.value = JSON.stringify(p.hit_policy.business_login || [], null, 2);
}
normalize();
const baseline = JSON.stringify(draft.value);
const dirty = computed(
  () =>
    JSON.stringify(draft.value) !== baseline ||
    Object.values(secrets).some(Boolean) ||
    !!businessHeadersJSON.value ||
    !!pluginProfilesJSON.value ||
    Object.values(invalid).some(Boolean),
);
const hasMixedLists = computed(
  () =>
    draft.value.policy.ip_group.entries.some((e) => e.type === 'deny') &&
    draft.value.policy.ip_group.entries.some((e) => e.type === 'allow' && e.trust === 'trusted'),
);
const timeText = (value: number) => (value ? new Date(value * 1000).toLocaleString('zh-CN') : '未记录');
function localDate(value: number) {
  if (!value) return '';
  const d = new Date(value * 1000);
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}
const epoch = (value: string) => (value ? Math.floor(new Date(value).getTime() / 1000) : 0);
async function run(work: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await work();
  } catch (e) {
    if (e !== 'cancel' && e !== 'close') error.value = e.message || String(e);
  } finally {
    busy.value = false;
  }
}
function addIP() {
  draft.value.policy.ip_group.entries.push({
    id: uuidv4(),
    type: 'allow',
    value: '',
    trust: 'trusted',
    expires_at: 0,
    remark: '',
  });
}
function openRule(rule: any) {
  activeRule.value = rule;
  ruleVisible.value = true;
}
async function testUpstream() {
  await run(async () => {
    const message = await pipelinePost('upstream/test', { upstream: draft.value.upstream });
    ElMessage.success(message + '（TCP）');
  });
}
async function relearnTamper(rule: any) {
  await run(async () => {
    await ElMessageBox.confirm('保存后将重新学习下一次合格响应。请先确认上游未被篡改。', '重新学习防篡改基线', {
      type: 'warning',
    });
    rule.generation = (rule.generation || 0) + 1;
    rule.learned_at = 0;
    ElMessage.info('已标记重新学习，请保存端口配置');
  });
}
async function explainNow() {
  const headers = JSON.parse(sampleHeaders.value);
  preview.value = await pipelinePost('explain', { config: draft.value, ...sample, headers });
  previewKey.value = JSON.stringify(draft.value);
}
async function explain() {
  await run(explainNow);
}
async function previewTemplate() {
  await run(async () => {
    if (!selectedTemplate.value || !selectedSections.value.length) throw new Error('请选择模板及至少一个分区');
    templatePreview.value = await pipelinePost('templates/preview', {
      config: draft.value,
      template_id: selectedTemplate.value,
      sections: selectedSections.value,
    });
    templatePreviewKey.value = JSON.stringify(draft.value);
  });
}
async function acceptTemplate() {
  await run(async () => {
    if (templatePreviewKey.value !== JSON.stringify(draft.value)) throw new Error('草稿已变化，请重新预览差异');
    await ElMessageBox.confirm('将选中分区复制到当前草稿，点击保存后才生效。', '确认应用模板', { type: 'warning' });
    draft.value = templatePreview.value.config;
    normalize();
    templatePreview.value = null;
    ElMessage.success('已应用到草稿');
  });
}
async function tighten() {
  await run(async () => {
    if (!tightenRules.value.length) throw new Error('请选择已验证的规则');
    await ElMessageBox.confirm('确认这些命中不是正常业务？所选规则将对匹配请求执行拦截，保存后生效。', '渐进收紧', {
      type: 'warning',
    });
    for (const r of draft.value.policy.ruleset.rules) if (tightenRules.value.includes(r.id)) r.mode = 'block';
    tightenRules.value = [];
  });
}
function parseBusinessLogin() {
  try {
    const list = JSON.parse(businessLoginJSON.value);
    if (!Array.isArray(list)) throw new Error();
    draft.value.policy.hit_policy.business_login = list;
    invalid.businessLogin = false;
  } catch {
    invalid.businessLogin = true;
  }
}
async function applyAdvanced() {
  await run(async () => {
    const policy = JSON.parse(advancedJSON.value);
    if (!policy || sections.some((s) => !policy[s])) throw new Error('策略必须包含全部七个分区');
    const candidate = { ...draft.value, policy };
    await pipelinePost('explain', { config: candidate, ...sample, headers: JSON.parse(sampleHeaders.value) });
    draft.value = candidate;
    normalize();
    ElMessage.success('服务端校验通过，已写入草稿');
  });
}
async function save() {
  await run(async () => {
    parseBusinessLogin();
    if (Object.values(invalid).some(Boolean)) throw new Error('请先修正JSON字段');
    let plugin_profiles;
    if (pluginProfilesJSON.value.trim()) {
      plugin_profiles = JSON.parse(pluginProfilesJSON.value);
      if (!plugin_profiles || typeof plugin_profiles !== 'object' || Array.isArray(plugin_profiles))
        throw new Error('插件启动配置必须是JSON对象');
    }
    let business_headers;
    if (businessHeadersJSON.value.trim()) {
      business_headers = JSON.parse(businessHeadersJSON.value);
      if (!business_headers || typeof business_headers !== 'object' || Array.isArray(business_headers))
        throw new Error('加密业务头必须是JSON对象');
    }
    if (!props.isNew && draft.value.policy.access.enabled !== props.config.policy.access.enabled)
      await ElMessageBox.confirm(
        draft.value.policy.access.enabled
          ? '允许这个端口重新接收业务访问？'
          : '关闭后包括trusted在内的业务请求均被拒绝，已有连接按排空时间关闭。管理端不受影响。',
        '确认切换访问开关',
        { type: 'warning' },
      );
    if (props.isNew) draft.value.listen_enabled = false;
    const result = await pipelinePost<PortConfig>('ports/save', {
      config: draft.value,
      confirm_defense_migration: defenseMigrationConfirmed.value,
      ...secrets,
      business_headers,
      plugin_profiles,
    });
    Object.assign(secrets, {
      initial_username: '',
      initial_password: '',
      tls_certificate: '',
      tls_private_key: '',
      upstream_ca: '',
      clear_upstream_ca: false,
      privacy_engine_config: '',
      clear_privacy_engine_config: false,
    });
    businessHeadersJSON.value = '';
    pluginProfilesJSON.value = '';
    ElMessage.success(props.isNew ? '端口已创建，尚未监听' : '端口配置已保存');
    emit('saved', result);
  });
}
watch([selectedTemplate, () => selectedSections.value.join(',')], () => {
  templatePreview.value = null;
});
</script>

<style scoped>
.port-editor {
  color: var(--hw-ink);
  min-width: 0;
}
.port-editor:not(.is-new) {
  height: min(740px, calc(86dvh - 140px));
  display: flex;
  flex-direction: column;
}
.editor-top {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--hw-line);
  flex-shrink: 0;
}
.editor-top > div {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-right: auto;
}
.editor-top strong {
  font-size: 15px;
  font-weight: 600;
}
.editor-top code,
.editor-top > span {
  font-size: 12px;
  color: var(--hw-muted);
}
.port-sections {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.port-sections :deep(> .el-tabs) {
  flex: 1;
  height: 0;
  min-height: 0;
  overflow: hidden;
  display: flex;
}
.port-sections :deep(.el-tabs--top) {
  flex-direction: column;
}
.port-sections :deep(.el-tabs__header) {
  margin: 0;
  flex-shrink: 0;
  overflow: auto;
}
.port-sections :deep(.el-tabs--left > .el-tabs__header) {
  width: 190px;
  border-right: 1px solid var(--hw-line);
  padding: 10px;
}
.port-sections :deep(.el-tabs--top > .el-tabs__header) {
  padding: 0 16px;
}
.port-sections :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 24px;
}
.editor-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px 24px;
  border-top: 1px solid var(--hw-line);
  background: var(--hw-subtle);
}
.editor-actions > div {
  flex: 1;
  min-width: 180px;
  font-size: 13px;
}
.editor-actions small {
  display: block;
  margin-top: 4px;
}
.two-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.two-columns label {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.muted,
.rule-help,
small {
  color: var(--hw-muted);
  line-height: 1.7;
  font-size: 13px;
}
.rule-help {
  display: block;
  margin-top: 5px;
}
.space {
  margin: 16px 0;
}
h3 {
  margin: 24px 0 12px;
  font-size: 16px;
}
.common-settings h3 {
  margin: 0 0 8px;
}
.common-setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  border-top: 1px solid var(--hw-line);
  margin-top: 24px;
  padding-top: 20px;
}
.common-setting-row p {
  color: var(--hw-muted);
  font-size: 13px;
}
.editor-advanced {
  padding: 16px 0;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--hw-line);
}
summary {
  cursor: pointer;
  font-weight: 500;
  padding: 8px 0;
}
.editor-advanced[open] summary {
  margin-bottom: 16px;
}
pre {
  padding: 12px;
  background: var(--hw-subtle);
  overflow: auto;
  max-height: 320px;
  white-space: pre-wrap;
}
input[type='datetime-local'] {
  padding: 8px;
  border: 1px solid var(--hw-line);
  border-radius: 4px;
  background: var(--hw-panel);
  color: var(--hw-ink);
  max-width: 100%;
}
.field-error {
  color: var(--hw-danger);
}
@media (max-width: 700px) {
  .two-columns {
    grid-template-columns: 1fr;
  }
  .editor-top {
    padding: 12px 16px;
  }
  .port-sections :deep(.el-tabs__content) {
    padding: 16px;
  }
  .editor-actions {
    padding: 12px 16px;
  }
  .editor-actions > div {
    flex-basis: 100%;
  }
  .common-setting-row {
    flex-wrap: wrap;
  }
}
</style>
