// Service protection navigation always edits the configuration used by the listener.
export const servicePolicyEntries: Record<string, { title: string; tab: string; panels?: string[]; hint: string }> = {
  WafAntiCCList: {
    title: 'CC 防御',
    tab: 'cc',
    hint: '按服务设置异常访问频率。仅观察会记录命中；拦截模式会执行验证或拒绝访问。',
  },
  WafIpWhiteList: {
    title: 'IP 白名单',
    tab: 'ip_group',
    hint: '为选中的服务维护来源名单。白名单仍可继续接受登录和防护检查。',
  },
  WafIpBlockList: { title: 'IP 黑名单', tab: 'ip_group', hint: '为选中的服务阻止指定 IP、网段或地址范围。' },
  WafUrlWhiteList: {
    title: 'URL 白名单',
    tab: 'ruleset',
    panels: ['attacks'],
    hint: '维护该服务的路径例外。白名单不绕过访问限制和登录要求。',
  },
  WafUrlBlockList: {
    title: 'URL 黑名单',
    tab: 'ruleset',
    panels: ['attacks'],
    hint: '维护该服务的禁止访问路径，保存后更新实际防护策略。',
  },
  WafRule: {
    title: '防御规则',
    tab: 'ruleset',
    panels: ['attacks'],
    hint: '调整当前服务的攻击检测规则、模式与适用范围。',
  },
  Wafruleedit: {
    title: '自定义规则',
    tab: 'ruleset',
    panels: ['attacks'],
    hint: '自定义规则与当前服务的防护策略一起保存和生效。',
  },
  WafSensitiveList: {
    title: '敏感词检测',
    tab: 'ruleset',
    panels: ['controls'],
    hint: '敏感词检测按服务配置；规则模式决定只记录还是拦截。',
  },
  WafLdpUrlList: { title: '隐私保护路径', tab: 'network', hint: '在当前服务的响应策略中设置脱敏路径和规则。' },
  PrivateInfo: {
    title: '响应隐私保护',
    tab: 'network',
    hint: '响应脱敏配置加密保存在当前服务中，空凭据不会覆盖已有配置。',
  },
  WafPathRuleList: { title: '路径与转发', tab: 'network', hint: '按主机名和路径选择当前服务的业务目标。' },
  WafAccessConfig: {
    title: '访问认证',
    tab: 'ruleset',
    panels: ['gate'],
    hint: '配置当前服务的登录要求、服务令牌和访问账号。',
  },
  WafAccessAccount: {
    title: '访问账号',
    tab: 'ruleset',
    panels: ['gate'],
    hint: '账号属于选中的服务；修改不会影响其他服务。',
  },
  WafAccessSession: { title: '在线会话', tab: 'ruleset', panels: ['gate'], hint: '查看和管理选中服务的已登录会话。' },
};
