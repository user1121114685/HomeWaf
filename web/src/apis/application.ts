// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import request from '@/utils/request'
import { getAppOpPassword } from '@/utils/appOperationPassword';
export { getAppOpPassword, saveAppOpPassword, clearAppOpPassword } from '@/utils/appOperationPassword';

// 高危请求：自动附加 X-App-Op-Password（仅从短期内存缓存取）
function dangerousReq(config: any) {
  const pwd = getAppOpPassword()
  if (pwd) {
    config.headers = { ...(config.headers || {}), 'X-App-Op-Password': pwd }
  }
  return request(config)
}

// --- 普通 API ---
export function wafAppFeatureStatusApi() {
  return request({ url: '/application/app/feature_status', method: 'get' })
}
export function wafAppListApi(params: any) {
  return request({ url: '/application/app/list', method: 'post', data: params })
}
export function wafAppDetailApi(params: any) {
  return request({ url: '/application/app/detail', method: 'get', params })
}
export function wafAppStatusApi(params: any) {
  return request({ url: '/application/app/status', method: 'get', params })
}
export function wafAppLogsApi(params: any) {
  return request({ url: '/application/app/logs', method: 'get', params })
}
export function wafAppClearLogsApi(params: any) {
  return request({ url: '/application/app/clearlogs', method: 'post', data: params })
}
export function wafAppBackupsApi(params: any) {
  return request({ url: '/application/app/backups', method: 'get', params })
}
export function wafAppNetStatsApi(params: any) {
  return request({ url: '/application/app/network', method: 'get', params })
}
export function wafAppChangeLogsApi(params: any) {
  return request({ url: '/application/app/changelogs', method: 'post', data: params })
}
// 工作目录预检：解析服务端实际路径/允许范围/相对基准（旧后端无此接口，返回 404）
export function wafAppDirPreviewApi(params: any) {
  return request({ url: '/application/app/directory_preview', method: 'post', data: params, background: true })
}

// 验证操作密码是否正确（先验证再保存，避免错误密码存入 session）
export function wafAppVerifyPwdApi(pwd: string) {
  return request({ url: '/application/app/verifypwd', method: 'get', headers: { 'X-App-Op-Password': pwd } })
}

// --- 高危 API（需操作密码头）---
export function wafAppAddApi(params: any) {
  return dangerousReq({ url: '/application/app/add', method: 'post', data: params })
}
export function wafAppEditApi(params: any) {
  return dangerousReq({ url: '/application/app/edit', method: 'post', data: params })
}
export function wafAppDelApi(params: any) {
  return dangerousReq({ url: '/application/app/del', method: 'get', params })
}
export function wafAppStartApi(params: any) {
  return dangerousReq({ url: '/application/app/start', method: 'get', params })
}
export function wafAppStopApi(params: any) {
  return request({ url: '/application/app/stop', method: 'get', params }) // stop 不在高危组
}
export function wafAppRestartApi(params: any) {
  return dangerousReq({ url: '/application/app/restart', method: 'get', params })
}
// 返回带 op password header 的 request 配置，供 AppUpgrade/AppForm 直接 request()
// opPwd 优先使用显式传入值，fallback 到短期内存缓存；onUploadProgress 可选（上传进度回调）
export function buildUploadConfig(url: string, data: any, timeout: number, opPwd?: string, onUploadProgress?: (percent: number) => void): any {
  const pwd = opPwd || getAppOpPassword()
  const headers: any = pwd ? { 'X-App-Op-Password': pwd } : {}
  const config: any = { url, method: 'post', data, timeout, headers }
  if (onUploadProgress) {
    config.onUploadProgress = (e: any) => {
      if (e && e.total) onUploadProgress(Math.min(100, Math.round((e.loaded / e.total) * 100)))
    }
  }
  return config
}
export function buildRollbackConfig(params: any, opPwd?: string): any {
  const pwd = opPwd || getAppOpPassword()
  const headers: any = pwd ? { 'X-App-Op-Password': pwd } : {}
  return { url: '/application/app/rollback', method: 'get', params, timeout: 60000, headers }
}
