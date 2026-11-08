// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//Sensitive Manage
export function wafSensitiveListApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/list',
    method: 'post',
    data: params
  })
}
//Delete Sensitive
export function wafSensitiveDelApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/del',
    method: 'get',
    params: params
  })
}
//Edit Sensitive
export function wafSensitiveEditApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/edit',
    method: 'post',
    data: params
  })
}
//Add Sensitive
export function wafSensitiveAddApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/add',
    method: 'post',
    data: params
  })
}
//Detail Sensitive
export function wafSensitiveDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/detail',
    method: 'get',
    params: params
  })
}
//批量删除敏感词
export function wafSensitiveBatchDelApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/batch/del',
    method: 'post',
    data: params
  })
}
//清空所有敏感词
export function wafSensitiveDelAllApi(params: any = {}) {
  return request({
    url: '/wafhost/sensitive/delall',
    method: 'post',
    data: params
  })
}
