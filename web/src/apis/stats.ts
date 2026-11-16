// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request'

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
