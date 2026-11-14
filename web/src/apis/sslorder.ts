// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request';

export interface DNSProviderField {
  name: string;
  description: string;
  advanced: boolean;
}

export interface DNSProvider {
  code: string;
  name: string;
  aliases: string[];
  documentation: string;
  fields: DNSProviderField[];
  instructions: string;
  example: string;
  unavailable?: string;
}

export function sslDNSProvidersApi() {
  return request({ url: '/wafhost/sslorder/dns/providers', method: 'get' });
}

export function sslDNSCredentialStatusApi(provider: string, group: string) {
  return request({ url: '/wafhost/sslorder/dns/credentials', method: 'get', params: { provider, group } });
}

// SSL 订单列表
export function sslOrderListApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/list',
    method: 'post',
    data: params,
  });
}

// SSL 订单 - 删除
export function sslOrderDelApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/del',
    method: 'get',
    params: params,
  });
}

// SSL 订单 - 编辑
export function sslOrderEditApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/edit',
    method: 'post',
    data: params,
  });
}

// SSL 订单 - 添加
export function sslOrderAddApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/add',
    method: 'post',
    data: params,
  });
}

// SSL 订单 - 获取详情
export function sslOrderDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/sslorder/detail',
    method: 'get',
    params: params,
  });
}
