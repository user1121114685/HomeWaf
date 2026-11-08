// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//在线会话列表
export function wafHttpAuthSessionListApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthsession/list',
    method: 'post',
    data: params
  })
}
//踢下线单条
export function wafHttpAuthSessionKickApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthsession/kick',
    method: 'get',
    params: params
  })
}
//踢下线某用户在本站点的全部会话
export function wafHttpAuthSessionKickByUserApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthsession/kickbyuser',
    method: 'post',
    data: params
  })
}
//清空本站点全部会话
export function wafHttpAuthSessionKickAllApi(params: any = {}) {
  return request({
    url: '/wafhost/httpauthsession/kickall',
    method: 'post',
    data: params
  })
}
