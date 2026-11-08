// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//查询顶部的汇总天信息
export function wafstatsumdayapi(params: any = {}) {
  return request({
    url: 'wafstatsumday',
    method: 'get',
    params: params
  })
}
//查询实时QPS趋势(内存采样)
export function wafstatqpstrendapi(params?, opts?) {
  return request({
    url: 'wafstatqpstrend',
    method: 'get',
    params: params,
    ...(opts || {})
  })
}
//查询周期区间的攻击和正常信息
export function wafstatsumdayrangeapi(params: any = {}) {
  return request({
    url: 'wafstatsumdayrange',
    method: 'get',
    params: params
  })
}
//查询周期区间的IP攻击和正常信息
export function wafstatsumdaytopiprangeapi(params: any = {}) {
  return request({
    url: 'wafstatsumdaytopiprange',
    method: 'get',
    params: params
  })
}
//首页获取基本信息
export function wafStatSysinfoapi(params: any = {}) {
  return request({
    url: 'statsysinfo',
    method: 'get',
    params: params
  })
}
//首页获取运行基本信息
export function wafStatRuntimeSysinfoapi(params: any = {}) {
  return request({
    url: 'statrumtimesysinfo',
    method: 'get',
    params: params
  })
}


//【数据分析】查询周期区间国家级别攻击和正常信息
export function wafanalysisdaycountryrange(params: any = {}) {
  return request({
    url: 'wafanalysisdaycountryrange',
    method: 'get',
    params: params
  })
}
//【站点综合统计】查询站点概览（汇总卡片+站点列表）
export function wafstatsiteoverviewapi(params: any = {}) {
  return request({
    url: 'wafstatsiteoverview',
    method: 'get',
    params: params
  })
}

//【站点综合统计】查询单站点详情趋势（1h/24h/7d/30d）
export function wafstatsitedetailapi(params: any = {}) {
  return request({
    url: 'wafstatsitedetail',
    method: 'get',
    params: params
  })
}