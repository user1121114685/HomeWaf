// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
//查看一键修改记录列表
export function wafOneKeyModListApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/list',
    method: 'post',
    data: params
  })
}
//删除一键修改记录
export function wafOneKeyModDelApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/del',
    method: 'get',
    params: params
  })
}
//详细一键修改记录
export function wafOneKeyModDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/detail',
    method: 'get',
    params: params
  })
}
//触发一键修改
export function wafDoOneKeyModApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/doModify',
    method: 'post',
    data: params
  })
}
//还原
export function wafRestoreOneKeyModApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/restore?id=' + params.id,
    method: 'get',
  })
}
//解析nginx配置为待添加主机候选（source: text 粘贴 / scan 扫描目录）
export function wafParseNginxApi(params: any = {}) {
  return request({
    url: '/wafhost/onekeymod/parseNginx',
    method: 'post',
    data: params
  })
}
