// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

// ---------- 审计日志 ----------

//访问认证审计日志
export function wafAccessAuditListApi(params: any = {}) {
  return request({
    url: '/wafhost/accessaudit/list',
    method: 'post',
    data: params
  })
}
