import type { PortConfig } from '../pipeline/model';

export interface AssessmentHandoff {
  config?: PortConfig;
  source?: string;
}

// Drafts and samples stay in memory; only a rendezvous ID enters the URL.
export function provideAssessmentHandoff(id: string, value: AssessmentHandoff) {
  const channel = new BroadcastChannel('homewaf-assessment-' + id);
  const snapshot = JSON.parse(JSON.stringify(value));
  channel.onmessage = (event) => {
    if (event.data?.type === 'ready') channel.postMessage({ type: 'draft', value: snapshot });
  };
  const timer = window.setTimeout(() => channel.close(), 120000);
  return () => {
    window.clearTimeout(timer);
    channel.close();
  };
}
export function receiveAssessmentHandoff(id: string, accept: (value: AssessmentHandoff) => void, fail: () => void) {
  if (!/^[0-9a-f-]{36}$/i.test(id)) {
    fail();
    return () => {};
  }
  const channel = new BroadcastChannel('homewaf-assessment-' + id);
  let interval: number, timer: number;
  const close = () => {
    window.clearInterval(interval);
    window.clearTimeout(timer);
    channel.close();
  };
  channel.onmessage = (event) => {
    if (event.data?.type !== 'draft' || !event.data.value || typeof event.data.value !== 'object') return;
    accept(event.data.value);
    close();
  };
  interval = window.setInterval(() => channel.postMessage({ type: 'ready' }), 200);
  timer = window.setTimeout(() => {
    close();
    fail();
  }, 10000);
  channel.postMessage({ type: 'ready' });
  return close;
}
