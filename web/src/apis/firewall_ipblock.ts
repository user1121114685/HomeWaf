// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

// 查看防火墙IP封禁列表
export function wafFirewallIPBlockListApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/list',
    method: 'post',
    data: params
  })
}

// 添加防火墙IP封禁
export function wafFirewallIPBlockAddApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/add',
    method: 'post',
    data: params
  })
}

// 获取防火墙IP封禁详情
export function wafFirewallIPBlockDetailApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/detail',
    method: 'get',
    params: params
  })
}

// 编辑防火墙IP封禁
export function wafFirewallIPBlockEditApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/edit',
    method: 'post',
    data: params
  })
}

// 删除防火墙IP封禁
export function wafFirewallIPBlockDelApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/del',
    method: 'get',
    params: params
  })
}

// 批量添加防火墙IP封禁
export function wafFirewallIPBlockBatchAddApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/batch/add',
    method: 'post',
    data: params
  })
}

// 批量删除防火墙IP封禁
export function wafFirewallIPBlockBatchDelApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/batch/del',
    method: 'post',
    data: params
  })
}

// 启用防火墙IP封禁
export function wafFirewallIPBlockEnableApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/enable',
    method: 'post',
    data: params
  })
}

// 禁用防火墙IP封禁
export function wafFirewallIPBlockDisableApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/disable',
    method: 'post',
    data: params
  })
}

// 同步防火墙规则
export function wafFirewallIPBlockSyncApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/sync',
    method: 'post',
    data: params
  })
}

// 清理过期规则
export function wafFirewallIPBlockClearExpiredApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/clear/expired',
    method: 'post',
    data: params
  })
}

// 获取统计信息
export function wafFirewallIPBlockStatisticsApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/statistics',
    method: 'get',
    params: params
  })
}

// 获取当前环境的系统防火墙能力
export function wafFirewallIPBlockCapabilityApi(params: any = {}) {
  return request({
    url: '/firewall/ipblock/capability',
    method: 'get',
    params: params
  })
}
