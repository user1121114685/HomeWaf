// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request'

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
