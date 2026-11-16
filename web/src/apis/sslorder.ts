// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request';

export interface ZeroSSLAccountStatus {
  has_access_key: boolean;
  has_eab_kid: boolean;
  has_eab_hmac_key: boolean;
  configured: boolean;
}

export function zeroSSLAccountStatusApi() {
  return request({ url: '/wafhost/sslorder/zerossl/account', method: 'get' });
}

export function saveZeroSSLAccountApi(data: {
  mode: string;
  access_key?: string;
  eab_kid?: string;
  eab_hmac_key?: string;
}) {
  return request({ url: '/wafhost/sslorder/zerossl/account', method: 'post', data });
}

export interface SSLCertificateTarget {
  value: string;
  label: string;
  source: 'pipeline' | 'host';
  port: number;
  protocol: 'http' | 'https';
  domains: string[];
  http01_ready: boolean;
}

export function sslCertificateTargetsApi() {
  return request({ url: '/wafhost/sslorder/targets', method: 'get' });
}

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

export function sslOrderRetryApi(data: { id: string; request_id: string }) {
  return request({ url: '/wafhost/sslorder/retry', method: 'post', data });
}

export function sslOrderLogsApi(id: string) {
  return request({ url: '/wafhost/sslorder/logs', method: 'get', params: { id }, background: true });
}
