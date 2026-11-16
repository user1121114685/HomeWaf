/** Product navigation is independent of backend module names. Route permissions
 * remain the source of truth: this metadata never grants access to a route. */
export const workspaceGroups = [
  {
    id: 'services',
    label: ['服务与连接', 'Services'],
    names: [
      'WafPipeline',
      'WafPipelineTemplates',
      'AppList',
      'Tunnel',
      'OneKeyMod',
      'LegacyWafHosts',
      'WebsiteDiagnostic',
    ],
  },
  {
    id: 'certificates',
    label: ['证书与 HTTPS', 'Certificates'],
    names: ['WafSSLConfigList', 'WafSSLOrderList', 'WafsslExpireList'],
  },
  {
    id: 'access',
    label: ['访问控制', 'Access control'],
    names: [
      'WafAccessConfig',
      'WafAccessAccount',
      'WafAccessSession',
      'WafIpWhiteList',
      'WafIpBlockList',
      'WafIpGroup',
      'WafUrlWhiteList',
      'WafUrlBlockList',
    ],
  },
  {
    id: 'protection',
    label: ['防护策略', 'Protection'],
    names: [
      'WafPipelineHoneypot',
      'WafRule',
      'Wafruleedit',
      'WafAntiCCList',
      'WafSensitiveList',
      'WafLdpUrlList',
      'PrivateInfo',
      'WafPathRuleList',
      'WafBlockingPagelist',
      'WafIPBan',
      'WafFirewallIPBlock',
      'WafThreatIP',
      'OwaspManage',
      'AIModelManage',
    ],
  },
  {
    id: 'traffic',
    label: ['流量与记录', 'Traffic & records'],
    names: [
      'WafvisitLog',
      'WafAttackLog',
      'WafAttackLogDetail',
      'DashboardBase',
      'DashboardOverview',
      'DashboardStats',
      'DashboardScreen',
      'DashboardShared',
      'WafAnalysisLog',
      'WafAnalysisSpiderLog',
      'WafAnalysisSourcePath',
      'WafAccessAudit',
      'LogFileWrite',
    ],
  },
  {
    id: 'notifications',
    label: ['消息通知', 'Notifications'],
    names: ['NotifyChannel', 'NotifySubscription', 'NotifyLog'],
  },
  {
    id: 'accounts',
    label: ['账号与 API', 'Accounts & API'],
    names: [
      'Account',
      'AccountLog',
      'LoginHistory',
      'OTP',
      'UserIndex',
      'OPlatformKey',
      'OPlatformLog',
      'OPlatformDoc',
    ],
  },
  {
    id: 'system',
    label: ['实例与运维', 'Instance & operations'],
    names: [
      'VpConfig',
      'HostGuard',
      'WafCDNIP',
      'IPLocation',
      'SystemConfig',
      'Task',
      'WafBatchTaskList',
      'DataRetention',
      'FileManage',
      'RunDiagnostic',
      'RumtimeSysteminfo',
      'SysLog',
      'SqlQuery',
      'UpgradeNotice',
      'License',
    ],
  },
] as const;

export function workspaceFor(name: string) {
  if (name === 'WafPipelineDiagnostics') name = 'WafPipeline';
  if (['CertificateRequest', 'CertificateSettings', 'CertificateDetail'].includes(name)) name = 'WafSSLConfigList';
  return workspaceGroups.find((group) => (group.names as readonly string[]).includes(name));
}

export const taskFamilies = [
  ['WafPipeline', 'WafPipelineTemplates'],
  ['WafSSLConfigList', 'WafSSLOrderList', 'WafsslExpireList'],
  ['NotifyChannel', 'NotifySubscription', 'NotifyLog'],
  ['WafAccessConfig', 'WafAccessAccount', 'WafAccessSession'],
  ['OPlatformKey', 'OPlatformLog', 'OPlatformDoc'],
  ['DashboardBase', 'DashboardOverview', 'DashboardStats'],
  ['WafAnalysisLog', 'WafAnalysisSpiderLog', 'WafAnalysisSourcePath'],
  ['Account', 'LoginHistory', 'AccountLog', 'OTP'],
  ['RunDiagnostic', 'RumtimeSysteminfo', 'SysLog'],
] as const;

export const taskTitles: Record<string, [string, string]> = {
  OPlatformDoc: ['接口文档', 'API reference'],
  WafPipeline: ['我的服务', 'My services'],
  WafPipelineTemplates: ['策略模板', 'Policy templates'],
  WafPipelineHoneypot: ['统一蜜罐', 'Shared honeypot'],
  WafSSLOrderList: ['申请与续期记录', 'Certificate jobs'],
  WafSSLConfigList: ['证书管理', 'Certificates'],
  WafsslExpireList: ['外部证书监测', 'Certificate monitoring'],
  WafAccessConfig: ['认证设置', 'Authentication'],
  WafAccessAccount: ['访问账号', 'Visitor accounts'],
  WafAccessSession: ['在线会话', 'Sessions'],
  NotifyChannel: ['通知渠道', 'Channels'],
  NotifySubscription: ['事件订阅', 'Subscriptions'],
  NotifyLog: ['发送记录', 'Delivery records'],
  WafAntiCCList: ['CC 防护', 'CC protection'],
  WafIpGroup: ['共享 IP 组', 'Shared IP groups'],
  PrivateInfo: ['响应脱敏', 'Response privacy'],
};
