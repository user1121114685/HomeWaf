// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//列表
export function wafPrivateInfoListApi(params: any = {}) {
  return request({
    url: '/wafhost/privateinfo/list',
    method: 'post',
    data: params
  })
}
//删除
export function wafPrivateInfoDelApi(params: any = {}) {
  return request({
    url: '/wafhost/privateinfo/del',
    method: 'get',
    params: params
  })
}
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
//详细
export function wafPrivateInfoDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/privateinfo/detail',
    method: 'get',
    params: params
  })
}
