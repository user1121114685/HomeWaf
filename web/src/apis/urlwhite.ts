// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//查看URL白名单列表
export function wafURLWhiteListApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/list',
    method: 'post',
    data: params
  })
}
//删除URL白名单
export function wafURLWhiteDelApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/del',
    method: 'get',
    params: params
  })
}
//编辑URL白名单
export function wafURLWhiteEditApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/edit',
    method: 'post',
    data: params
  })
}
//添加URL白名单
export function wafURLWhiteAddApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/add',
    method: 'post',
    data: params
  })
}
//详细URL白名单
export function wafURLWhiteDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/detail',
    method: 'get',
    params: params
  })
}
//批量删除URL白名单
export function wafURLWhiteBatchDelApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/batchdel',
    method: 'post',
    data: params
  })
}
//清空所有URL白名单
export function wafURLWhiteDelAllApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/delall',
    method: 'post',
    data: params
  })
}

//切换URL白名单条目启用状态
export function wafURLWhiteToggleApi(params: any = {}) {
  return request({
    url: '/wafhost/urlwhite/toggle',
    method: 'post',
    data: params
  })
}
