// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request'

//添加IP白名单
export function wafIPWhiteAddApi(params: any = {}) {
  return request({
    url: '/wafhost/ipwhite/add',
    method: 'post',
    data: params
  })
}
