export interface HoneypotLure {
  id: string;
  title: string;
  message: string;
  weight: number;
}

// Templates populate the existing text fields only. They never change routing,
// trap paths, recording, limits or service policies.
export const honeypotTemplates = [
  { id: 'admin', name: '管理入口', description: '通用后台登录', title: '管理控制台', message: '请输入管理账号。' },
  {
    id: 'device',
    name: '设备管理',
    description: '家庭设备与网络管理',
    title: '设备管理',
    message: '请使用管理员账号登录，以查看设备状态和网络设置。',
  },
  {
    id: 'files',
    name: '文件管理',
    description: '文件与共享目录入口',
    title: '文件管理中心',
    message: '请登录后管理文件和共享目录。',
  },
  {
    id: 'apps',
    name: '应用运维',
    description: '应用运行与维护入口',
    title: '应用管理',
    message: '请登录后查看应用运行状态和维护设置。',
  },
] as const;

export const defaultHoneypotLure: HoneypotLure = {
  id: 'default',
  title: '管理控制台',
  message: '请输入管理账号。',
  weight: 1,
};

export type HoneypotPreviewState = 'visit' | 'login' | 'busy';

function escapeHTML(text: string) {
  return text.replace(
    /[&<>"']/g,
    (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]!,
  );
}

// Mirrors wafpipeline/honeypot.go. The iframe is additionally sandboxed; it has
// no scripts, network resources, credential collection or submission behavior.
export function honeypotPreviewHTML(lure: Pick<HoneypotLure, 'title' | 'message'>, state: HoneypotPreviewState) {
  const head =
    '<!doctype html><html lang="zh"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; style-src \'unsafe-inline\'; form-action \'none\'; base-uri \'none\'">';
  if (state === 'busy') return head + '<title>繁忙响应预览</title><body><pre>Service busy\n</pre></body></html>';
  return (
    head +
    `<title>${escapeHTML(lure.title)}</title><style>body{font:16px system-ui;max-width:360px;margin:12vh auto;padding:24px}label,input,button{display:block;box-sizing:border-box;width:100%;margin:12px 0}input,button{padding:12px}small{opacity:.7}</style><body><h1>${escapeHTML(lure.title)}</h1><p>${escapeHTML(lure.message)}</p><form><label>账号<input readonly tabindex="-1" autocomplete="off"></label><label>密码<input type="password" readonly tabindex="-1" autocomplete="off"></label><button type="button" tabindex="-1">登录</button></form>${state === 'login' ? '<p>账号或密码错误，请稍后重试。</p>' : ''}</body></html>`
  );
}
