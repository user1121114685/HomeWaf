import request from '@/utils/request';

export type AssessmentProgress = { state: string; done: number; total: number };
async function call(url: string, data?: unknown, signal?: AbortSignal) {
  const response = (await request({
    url: `/pipeline/${url}`,
    method: data ? 'post' : 'get',
    data,
    signal,
    timeout: 35000,
  })) as any;
  if (response.code !== 0) throw new Error(response.msg || '检测失败，请重试');
  return response.data;
}
export async function runAssessment(data: unknown, signal: AbortSignal, progress: (value: AssessmentProgress) => void) {
  // Keep the start response so cancellation can always address the accepted job.
  const started = await call('assessment', { ...(data as object), background: true });
  if (!started.job) return started;
  const id = started.job.id;
  let complete = false;
  try {
    progress(started.job);
    while (!signal.aborted) {
      const job = await call(`assessment/jobs?id=${encodeURIComponent(id)}`, undefined, signal);
      progress(job);
      if (job.state === 'complete') {
        complete = true;
        return job.result;
      }
      if (job.state === 'failed' || job.state === 'cancelled') {
        complete = true;
        throw new Error(job.error || '检测未完成');
      }
      await new Promise<void>((resolve) => {
        const finish = () => {
          clearTimeout(timer);
          signal.removeEventListener('abort', finish);
          resolve();
        };
        const timer = setTimeout(finish, 500);
        signal.addEventListener('abort', finish, { once: true });
      });
    }
    throw new Error('检测已取消，输入已保留。');
  } finally {
    if (!complete) await call('assessment/cancel', { id }).catch(() => undefined);
  }
}
