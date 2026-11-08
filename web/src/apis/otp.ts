// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//初始化
export function wafOtpInitApi(params: any = {}) {
  return request({
    url: '/wafhost/otp/init',
    method: 'get',
    params: params
  })
}

// 绑定OTP
export function wafOtpBindApi(params: any = {}) {
  return request({
    url: '/wafhost/otp/bind',
    method: 'post',
    data: params
  })
}

//解绑OTP
export function wafOtpUnBindApi(params: any = {}) {
  return request({
    url: '/wafhost/otp/unbind',
    method: 'post',
    data: params
  })
}
