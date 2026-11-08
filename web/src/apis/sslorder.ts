// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

// SSL 订单列表
export function sslOrderListApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/list',
    method: 'post',
    data: params
  })
}

// SSL 订单 - 删除
export function sslOrderDelApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/del',
    method: 'get',
    params: params
  })
}

// SSL 订单 - 编辑
export function sslOrderEditApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/edit',
    method: 'post',
    data: params
  })
}

// SSL 订单 - 添加
export function sslOrderAddApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/add',
    method: 'post',
    data: params
  })
}

// SSL 订单 - 获取详情
export function sslOrderDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/detail',
    method: 'get',
    params: params
  })
}
