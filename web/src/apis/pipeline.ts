import request from '@/utils/request';

export async function pipelineGet<T = any>(path: string, params: Record<string, unknown> = {}): Promise<T> {
  const result = await request({ url: '/pipeline/' + path, method: 'get', params }) as unknown as { code: number; data: T; msg: string };
  if (result.code !== 0) throw new Error(result.msg || '读取端口配置失败');
  return result.data;
}
export async function pipelinePost<T = any>(path: string, data: unknown): Promise<T> {
  const result = await request({ url: '/pipeline/' + path, method: 'post', data }) as unknown as { code: number; data: T; msg: string };
  if (result.code !== 0) throw new Error(result.msg || '操作未完成');
  return result.data;
}
