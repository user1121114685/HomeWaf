export function accessSourceSummary(policy: Record<string, any>): string {
  const group = policy.ip_group || {};
  const allow: string[] = [];
  if (group.unlisted_action === 'deny') {
    if (group.regions?.length)
      allow.push(
        group.regions.map((region: any) => [region.country, region.province].filter(Boolean).join(' / ')).join('、'),
      );
    if (group.allow_groups?.length) allow.push(`${group.allow_groups.length} 个白名单 IP 组`);
    if (group.entries?.some((entry: any) => entry.type === 'allow')) allow.push('已有地址白名单');
  }
  const parts = allow.length
    ? [`仅允许：${allow.join(' 或 ')}`]
    : [
        group.unlisted_action === 'deny'
          ? '仅允许白名单来源'
          : group.unlisted_action === 'honeypot'
            ? '未命中来源进入蜜罐'
            : '来源默认继续检测',
      ];
  if (group.block_outside_mainland) parts.push('拒绝中国大陆以外及未知公网来源');
  if (group.denied_regions?.length)
    parts.push(
      `屏蔽地区：${group.denied_regions.map((region: any) => [region.country, region.province].filter(Boolean).join(' / ')).join('、')}`,
    );
  if (group.deny_groups?.length) parts.push(`${group.deny_groups.length} 个黑名单 IP 组`);
  if (group.entries?.some((entry: any) => entry.type === 'deny')) parts.push('已有地址黑名单');
  return parts.join('；');
}

const sourceReasons: Record<string, string> = {
  'blacklist:custom': '命中手动 IP 黑名单',
  'blacklist:group': '命中黑名单 IP 组',
  'blacklist:region': '命中禁止访问的地区',
  'blacklist:outside_mainland': '来源不属于已识别的中国大陆公网地址',
  'blacklist:vip': '命中威胁订阅名单',
  'whitelist:group': '命中白名单 IP 组，继续后续防护',
  'whitelist:region': '命中允许地区，继续后续防护',
  'whitelist:allow_only': '命中地址白名单，继续后续防护',
  'whitelist:trusted': '命中高级 trusted 免检规则',
  'ip:group_unavailable': '引用的 IP 组不可用，暂时拒绝访问',
  'ip:unlisted': '未命中来源白名单，按未命中策略处理',
};
export function accessReason(reason: string): string {
  return sourceReasons[reason] ? `${sourceReasons[reason]}（${reason}）` : reason;
}
