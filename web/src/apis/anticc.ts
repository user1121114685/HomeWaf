// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//查看抵御CC攻击列表
export function wafAntiCCListApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/list',
    method: 'post',
    data: params
  })
}
//删除抵御CC攻击
export function wafAntiCCDelApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/del',
    method: 'get',
    params: params
  })
}
//编辑抵御CC攻击
export function wafAntiCCEditApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/edit',
    method: 'post',
    data: params
  })
}
//添加抵御CC攻击
export function wafAntiCCAddApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/add',
    method: 'post',
    data: params
  })
}
//详细抵御CC攻击
export function wafAntiCCDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/detail',
    method: 'get',
    params: params
  })
}

/**
 * 封禁ip列表
 * @param params
 */
export function wafAntiCCBanIPListApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/baniplist',
    method: 'get',
    params: params
  })
}

/**
 *  移除ip方法
 * @param params
 */
export function wafAntiCCRemoveBanIpApi(params: any = {}) {
  return request({
    url: '/wafhost/anticc/removebanip',
    method: 'post',
    data: params
  })
}
