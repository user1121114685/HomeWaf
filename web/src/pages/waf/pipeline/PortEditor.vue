<template>
  <div class="port-editor" data-testid="port-editor" :aria-busy="busy">
    <ElAlert v-if="error" :title="error" type="error" :closable="false" class="space" />
    <template v-if="isNew">
      <ElSteps :active="wizardStep" finish-status="success" align-center class="space"><ElStep title="端口与上游" /><ElStep title="选择模板" /><ElStep title="访问账号" /><ElStep title="解释与创建" /></ElSteps>
      <section v-if="wizardStep === 0">
        <Fields :model="draft" :definitions="basicFields" />
        <ElAlert title="新端口创建后不会自动监听。上游测试只检查 TCP 可达性，不验证业务账号或 TLS 信任。" type="info" :closable="false" class="space" />
        <div v-if="draft.protocol === 'https'" class="two-columns"><label>TLS 证书 PEM<ElInput v-model="secrets.tls_certificate" type="textarea" :rows="4" aria-label="TLS 证书 PEM" /></label><label>TLS 私钥 PEM<ElInput v-model="secrets.tls_private_key" type="textarea" :rows="4" aria-label="TLS 私钥 PEM" /><small>只提交到实例加密存储，不导出到模板。</small></label></div>
      </section>
      <section v-if="wizardStep === 1">
        <label>起始模板<ElSelect v-model="selectedTemplate" aria-label="起始模板" @change="applyWizardTemplate"><ElOption label="保留当前草稿" value="" /><ElOption v-for="tpl in templates" :key="tpl.id" :label="tpl.name" :value="tpl.id" /></ElSelect></label>
        <div class="policy-summary"><span>陌生来源：{{ draft.policy.ip_group.unlisted_action }}</span><span>认证：{{ draft.policy.gate.require_token ? '必须使用服务令牌' : draft.policy.gate.enabled ? '需要账号' : '关闭' }}</span><span>自动处罚：{{ draft.policy.hit_policy.enabled ? '开启' : '关闭' }}</span></div>
        <p class="muted">模板应用即复制，之后独立调整；模板更新不会自动改变这个端口。</p>
      </section>
      <section v-if="wizardStep === 2">
        <ElSwitch v-model="draft.policy.gate.enabled" :disabled="draft.policy.gate.require_token" active-text="启用本端口登录门" aria-label="新端口登录门" />
        <ElAlert v-if="draft.policy.gate.require_token" title="此模板必须使用服务令牌。创建草稿后，到第3步登录门设置生成令牌，再启用监听；缺失或错误令牌返回401，不回源。" type="warning" :closable="false" class="space" />
        <ElAlert v-if="draft.policy.gate.enabled" title="必须建立至少一个账号；口令至少10字符，不能是弱口令或与用户名相同。不同端口账号独立。" type="warning" :closable="false" class="space" />
        <div v-if="draft.policy.gate.enabled" class="two-columns"><label>初始账号<ElInput v-model="secrets.initial_username" autocomplete="off" aria-label="初始账号" maxlength="128" /></label><label>初始口令<ElInput v-model="secrets.initial_password" type="password" show-password autocomplete="new-password" aria-label="初始口令" maxlength="72" /></label></div>
        <p v-else-if="!draft.policy.gate.require_token" class="muted">业务将依靠 IP 组与攻击检测保护；也可稍后增加本端口账号再开启登录门。</p>
      </section>
      <section v-if="wizardStep === 3">
        <p>模拟一个陌生来源访问 <code>/</code>。此操作不回源、不建立会话、不产生处罚。</p>
        <ElButton :loading="busy" @click="explain">重新解释当前配置</ElButton><Trace :result="preview" class="space" />
        <ElAlert title="创建后状态为「未监听」。回到总览确认预览后再启用；观察启动只放宽攻击检测与处罚，登录门和IP访问限制仍然有效。" type="info" :closable="false" class="space" />
      </section>
      <div class="editor-actions"><ElButton v-if="wizardStep > 0" :disabled="busy" @click="wizardStep--">上一步</ElButton><ElButton v-if="wizardStep < 3" type="primary" :loading="busy" @click="nextWizard">{{ wizardStep === 0 ? '检查并继续' : '下一步' }}</ElButton><ElButton v-else type="primary" :disabled="!preview || previewKey !== JSON.stringify(draft)" :loading="busy" @click="save">创建为未监听</ElButton></div>
    </template>
    <template v-else>
      <div class="editor-top"><strong>{{ draft.protocol.toUpperCase() }} :{{ draft.port }} · {{ draft.name || '未命名端口' }}</strong><ElTag>{{ draft.listen_enabled ? '正在监听' : '未监听' }}</ElTag><span>配置版本 {{ draft.revision }}</span></div>
      <ElTabs v-model="activeTab">
        <ElTabPane label="1 · 访问开关" name="access"><Fields :model="draft.policy.access" :definitions="fields.access" /><p class="muted">最近访问开关变更：{{ timeText(config.access_changed_at) }} · {{ config.access_changed_by || '未记录操作人' }}</p><h3>端口基本信息</h3><Fields :model="draft" :definitions="basicFields.filter(f => f.path !== 'port')" /></ElTabPane>
        <ElTabPane label="2 · IP 组" name="ip_group">
          <Fields :model="draft.policy.ip_group" :definitions="fields.ip_group" />
          <ElAlert title="手动 deny 优先于 trusted。trusted 免检和免自动处罚；区域白名单默认 allow_only，仍需登录并经过规则。VPN/内网应填写手动CIDR。" :closable="false" class="space" />
          <ElAlert v-if="hasMixedLists" title="同时存在 deny 与 trusted 条目：若范围重叠，deny 优先。请在请求解释中验证家人的真实来源，避免误封。" type="warning" :closable="false" class="space" />
          <h3>手动名单 <ElButton size="small" @click="addIP">添加条目</ElButton></h3>
          <ElTable :data="draft.policy.ip_group.entries || []" size="small">
            <ElTableColumn label="IP / CIDR / 范围" min-width="170"><template #default="{ row }"><ElInput v-model="row.value" placeholder="192.168.1.0/24" aria-label="名单IP" /></template></ElTableColumn>
            <ElTableColumn label="类型" width="125"><template #default="{ row }"><ElSelect v-model="row.type" aria-label="名单类型"><ElOption label="白名单" value="allow" /><ElOption label="黑名单" value="deny" /></ElSelect></template></ElTableColumn>
            <ElTableColumn label="信任级别" width="160"><template #default="{ row }"><ElSelect v-model="row.trust" :disabled="row.type === 'deny'" aria-label="信任级别"><ElOption label="trusted · 可信免检" value="trusted" /><ElOption label="allow_only · 继续检测" value="allow_only" /></ElSelect></template></ElTableColumn>
            <ElTableColumn label="到期时间" width="195"><template #default="{ row }"><input type="datetime-local" :value="localDate(row.expires_at)" aria-label="名单到期时间" @change="row.expires_at = epoch(($event.target as HTMLInputElement).value)" /></template></ElTableColumn>
            <ElTableColumn label="备注" min-width="130"><template #default="{ row }"><ElInput v-model="row.remark" aria-label="名单备注" /></template></ElTableColumn>
            <ElTableColumn width="70"><template #default="{ $index }"><ElButton link type="danger" @click="draft.policy.ip_group.entries.splice($index, 1)">移除</ElButton></template></ElTableColumn>
          </ElTable>
          <h3>区域白名单</h3><p class="muted">选项来自当前本地归属地库；归属地并不精确，私网和查不到的来源不会被区域白名单放行。</p>
          <ElSelectV2 v-model="chosenRegions" :options="regionOptions.map(r => ({label:regionLabel(r),value:regionKey(r)}))" multiple filterable clearable placeholder="选择国家或省份" aria-label="区域白名单" @change="setRegions" />
          <p v-if="!regions.length" class="muted">暂无可枚举地区，请先配置本地IP库。已有选中项仍保留。</p>
        </ElTabPane>
        <ElTabPane label="3 · 规则与登录门" name="ruleset">
          <ElCollapse v-model="rulePanels">
            <ElCollapseItem title="协议 / QPS / CC / BOT（先执行）" name="controls"><Fields :model="draft.policy.ruleset" :definitions="fields.ruleset" @invalid="invalid.ruleset = $event" /></ElCollapseItem>
            <ElCollapseItem title="端口登录门与账号（攻击正文检测之前）" name="gate">
              <Fields :model="draft.policy.gate" :definitions="fields.gate" @invalid="invalid.gate = $event" />
              <label class="space">登录页Logo（PNG/JPEG，64KiB以内，宽高不超过512像素）<input type="file" accept="image/png,image/jpeg" aria-label="登录页Logo" @change="setGateLogo" /></label><img v-if="draft.policy.gate.logo_data" :src="draft.policy.gate.logo_data" alt="登录页Logo预览" style="display:block;max-width:96px;max-height:96px" /><ElButton v-if="draft.policy.gate.logo_data" link @click="draft.policy.gate.logo_data = ''">清除Logo</ElButton><p class="muted">Logo嵌入登录页，不从外部地址加载，也不展示上游信息。</p>
              <ElAlert v-if="draft.policy.gate.mode === 'basic'" title="Basic占用Authorization；若业务也使用Basic，请改用表单、trusted网段、免登录路径或独立服务令牌。" type="warning" :closable="false" class="space" />
              <h3>本端口账号 <ElButton size="small" @click="newAccount">添加账号</ElButton><ElButton size="small" @click="loadAccounts">刷新</ElButton></h3>
              <ElTable :data="accounts" size="small"><ElTableColumn prop="username" label="账号" /><ElTableColumn label="状态"><template #default="{ row }">{{ row.enabled ? '启用' : '停用' }} · {{ row.expires_at ? timeText(row.expires_at) : '不设到期' }}</template></ElTableColumn><ElTableColumn prop="remark" label="备注" /><ElTableColumn label="操作"><template #default="{ row }"><ElButton link @click="editAccount(row)">修改 / 重设口令</ElButton><ElButton link type="danger" @click="deleteAccount(row)">删除</ElButton></template></ElTableColumn></ElTable>
              <h3>活动会话</h3><ElTable :data="sessions" size="small"><ElTableColumn prop="username" label="账号" /><ElTableColumn prop="client_ip" label="来源" /><ElTableColumn label="到期"><template #default="{ row }">{{ timeText(row.expires_at) }}</template></ElTableColumn><ElTableColumn label="操作"><template #default="{ row }"><ElButton link type="danger" @click="kick(row)">立即踢出</ElButton></template></ElTableColumn></ElTable>
              <div class="space"><ElButton @click="rotateToken(false)">生成 / 轮换服务令牌</ElButton><ElButton type="danger" plain @click="rotateToken(true)">撤销服务令牌</ElButton></div>
              <ElAlert v-if="serviceToken" title="令牌只显示这一次；离开对话框即清除。旧令牌已失效。" type="warning" :closable="false" class="space" /><ElInput v-if="serviceToken" :model-value="serviceToken" readonly aria-label="一次性服务令牌" />
            </ElCollapseItem>
            <ElCollapseItem title="攻击检测（登录成功后继续执行）" name="attacks">
              <ElAlert title="先观察命中原因和正常业务，再逐条收紧。观察模式不阻断，也不会进入自动处罚。" :closable="false" class="space" />
              <Fields :model="draft.policy.ruleset" :definitions="[{path:'plugins',label:'签名插件策略',kind:'json',nullable:true,help:'mode为off/observe/block；ids为插件ID数组（最多4个）；timeout_ms为共享检测预算内的RPC时限（0使用剩余预算，最大1000毫秒）；scope可限定路径。每端口独立进程，异常只观察并放行；dry-run不运行插件。'}]" @invalid="invalid.plugins = $event" />
              <h3>加密插件启动配置</h3><p class="muted">JSON对象以插件ID为键，值包含binary（已签名的本地可执行文件路径）和params（配置对象）。只更新填写的ID，留空保留；删除策略中的ID会清除对应配置。配置加密保存，不回显、不随模板导出。插件在登录门之后执行，仅接收请求元数据；不要在插件参数中启用未经配置的联网行为。</p><ElInput v-model="pluginProfilesJSON" type="textarea" :rows="4" aria-label="加密插件配置JSON" autocomplete="off" /><ElButton v-if="draft.policy.ruleset.plugins" class="space" @click="reloadPlugins">保存时重新加载插件</ElButton>
              <Fields :model="draft.policy.ruleset" :definitions="[{path:'scripts',label:'自定义GRL规则JSON',kind:'json-array',help:'每项id、source、mode和severity。只求值条件；登录后按优先级仲裁。Ban产生异步命中事件，观察不处罚。GetIPFailureCount使用端口独立的内存窗口。'},{path:'urls',label:'URL黑白名单JSON',kind:'json-array',help:'每项id、match（exact/prefix/suffix/contains）、value、action（allow/deny）和mode。黑名单优先；白名单只跳过后续攻击检测，仍须登录。'}]" @invalid="invalid.extensions = $event" />
              <Fields :model="draft" :definitions="[{path:'script_groups',label:'自定义规则IP组快照JSON',kind:'json',help:'组名或组code映射到IP/CIDR数组；只影响本端口，与原共享IP组脱钩。'}]" @invalid="invalid.scriptGroups = $event" />
              <ElTable :data="draft.policy.ruleset.rules" size="small"><ElTableColumn label="规则" min-width="180"><template #default="{ row }"><strong>{{ ruleNames[row.category] || row.category }}</strong><small class="rule-help">{{ ruleHelp[row.category] }}</small></template></ElTableColumn><ElTableColumn label="模式" width="130"><template #default="{ row }"><ElSelect v-model="row.mode" :aria-label="`${ruleNames[row.category]}模式`"><ElOption v-for="m in modes" :key="m[0]" :label="m[1]" :value="m[0]" /></ElSelect></template></ElTableColumn><ElTableColumn label="严重度" width="130"><template #default="{ row }"><ElSelect v-model="row.severity" aria-label="规则严重度"><ElOption v-for="severity in ['low','medium','high','critical']" :key="severity" :label="severity" :value="severity" /></ElSelect></template></ElTableColumn><ElTableColumn label="范围 / 例外" width="120"><template #default="{ row }"><ElButton link @click="openRule(row)">配置范围</ElButton></template></ElTableColumn></ElTable>
              <h4>CRS 作用范围</h4><Scope :model="draft.policy.ruleset.owasp_scope" />
            </ElCollapseItem>
          </ElCollapse>
        </ElTabPane>
        <ElTabPane label="4 · 处罚与蜜罐" name="hit_policy">
          <ElAlert title="第4步是异步旁路：只改变后续请求。当前请求动作由第3步决定。trusted和受保护来源只告警，不被自动处罚。" :closable="false" class="space" />
          <Fields :model="draft.policy.hit_policy" :definitions="fields.hit_policy" />
          <h3>单IP累计阈值</h3><p class="muted">再次违规时长×4，上限7天，历史保留30天。以下时间单位均为秒。</p>
          <ElTable :data="thresholdRows" size="small"><ElTableColumn prop="label" label="类别" /><ElTableColumn label="窗口"><template #default="{ row }"><ElInputNumber v-model="draft.policy.hit_policy[row.key].window_seconds" :min="1" :max="86400" /></template></ElTableColumn><ElTableColumn label="命中次数"><template #default="{ row }"><ElInputNumber v-model="draft.policy.hit_policy[row.key].count" :min="1" :max="row.key === 'response_404' ? 128 : 100000" /></template></ElTableColumn><ElTableColumn label="首次封禁时长"><template #default="{ row }"><ElInputNumber v-if="row.ban" v-model="draft.policy.hit_policy[row.key].ban_seconds" :min="1" :max="604800" /><span v-else>导入蜜罐，使用下方TTL</span></template></ElTableColumn></ElTable>
          <h3>内置蜜罐</h3><Fields :model="draft.policy.honeypot" :definitions="fields.honeypot" />
          <h3>可选业务登录观察（JSON）</h3><p class="muted">默认空列表。每条配置 path、method、username_field、password_field、success_statuses、可选success_location_prefix/success_body_contains、body_prefix_bytes（1–8192）。响应正文只采样，不缓存整包。</p><ElInput v-model="businessLoginJSON" type="textarea" :rows="6" aria-label="业务登录观察JSON" @change="parseBusinessLogin" /><p v-if="invalid.businessLogin" role="alert" class="error">业务登录观察必须是合法JSON数组。</p>
        </ElTabPane>
        <ElTabPane label="5 · 业务代理" name="network">
          <ElAlert title="WebSocket、SSE、Range和大文件保持流式。上游TLS默认校验；私有CA可在下方显式配置。" :closable="false" class="space" />
          <Fields :model="draft.policy.network" :definitions="fields.network" @invalid="invalid.network = $event" />
          <h3>网页防篡改基线</h3><p class="muted">响应策略的tamper数组配置id、path、hosts、mode、action（replace/alert）、ignore_query和max_bytes。每端口基线总额1MiB；只从完整200响应学习，基线加密且不随模板导出。重新学习将在保存后使用下一次合格响应，请先确认上游页面正确。</p>
          <ElTable :data="draft.policy.network.response?.tamper || []" size="small"><ElTableColumn prop="id" label="规则" /><ElTableColumn prop="path" label="精确路径" /><ElTableColumn label="基线更新时间"><template #default="{row}">{{ row.learned_at ? timeText(row.learned_at) : '待学习' }}</template></ElTableColumn><ElTableColumn label="操作"><template #default="{row}"><ElButton link @click="relearnTamper(row)">重新学习</ElButton></template></ElTableColumn></ElTable>
          <Fields :model="draft" :definitions="[{path:'upstream_dial_ip',label:'默认上游固定连接IP',help:'可选。仅改变连接目标，Host和TLS SNI仍使用上游URL主机名。'},{path:'routes',label:'同端口业务路由JSON',kind:'json-array',help:'每项包含hosts、可选path_prefix、upstream、可选dial_ip。所有路由共用本端口安全策略、账号和计数器；匹配顺序为主机名精度与最长路径前缀。'}]" @invalid="invalid.routes = $event" />
          <h3>证书与信任</h3><p class="muted">迁入证书继续跟随源站点证书夹续期；新握手热更新，不中断现有连接。手动提交新证书会解除源证书绑定；留空保留。HTTP-01仅响应迁入主机已登记的本地挑战，访问总开关仍有效。</p><div class="two-columns"><label>TLS证书PEM（留空保留）<ElInput v-model="secrets.tls_certificate" type="textarea" :rows="4" /></label><label>TLS私钥PEM（留空保留）<ElInput v-model="secrets.tls_private_key" type="textarea" :rows="4" /></label><label>上游私有CA PEM（留空保留）<ElInput v-model="secrets.upstream_ca" type="textarea" :rows="4" /></label><label><ElCheckbox v-model="secrets.clear_upstream_ca">清除已保存的上游私有CA</ElCheckbox><small>清除后只使用系统信任库。</small></label></div>
          <ElButton class="space" @click="testUpstream">测试上游TCP连通性</ElButton>
          <h3>加密业务头</h3><p class="muted">默认配置引用：{{ draft.header_profile || '无' }}。路由可通过 header_profile 引用独立配置。头值可能含有上游令牌，保存后不回显、不随模板导出；留空保留。修改示例：{"配置名称":{"request":[{"header_name":"Authorization","header_value":"Bearer 新令牌"}],"response":[]}}。只更新填写的配置名称。</p><ElInput v-model="businessHeadersJSON" type="textarea" :rows="4" aria-label="加密业务头JSON" autocomplete="off" />
          <h3>响应隐私脱敏</h3><p class="muted">在响应策略的privacy对象设置mode及urls（match: exact/prefix/suffix/contains，value: URL）。复用本地DLP引擎；观察不修改正文，不输出原始识别结果。高级YAML配置加密保存、不回显、不随模板导出，留空保留；没有自定义配置时使用内置规则。</p><ElInput v-model="secrets.privacy_engine_config" type="textarea" :rows="4" aria-label="隐私引擎配置" autocomplete="off" /><ElCheckbox v-model="secrets.clear_privacy_engine_config">清除自定义隐私配置并使用内置规则</ElCheckbox>
        </ElTabPane>
        <ElTabPane label="解释 / 模板 / 高级" name="explain">
          <h3>请求解释（不产生处罚）</h3><div class="two-columns"><label>来源IP<ElInput v-model="sample.client_ip" aria-label="解释来源IP" /></label><label>方法<ElInput v-model="sample.method" /></label><label>路径<ElInput v-model="sample.path" aria-label="解释路径" /></label><label>请求头JSON<ElInput v-model="sampleHeaders" type="textarea" :rows="3" /></label><label>样例正文<ElInput v-model="sample.body" type="textarea" :rows="4" /></label></div><ElButton class="space" :loading="busy" @click="explain">解释当前草稿</ElButton><Trace :result="preview" />
          <h3>应用模板 / 从模板更新</h3><p class="muted">来源：{{ draft.template?.id || '独立配置' }} · 版本 {{ draft.template?.version || '—' }}。先查看差异，再只覆盖选中分区；名单条目、可信代理、凭据会保留。</p>
          <ElSelect v-model="selectedTemplate" aria-label="待应用模板"><ElOption v-for="tpl in templates" :key="tpl.id" :label="`${tpl.name} · v${tpl.version}`" :value="tpl.id" /></ElSelect><ElCheckboxGroup v-model="selectedSections" class="space"><ElCheckbox v-for="section in sections" :key="section" :value="section">{{ sectionNames[section] }}</ElCheckbox></ElCheckboxGroup><ElButton @click="previewTemplate">查看模板差异</ElButton>
          <div v-if="templatePreview" class="space"><ElAlert :title="`有${templatePreview.diff?.length || 0}个分区与模板不同`" :closable="false" /><details v-for="diff in templatePreview.diff || []" :key="diff.section"><summary>{{ sectionNames[diff.section] }}</summary><div class="two-columns"><pre>{{ JSON.stringify(diff.current,null,2) }}</pre><pre>{{ JSON.stringify(diff.template,null,2) }}</pre></div></details><ElButton type="primary" @click="acceptTemplate">确认应用选中分区到草稿</ElButton></div>
          <h3>观察 → 拦截</h3><p class="muted">先从处罚列表和trace确认不是正常业务，再选择具体规则。此向导不自动打开自动封禁或登录门。</p><ElSelect v-model="tightenRules" multiple placeholder="选择已验证的观察规则" aria-label="渐进收紧规则"><ElOption v-for="rule in draft.policy.ruleset.rules.filter(r => r.mode === 'observe')" :key="rule.id" :label="ruleNames[rule.category] || rule.id" :value="rule.id" /></ElSelect><ElButton class="space" @click="tighten">将所选规则设为拦截</ElButton>
          <details class="space"><summary>高级：编辑完整策略JSON（不含口令/私钥）</summary><p class="muted">可设置所有范围、参数和观察条件。服务端严格验证字段与数值；这里不会修改监听端口或凭据。</p><ElButton size="small" @click="advancedJSON = JSON.stringify(draft.policy, null, 2)">读取当前策略</ElButton><ElInput v-model="advancedJSON" type="textarea" :rows="12" aria-label="完整策略JSON" /><ElButton class="space" @click="applyAdvanced">校验JSON并写入草稿</ElButton></details>
        </ElTabPane>
      </ElTabs>
      <div class="editor-actions"><span class="muted">只保存当前端口。账号操作单独即时生效。</span><ElButton @click="emit('close')">关闭</ElButton><ElButton type="primary" :loading="busy" :disabled="Object.values(invalid).some(Boolean)" @click="save">保存端口配置</ElButton></div>
    </template>
    <ElDialog v-model="accountVisible" title="端口账号" width="min(620px,94vw)" append-to-body destroy-on-close>
      <ElButton :disabled="busy" @click="account.totp_secret = newTOTPSecret(); account.clear_totp = false">生成新的动态口令密钥</ElButton>
      <div class="account-form"><label>用户名<ElInput v-model="account.username" :disabled="editingAccount" aria-label="端口账号用户名" /></label><label>{{ editingAccount ? '新口令（留空保持）' : '口令' }}<ElInput v-model="account.password" type="password" show-password autocomplete="new-password" aria-label="端口账号口令" /></label><label>动态口令Base32密钥（可选，留空保留）<ElInput v-model="account.totp_secret" type="password" show-password autocomplete="new-password" aria-label="动态口令密钥" /></label><small>仅表单模式支持；密钥加密保存且不回显。设置后使用身份验证器的六位动态码登录。{{ account.totp_configured ? '当前已启用二次验证。' : '当前未设置动态口令。' }}</small><ElCheckbox v-if="account.totp_configured" v-model="account.clear_totp">明确移除该账号的二次验证</ElCheckbox><ElSwitch v-model="account.enabled" active-text="账号启用" /><label>到期时间（留空为不限）<input type="datetime-local" :value="localDate(account.expires_at)" @change="account.expires_at = epoch(($event.target as HTMLInputElement).value)" /></label><label>备注<ElInput v-model="account.remark" /></label><p class="muted">保存账号会撤销旧会话；不能删除或停用已开启登录门的最后一个有效账号。</p></div>
      <template #footer><ElButton @click="accountVisible = false">取消</ElButton><ElButton type="primary" :loading="busy" @click="saveAccount">立即保存账号</ElButton></template>
    </ElDialog>
    <ElDialog v-model="ruleVisible" :title="`规则范围：${activeRule?.id || ''}`" width="min(850px,94vw)" append-to-body>
      <template v-if="activeRule"><Scope :model="activeRule.scope" /><Fields v-if="activeRule.category === 'scanner'" :model="activeRule.params" :definitions="[{path:'probe_paths',label:'本站不存在的探测路径',kind:'lines',help:'每行一个路径；存在的管理页、WordPress登录页不要填入。'}]" /></template><template #footer><ElButton type="primary" @click="ruleVisible = false">返回配置</ElButton></template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import type { PropType } from 'vue';
import { ElAlert, ElButton, ElCheckbox, ElCheckboxGroup, ElCollapse, ElCollapseItem, ElDialog, ElInput, ElInputNumber, ElMessage, ElMessageBox, ElOption, ElSelect, ElSelectV2, ElStep, ElSteps, ElSwitch, ElTable, ElTableColumn, ElTabPane, ElTabs, ElTag } from 'element-plus';
import { pipelineGet, pipelinePost } from '@/apis/pipeline';
import { clone, fields, modes, parseLines, ruleHelp, ruleNames, sectionNames, sections, type Field, type PolicyTemplate, type PortConfig } from './model';
import Fields from './Fields.vue'; import Scope from './Scope.vue'; import Trace from './Trace.vue';
import { newTOTPSecret } from './model';
const props = defineProps({config:{type:Object as PropType<PortConfig & {access_changed_at?:number;access_changed_by?:string}>,required:true},templates:{type:Array as PropType<PolicyTemplate[]>,required:true},isNew:Boolean});
const emit = defineEmits(['saved','close']);
const draft = ref<PortConfig>(clone(props.config)); const busy=ref(false), error=ref(''), activeTab=ref('access'), wizardStep=ref(0);
const secrets=reactive({initial_username:'',initial_password:'',tls_certificate:'',tls_private_key:'',upstream_ca:'',clear_upstream_ca:false,privacy_engine_config:'',clear_privacy_engine_config:false});
const businessHeadersJSON=ref('');
const pluginProfilesJSON=ref('');
function reloadPlugins(){const plugins=draft.value.policy.ruleset.plugins;if(plugins){plugins.generation=(plugins.generation||0)+1;ElMessage.info('已标记重新加载，请保存端口配置')}}
async function setGateLogo(event: Event) { const input=event.target as HTMLInputElement; const file=input.files?.[0]; if(!file)return; await run(async()=>{if(file.size>65536||!['image/png','image/jpeg'].includes(file.type))throw new Error('请选择64KiB以内的PNG/JPEG');const data=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=()=>reject(new Error('Logo读取失败'));reader.readAsDataURL(file)});draft.value.policy.gate.logo_data=data;input.value=''}); }
const selectedTemplate=ref(draft.value.template?.id || ''), selectedSections=ref([...sections]), templatePreview=ref<any>(null), preview=ref<any>(null), previewKey=ref('');
const invalid=reactive<Record<string,boolean>>({}), rulePanels=ref(['controls','gate','attacks']), activeRule=ref<any>(null),ruleVisible=ref(false);
const sample=reactive({client_ip:'198.51.100.25',method:'GET',path:'/',body:''}),sampleHeaders=ref('{"Accept":"text/html"}');
const accounts=ref<any[]>([]),sessions=ref<any[]>([]),regions=ref<any[]>([]),chosenRegions=ref<string[]>([]),serviceToken=ref('');
const accountVisible=ref(false),editingAccount=ref(false),account=reactive({username:'',password:'',totp_secret:'',clear_totp:false,totp_configured:false,enabled:true,expires_at:0,remark:''});
const tightenRules=ref<string[]>([]),advancedJSON=ref(''),businessLoginJSON=ref('[]');
const templatePreviewKey=ref('');
const thresholdRows=[{key:'critical',label:'critical',ban:true},{key:'high',label:'high',ban:true},{key:'medium',label:'medium',ban:true},{key:'login_ban',label:'持续登录失败',ban:true},{key:'response_404',label:'不同路径404',ban:false},{key:'response_auth',label:'401/403异常',ban:false},{key:'response_5xx',label:'可归因5xx',ban:false}];
const basicFields:Field[]=[{path:'port',label:'监听端口',kind:'number',min:1,max:65535,help:'每个端口独立配置，不能占用管理端口或已有站点端口。'},{path:'name',label:'端口名称',help:'例如家庭相册、NAS文件。'},{path:'protocol',label:'监听协议',kind:'select',options:[['http','HTTP'],['https','HTTPS']],help:'HTTPS需要证书与私钥。'},{path:'upstream',label:'上游地址',help:'明确填写http(s)://主机:端口，不能在URL中夹带账号密码。'},{path:'host_names',label:'允许的主机名',kind:'lines',help:'每行一个，不含端口；留空接受所有Host。同一端口的所有域名共用安全策略。'}];
function normalize(){const p=draft.value.policy;p.ip_group.entries ||= [];p.ip_group.regions ||= [];p.ruleset.owasp_scope ||= {};for(const r of p.ruleset.rules||[]){r.scope ||= {};r.params ||= {}};const defaults={login_ban:[600,20,1800],response_404:[60,20,0],response_auth:[300,10,0],response_5xx:[300,5,0]};for(const [key,v] of Object.entries(defaults)){if(!p.hit_policy[key]?.count)p.hit_policy[key]={window_seconds:v[0],count:v[1],ban_seconds:v[2]}};businessLoginJSON.value=JSON.stringify(p.hit_policy.business_login||[],null,2);chosenRegions.value=p.ip_group.regions.map(regionKey)}
normalize();
function regionKey(r:any){return JSON.stringify([r.country,r.province||''])}function regionLabel(r:any){return r.country+(r.province?' / '+r.province:' / 全部省份')}
const regionOptions=computed(()=>{const map=new Map<string,any>();for(const r of [...regions.value,...draft.value.policy.ip_group.regions]){map.set(regionKey(r),r);map.set(regionKey({country:r.country}),{country:r.country})}return [...map.values()]});
function setRegions(values:string[]){draft.value.policy.ip_group.regions=values.map(v=>{const [country,province]=JSON.parse(v);return {country,province}})}
const hasMixedLists=computed(()=>draft.value.policy.ip_group.entries.some(e=>e.type==='deny')&&draft.value.policy.ip_group.entries.some(e=>e.type==='allow'&&e.trust==='trusted'));
const timeText=(value:number)=>value?new Date(value*1000).toLocaleString('zh-CN'):'未记录';
function localDate(value:number){if(!value)return '';const d=new Date(value*1000);return new Date(d.getTime()-d.getTimezoneOffset()*60000).toISOString().slice(0,16)}
const epoch=(value:string)=>value?Math.floor(new Date(value).getTime()/1000):0;
async function run(work:()=>Promise<void>){if(busy.value)return;busy.value=true;error.value='';try{await work()}catch(e){if(e!=='cancel'&&e!=='close')error.value=e.message||String(e)}finally{busy.value=false}}
function addIP(){draft.value.policy.ip_group.entries.push({id:crypto.randomUUID(),type:'allow',value:'',trust:'trusted',expires_at:0,remark:''})}
function openRule(rule:any){activeRule.value=rule;ruleVisible.value=true}
async function testUpstream(){await run(async()=>{const message=await pipelinePost('upstream/test',{upstream:draft.value.upstream});ElMessage.success(message+'（TCP）')})}
async function relearnTamper(rule:any){await run(async()=>{await ElMessageBox.confirm('保存后将重新学习下一次合格响应。请先确认上游未被篡改。','重新学习防篡改基线',{type:'warning'});rule.generation=(rule.generation||0)+1;rule.learned_at=0;ElMessage.info('已标记重新学习，请保存端口配置')})}
async function explainNow(){const headers=JSON.parse(sampleHeaders.value);preview.value=await pipelinePost('explain',{config:draft.value,...sample,headers});previewKey.value=JSON.stringify(draft.value)}
async function explain(){await run(explainNow)}
async function nextWizard(){await run(async()=>{if(wizardStep.value===0){await pipelinePost('ports/check',draft.value);await pipelinePost('upstream/test',{upstream:draft.value.upstream});if(draft.value.protocol==='https'&&(!secrets.tls_certificate||!secrets.tls_private_key))throw new Error('HTTPS需要证书和私钥')}if(wizardStep.value===2&&draft.value.policy.gate.enabled){if(!secrets.initial_username.trim()||secrets.initial_password.length<10)throw new Error('请设置账号和至少10字符的口令')}wizardStep.value++;if(wizardStep.value===3)await explainNow()})}
async function applyWizardTemplate(){if(!selectedTemplate.value)return;await run(async()=>{const result=await pipelinePost('templates/preview',{config:draft.value,template_id:selectedTemplate.value,sections:[]});draft.value=result.config;normalize()})}
async function previewTemplate(){await run(async()=>{if(!selectedTemplate.value||!selectedSections.value.length)throw new Error('请选择模板及至少一个分区');templatePreview.value=await pipelinePost('templates/preview',{config:draft.value,template_id:selectedTemplate.value,sections:selectedSections.value});templatePreviewKey.value=JSON.stringify(draft.value)})}
async function acceptTemplate(){await run(async()=>{if(templatePreviewKey.value!==JSON.stringify(draft.value))throw new Error('草稿已变化，请重新预览差异');await ElMessageBox.confirm('将选中分区复制到当前草稿，点击保存后才生效。','确认应用模板',{type:'warning'});draft.value=templatePreview.value.config;normalize();templatePreview.value=null;ElMessage.success('已应用到草稿')})}
async function tighten(){await run(async()=>{if(!tightenRules.value.length)throw new Error('请选择已验证的规则');await ElMessageBox.confirm('确认这些命中不是正常业务？所选规则将对匹配请求执行拦截，保存后生效。','渐进收紧',{type:'warning'});for(const r of draft.value.policy.ruleset.rules)if(tightenRules.value.includes(r.id))r.mode='block';tightenRules.value=[]})}
function parseBusinessLogin(){try{const list=JSON.parse(businessLoginJSON.value);if(!Array.isArray(list))throw new Error();draft.value.policy.hit_policy.business_login=list;invalid.businessLogin=false}catch{invalid.businessLogin=true}}
async function applyAdvanced(){await run(async()=>{const policy=JSON.parse(advancedJSON.value);if(!policy||sections.some(s=>!policy[s]))throw new Error('策略必须包含全部七个分区');const candidate={...draft.value,policy};await pipelinePost('explain',{config:candidate,...sample,headers:JSON.parse(sampleHeaders.value)});draft.value=candidate;normalize();ElMessage.success('服务端校验通过，已写入草稿')})}
async function save(){await run(async()=>{parseBusinessLogin();if(Object.values(invalid).some(Boolean))throw new Error('请先修正JSON字段');let plugin_profiles;if(pluginProfilesJSON.value.trim()){plugin_profiles=JSON.parse(pluginProfilesJSON.value);if(!plugin_profiles||typeof plugin_profiles!=='object'||Array.isArray(plugin_profiles))throw new Error('插件启动配置必须是JSON对象')}let business_headers;if(businessHeadersJSON.value.trim()){business_headers=JSON.parse(businessHeadersJSON.value);if(!business_headers||typeof business_headers!=='object'||Array.isArray(business_headers))throw new Error('加密业务头必须是JSON对象')}if(!props.isNew&&draft.value.policy.access.enabled!==props.config.policy.access.enabled)await ElMessageBox.confirm(draft.value.policy.access.enabled?'允许这个端口重新接收业务访问？':'关闭后包括trusted在内的业务请求均被拒绝，已有连接按排空时间关闭。管理端不受影响。','确认切换访问开关',{type:'warning'});if(props.isNew)draft.value.listen_enabled=false;const result=await pipelinePost<PortConfig>('ports/save',{config:draft.value,...secrets,business_headers,plugin_profiles});Object.assign(secrets,{initial_username:'',initial_password:'',tls_certificate:'',tls_private_key:'',upstream_ca:'',clear_upstream_ca:false,privacy_engine_config:'',clear_privacy_engine_config:false});businessHeadersJSON.value='';pluginProfilesJSON.value='';serviceToken.value='';ElMessage.success(props.isNew?'端口已创建，尚未监听':'端口配置已保存');emit('saved',result)})}
async function loadAccounts(){await run(async()=>{[accounts.value,sessions.value]=await Promise.all([pipelineGet('gate/users',{port:draft.value.port}),pipelineGet('gate/sessions',{port:draft.value.port})])})}
function newAccount(){editingAccount.value=false;Object.assign(account,{username:'',password:'',totp_secret:'',clear_totp:false,totp_configured:false,enabled:true,expires_at:0,remark:''});accountVisible.value=true}
function editAccount(row:any){editingAccount.value=true;Object.assign(account,{username:row.username,password:'',totp_secret:'',clear_totp:false,totp_configured:!!row.totp_configured,enabled:row.enabled,expires_at:row.expires_at,remark:row.remark});accountVisible.value=true}
async function saveAccount(){await run(async()=>{if(account.clear_totp)await ElMessageBox.confirm('移除后该账号只需密码即可登录；旧会话将撤销。','确认移除二次验证',{type:'warning'});const {totp_configured,...values}=account;await pipelinePost('gate/users/save',{port:draft.value.port,...values});account.password='';account.totp_secret='';accountVisible.value=false;accounts.value=await pipelineGet('gate/users',{port:draft.value.port});sessions.value=await pipelineGet('gate/sessions',{port:draft.value.port});ElMessage.success('账号已保存，旧会话已撤销')})}
async function deleteAccount(row:any){await run(async()=>{await ElMessageBox.confirm(`删除${row.username}并撤销其会话？`,'删除账号',{type:'warning'});await pipelinePost('gate/users/delete',{port:draft.value.port,username:row.username});accounts.value=await pipelineGet('gate/users',{port:draft.value.port});sessions.value=await pipelineGet('gate/sessions',{port:draft.value.port})})}
async function kick(row:any){await run(async()=>{await ElMessageBox.confirm('立即撤销该会话？','踢出会话',{type:'warning'});await pipelinePost('gate/sessions/kick',{port:draft.value.port,id:row.id});sessions.value=await pipelineGet('gate/sessions',{port:draft.value.port})})}
async function rotateToken(revoke:boolean){await run(async()=>{await ElMessageBox.confirm(revoke?'撤销后，使用旧服务令牌的客户端将无法登录。':'轮换后旧令牌立即失效，新令牌只显示一次。','服务令牌',{type:'warning'});const result=await pipelinePost('gate/token/rotate',{port:draft.value.port,revoke,revision:draft.value.revision});draft.value.revision=result.revision;serviceToken.value=result.token||'';ElMessage.success(revoke?'服务令牌已撤销':'服务令牌已轮换')})}
watch(accountVisible,value=>{if(!value){account.password='';account.totp_secret=''}});
watch([selectedTemplate,()=>selectedSections.value.join(',')],()=>{templatePreview.value=null});
onMounted(async()=>{try{regions.value=await pipelineGet('regions')}catch{}if(!props.isNew)await loadAccounts()});
</script>

<style scoped>
.port-editor{color:var(--el-text-color-primary)}.space{margin-top:18px;margin-bottom:18px}.two-columns{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}.two-columns label,.account-form label{display:flex;flex-direction:column;gap:8px;min-width:0}.muted,.rule-help,small{color:var(--el-text-color-secondary);line-height:1.65;font-size:12px}.rule-help{display:block;margin-top:5px}.policy-summary,.editor-top{display:flex;gap:18px;flex-wrap:wrap;margin:18px 0}.editor-actions{display:flex;align-items:center;justify-content:flex-end;gap:12px;padding:18px 0 0;margin-top:24px;border-top:1px solid var(--el-border-color-lighter)}.editor-actions .muted{margin-right:auto}.account-form{display:grid;gap:16px}h3{margin-top:28px;font-size:17px}input[type=datetime-local]{padding:7px;border:1px solid var(--el-border-color);border-radius:4px;background:var(--el-fill-color-blank);color:var(--el-text-color-primary);max-width:100%}pre{padding:12px;background:var(--el-fill-color-light);overflow:auto;max-height:320px;font-size:12px}details{margin:12px 0}summary{cursor:pointer;font-weight:600;padding:10px 0}.error{color:var(--el-color-danger)}@media(max-width:760px){.two-columns{grid-template-columns:1fr}.editor-actions{flex-wrap:wrap}.editor-top{gap:8px}.port-editor :deep(.el-steps){font-size:12px}}
</style>
