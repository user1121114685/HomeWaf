// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request';
import host from '@/config/host';
import { v4 as uuidv4 } from 'uuid';

const base = '/waflog/exports';
async function exportRequest(config) {
  return request({ ...config, decodeErrorData: true });
}
export const exportCapabilities = () => exportRequest({ url: `${base}/capabilities`, method: 'get' });
export const createLogExport = (data) => exportRequest({ url: base, method: 'post', data });
export const listLogExports = (params) => exportRequest({ url: base, method: 'get', params, background: true });
export const getLogExport = (id) => exportRequest({ url: `${base}/${encodeURIComponent(id)}`, method: 'get', background: true });
export const cancelLogExport = (id) => exportRequest({ url: `${base}/${encodeURIComponent(id)}/cancel`, method: 'post' });
export const deleteLogExport = (id) => exportRequest({ url: `${base}/${encodeURIComponent(id)}`, method: 'delete' });
export const logExportTicket = (id) => exportRequest({ url: `${base}/${encodeURIComponent(id)}/download-ticket`, method: 'post' });

export function startLogExportDownload(id: string, ticket: string) {
  const mode = import.meta.env.MODE || 'development';
  const api = host[mode].API;
  const url = new URL(`${api}${base}/${encodeURIComponent(id)}/download`, window.location.origin);
  url.searchParams.set('ticket', ticket);
  url.searchParams.set('X-Request-Time', Math.floor(Date.now() / 1000).toString());
  url.searchParams.set('X-Request-Id', uuidv4());
  const link = document.createElement('a');
  link.href = url.toString();
  link.referrerPolicy = 'no-referrer';
  link.rel = 'noreferrer';
  // Browser navigation streams the attachment without buffering it in JavaScript.
  document.body.appendChild(link);
  link.click();
  link.remove();
}
