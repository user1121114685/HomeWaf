// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'


//导出excel文件
export function export_api(params: any = {}) {
  return request.download({
    url: 'export',
    method: 'get',
    responseType: 'blob',
    timeout: 20000,
    params: params
  })
}
//心跳数据
export function heartbeat_api() {
  return request({
    url: 'heartbeat',
    method: 'get',
    timeout: 10000
  })
}
