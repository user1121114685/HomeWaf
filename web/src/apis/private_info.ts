// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

//编辑
export function wafPrivateInfoEditApi(params: any = {}) {
  return request({
    url: '/wafhost/privateinfo/edit',
    method: 'post',
    data: params
  })
}
//添加
export function wafPrivateInfoAddApi(params: any = {}) {
  return request({
    url: '/wafhost/privateinfo/add',
    method: 'post',
    data: params
  })
}
