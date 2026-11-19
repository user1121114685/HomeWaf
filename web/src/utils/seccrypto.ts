// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import { v4 as uuidv4 } from 'uuid';
import { fromBase64, toBase64, transportKeyPair, TransportSession } from './transportCipher';
import { getBaseUrl } from './usuallytool';

let active: TransportSession | undefined;
let pending: Promise<TransportSession> | undefined;

async function handshake(): Promise<TransportSession> {
  const ephemeral = transportKeyPair();
  try {
    const response = await fetch(`${getBaseUrl()}/public/seckey`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Request-Time': String(Math.floor(Date.now() / 1000)),
        'X-Request-Id': uuidv4(),
      },
      body: JSON.stringify({ epk: toBase64(ephemeral.publicKey) }),
    });
    if (!response.ok) throw new Error(`管理会话握手失败 (HTTP ${response.status})`);
    const { code, data, msg } = await response.json();
    if (code !== 0 || !data?.keyid || !data.pub) throw new Error(msg || '管理会话握手失败');
    active = new TransportSession(data.keyid, fromBase64(data.pub), ephemeral.secretKey);
    return active;
  } finally {
    ephemeral.secretKey.fill(0);
  }
}

export function ensureSecSession(): Promise<TransportSession> {
  if (active) return Promise.resolve(active);
  pending ??= handshake().finally(() => { pending = undefined; });
  return pending;
}

export function invalidateSecSession(id: string) {
  if (active?.id === id) active = undefined;
}
