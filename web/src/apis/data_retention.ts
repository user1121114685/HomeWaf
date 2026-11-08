// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

// 列表
export function wafDataRetentionListApi(params: any = {}) {
  return request({
    url: '/wafhost/dataretention/list',
    method: 'post',
    data: params
  })
}

// 详情
export function wafDataRetentionDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/dataretention/detail',
    method: 'get',
    params: params
  })
}

// 编辑
export function wafDataRetentionEditApi(params: any = {}) {
  return request({
    url: '/wafhost/dataretention/edit',
    method: 'post',
    data: params
  })
}
