import request from '@/utils/request';

export async function camouflageGet<T = any>(path: string, params: Record<string, unknown> = {}): Promise<T> {
  const result = (await request({ url: `/pipeline/camouflage/${path}`, method: 'get', params })) as any;
  if (result.code !== 0) throw new Error(result.msg || '读取协议伪装配置失败');
  return result.data as T;
}

export async function camouflagePost<T = any>(path: string, data: unknown): Promise<T> {
  const result = (await request({ url: `/pipeline/camouflage/${path}`, method: 'post', data })) as any;
  if (result.code !== 0) throw new Error(result.msg || '协议伪装操作失败');
  return result.data as T;
}
