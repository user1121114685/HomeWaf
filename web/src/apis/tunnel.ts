// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//列表
export function wafTunnelListApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/list',
    method: 'post',
    data: params
  })
}
//删除
export function wafTunnelDelApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/del',
    method: 'get',
    params: params
  })
}
//编辑
export function wafTunnelEditApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/edit',
    method: 'post',
    data: params
  })
}
//添加
export function wafTunnelAddApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/add',
    method: 'post',
    data: params
  })
}
//详细
export function wafTunnelDetailApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/detail',
    method: 'get',
    params: params
  })
}
//连接信息
export function wafTunnelConnectionApi(params: any = {}) {
  return request({
    url: '/tunnel/tunnel/connections',
    method: 'get',
    params: params
  })
}