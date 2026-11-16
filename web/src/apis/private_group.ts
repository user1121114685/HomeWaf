// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

export function wafPrivateGroupListByBelongCloudApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/listbybelongcloud',
    method: 'post',
    data: params
  })
}

//添加
export function wafPrivateGroupAddApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/add',
    method: 'post',
    data: params
  })
}
