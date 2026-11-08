// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//列表
export function wafPrivateGroupListApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/list',
    method: 'post',
    data: params
  })
}
export function wafPrivateGroupListByBelongCloudApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/listbybelongcloud',
    method: 'post',
    data: params
  })
}
//删除
export function wafPrivateGroupDelApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/del',
    method: 'get',
    params: params
  })
}
//编辑
export function wafPrivateGroupEditApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/edit',
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
//详细
export function wafPrivateGroupDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/privategroup/detail',
    method: 'get',
    params: params
  })
}
