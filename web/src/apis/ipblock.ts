// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

//添加IP黑名单
export function wafIPBlockAddApi(params: any = {}) {
  return request({
    url: '/wafhost/ipblock/add',
    method: 'post',
    data: params
  })
}
