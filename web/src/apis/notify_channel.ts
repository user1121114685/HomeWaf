// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'

//列表
export function getNotifyChannelList(params: any = {}) {
  return request({
    url: '/notify/channel/list',
    method: 'post',
    data: params
  })
}

//添加
export function addNotifyChannel(params: any = {}) {
  return request({
    url: '/notify/channel/add',
    method: 'post',
    data: params
  })
}

//编辑
export function editNotifyChannel(params: any = {}) {
  return request({
    url: '/notify/channel/edit',
    method: 'post',
    data: params
  })
}

//删除
export function deleteNotifyChannel(params: any = {}) {
  return request({
    url: '/notify/channel/del',
    method: 'get',
    params: params
  })
}

//详情
export function getNotifyChannelDetail(params: any = {}) {
  return request({
    url: '/notify/channel/detail',
    method: 'get',
    params: params
  })
}

//测试连接
export function testNotifyChannel(params: any = {}) {
  return request({
    url: '/notify/channel/test',
    method: 'post',
    data: params
  })
}
