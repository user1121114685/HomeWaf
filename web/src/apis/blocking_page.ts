// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//列表
export function wafBlockingPageListApi(params: any = {}) {
  return request({
    url: '/wafhost/blockingpage/list',
    method: 'post',
    data: params
  })
}
//删除
export function wafBlockingPageDelApi(params: any = {}) {
  return request({
    url: '/wafhost/blockingpage/del',
    method: 'get',
    params: params
  })
}
//编辑
export function wafBlockingPageEditApi(params: any = {}) {
  return request({
    url: '/wafhost/blockingpage/edit',
    method: 'post',
    data: params
  })
}
//添加
export function wafBlockingPageAddApi(params: any = {}) {
  return request({
    url: '/wafhost/blockingpage/add',
    method: 'post',
    data: params
  })
}
//详细
export function wafBlockingPageDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/blockingpage/detail',
    method: 'get',
    params: params
  })
}
//未匹配网站内置页面（预览用）
export function wafBlockingPageUnmatchedBuiltinApi() {
  return request({
    url: '/wafhost/blockingpage/unmatchedbuiltin',
    method: 'get',
  })
}
