// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import request from '@/utils/request'
import { AI_GEN_TIMEOUT } from '@/config/requestTimeout'
//查看规则列表
export function wafRuleListApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/list',
    method: 'post',
    data: params
  })
}
//删除规则
export function wafRuleDelApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/del',
    method: 'get',
    params: params
  })
}
//编辑规则
export function wafRuleEditApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/edit',
    method: 'post',
    data: params
  })
}
//添加规则
export function wafRuleAddApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/add',
    method: 'post',
    data: params
  })
}
//详细规则
export function wafRuleDetailApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/detail',
    method: 'get',
    params: params
  })
}
//批量删除规则
export function wafRuleBatchDelApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/batchdel',
    method: 'post',
    data: params
  })
}
//清空所有规则
export function wafRuleDelAllApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/delall',
    method: 'post',
    data: params
  })
}
// 规则格式预览
export function wafRuleFormatApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/format',
    method: 'post',
    data: params
  })
}
//更改规则状态
export function changeRuleStatus(params: any = {}) {
  return request({
    url: '/wafhost/rule/rulestatus',
    method: 'get',
    params: params
  })
}
//测试规则
export function wafRuleTestApi(params: any = {}) {
  return request({
    url: '/wafhost/rule/test',
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
