type DiagnosticHit = {
  category?: string;
  rule_id?: string;
  severity?: string;
  observe?: boolean;
  trusted?: boolean;
  source?: string;
  code?: string;
};

export function requestDiagnosticView(detail: Record<string, any>, locale = 'zh_CN') {
  const tr = (zh: string, en: string) => (locale.startsWith('zh') ? zh : en);
  const data = detail.diagnostics;
  const state = data?.state || 'unsupported';
  const missing = tr('未记录', 'Not recorded');
  const sources: Record<string, string> = {
    header: tr('请求头', 'Request header'),
    cookie: 'Cookie',
    query: tr('URL 查询参数', 'URL query parameter'),
    form: tr('表单参数', 'Form parameter'),
    json: tr('JSON 值', 'JSON value'),
    json_key: tr('JSON 键', 'JSON key'),
    multipart: tr('上传表单字段', 'Multipart field'),
    body: tr('请求正文', 'Request body'),
    path: tr('请求路径', 'Request path'),
    request_headers: tr('HTTP 请求头或协议', 'HTTP headers or protocol'),
  };
  const reasons: Record<string, string> = {
    'ssrf.internal_url': tr(
      '检测到内网、回环、链路本地等地址或受限协议的 URL。此项只说明 URL 特征命中，不代表后端已经发起请求。',
      'Detected a URL with an internal, loopback, link-local or similar address, or a restricted protocol. This does not establish that the backend fetched it.',
    ),
    'csrf.missing_origin_observe': tr(
      '请求缺少 Origin / Referer 来源头，规则仅观察。原生 App 也可能不发送来源头，不能仅凭此项确认跨站攻击。',
      'Origin / Referer is missing and the rule only observes. Native apps may omit these headers; this alone does not establish a cross-site attack.',
    ),
    'csrf.missing_origin': tr(
      '请求缺少 Origin / Referer 来源头，触发了已配置的缺失来源策略。',
      'Origin / Referer is missing, triggering the configured missing-origin policy.',
    ),
    'csrf.origin_mismatch': tr(
      '请求来源与服务或允许来源不匹配，或来源头格式存在歧义。',
      'The origin does not match the service or allowed origins, or its header is ambiguous.',
    ),
    'csrf.invalid_host': tr(
      '无法从请求 Host 解析有效的服务来源。',
      'The request Host does not identify a valid service origin.',
    ),
  };
  const labels = {
    title: tr('命中原因与排查信息', 'Detection details'),
    copy: tr('复制排查信息', 'Copy diagnostics'),
    retry: tr('重新读取', 'Reload'),
    position: tr('命中位置', 'Location'),
    code: tr('原因代码', 'Reason code'),
    rule: tr('规则 ID', 'Rule ID'),
    mode: tr('本次处理', 'Disposition'),
    privacy: tr(
      '复制内容仅含请求编号、时间、处理结果和规则元数据，不包含请求报文、Cookie、令牌或参数值。',
      'Copied diagnostics contain request ID, time, outcome and rule metadata only; no request payload, cookies, tokens or parameter values.',
    ),
    truncated: tr(
      '请求正文检测被截断，结论仅覆盖已检查的部分。',
      'Request body inspection was truncated; findings cover only the inspected portion.',
    ),
  };
  const messages: Record<string, string> = {
    missing: tr(
      '没有找到这条请求的检测明细，可能未保存或已超过保留期；不代表没有命中。',
      'No retained detection details were found. They may not have been saved or may have expired; this does not mean there were no matches.',
    ),
    unavailable: tr(
      '检测明细读取失败，当前无法确定命中位置。请重新读取。',
      'Detection details could not be read. The match location is unknown. Please reload.',
    ),
    invalid: tr(
      '检测明细不完整或与这条请求不一致，无法用于判定。请重新读取。',
      'Detection details are incomplete or do not match this request. Please reload.',
    ),
    unsupported: tr(
      '当前接口没有提供检测明细，不能仅凭脱敏报文确定命中原因。',
      'This server does not provide detection details. A redacted request alone cannot establish the cause.',
    ),
  };
  const hits = (state === 'available' && Array.isArray(data?.hits) ? (data.hits as DiagnosticHit[]) : []).map(
    (hit) => ({
      category: hit.category || missing,
      rule: hit.rule_id || missing,
      position: sources[hit.source || ''] || missing,
      code: hit.code || missing,
      mode: hit.trusted
        ? tr('可信来源命中', 'Trusted-source match')
        : hit.observe
          ? tr('仅观察，未由此项拦截', 'Observed; this match did not block')
          : tr('非观察命中；最终动作见请求结果', 'Enforced match; see the final request outcome'),
      observe: !!hit.observe,
      reason:
        reasons[`${hit.category}.${hit.code}`] ||
        (hit.code
          ? tr('检测器在该位置匹配到规则特征。', 'The detector matched a rule signature at this location.')
          : tr(
              '此检测器没有提供结构化位置或原因代码。',
              'This detector did not provide a structured location or reason code.',
            )),
    }),
  );
  const summary =
    messages[state] ||
    (['阻止', '禁止'].includes(detail.action) && detail.rule === 'host_not_allowed'
      ? tr(
          '请求 Host 不在服务允许的域名范围内，已在主机校验阶段拒绝，未转发到上游。这是访问策略拒绝，不能仅凭此项确认具体攻击类型。',
          'The request Host is outside the service’s allowed hostnames. It was rejected before forwarding upstream. This access-policy decision alone does not identify a specific attack type.',
        )
      : '') ||
    (hits.length
      ? tr(
          '检测记录与最终处理结果分别展示。仅观察的线索不代表确认攻击；是否拦截或响应失败，以最终动作和 HTTP 响应码为准。',
          'Detection records and the final outcome are shown separately. Observations do not confirm an attack; check the final action and HTTP status for blocks or response failures.',
        )
      : tr(
          '保留的检测记录中没有逐项规则命中；仍需结合最终动作和 HTTP 响应码判断请求结果。',
          'The retained record contains no individual rule matches. Check the final action and HTTP status for the request outcome.',
        ));
  const lines = [
    'HomeWaf request diagnostics',
    `Request ID: ${detail.req_uuid || missing}`,
    `Time: ${detail.create_time || missing}`,
    `Method: ${detail.method || missing}`,
    `Action: ${detail.action || missing}`,
    `HTTP status: ${detail.status_code || missing}`,
    `Recorded rule: ${detail.rule || missing}`,
    `Log-only mode: ${Number(detail.log_only_mode) === 1}`,
    `Detection details: ${state}`,
    summary,
  ];
  for (const hit of hits)
    lines.push(
      '',
      `Category: ${hit.category}`,
      `Rule ID: ${hit.rule}`,
      `${labels.position}: ${hit.position}`,
      `${labels.code}: ${hit.code}`,
      `${labels.mode}: ${hit.mode}`,
      hit.reason,
    );
  if (data?.body_truncated) lines.push(labels.truncated);
  return { state, labels, hits, summary, truncated: !!data?.body_truncated, report: lines.join('\n') };
}
