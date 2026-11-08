// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//获取当前登录账号的界面偏好
export function get_ui_preference_api(params: any = {}) {
  return request({
    url: 'uipreference/get',
    method: 'get',
    params: params
  })
}
//保存当前登录账号的界面偏好
export function save_ui_preference_api(data: any = {}) {
  return request({
    url: 'uipreference/save',
    method: 'post',
    data: data
  })
}
