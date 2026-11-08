// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//列表
export function wafHttpAuthBaseListApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthbase/list',
    method: 'post',
    data: params
  })
}
//删除
export function wafHttpAuthBaseDelApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthbase/del',
    method: 'get',
    params: params
  })
}
//编辑
export function wafHttpAuthBaseEditApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthbase/edit',
    method: 'post',
    data: params
  })
}
//添加
export function wafHttpAuthBaseAddApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthbase/add',
    method: 'post',
    data: params
  })
}
//详细
export function wafHttpAuthBaseDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthbase/detail',
    method: 'get',
    params: params
  })
}
