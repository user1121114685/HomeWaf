// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request'

// 规则格式预览
export function wafRuleFormatApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/format',
    method: 'post',
    data: params
  })
}
