// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
import { AI_GEN_TIMEOUT } from '@/config/requestTimeout'

// 规则格式预览
export function wafRuleFormatApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/format',
    method: 'post',
    data: params
  })
}

//AI 生成规则（后端调用已配置的 GPT，生成后服务端校验）
//AI 生成本质耗时较长（后端最多 3 次校验修复），单独用长超时，避免被全局默认超时误杀
export function wafRuleAiGenApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/aigen',
    method: 'post',
    data: params,
    timeout: AI_GEN_TIMEOUT
  })
}
//获取"复制给AI"的提示词（唯一来源在后端）
export function wafRuleAiPromptApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/aiprompt',
    method: 'get',
    params: params
  })
}
