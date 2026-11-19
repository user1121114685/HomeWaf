import { fromBase64, toBase64, transportKeyPair, TransportSession } from '../src/utils/transportCipher.ts';

const serverKeys = transportKeyPair();
const sessions = new Map();

export function acceptHandshake({ epk }) {
  const id = crypto.randomUUID();
  const session = new TransportSession(id, fromBase64(epk), serverKeys.secretKey);
  sessions.set(id, session);
  return { code: 0, data: { keyid: id, pub: toBase64(serverKeys.publicKey), ttl: 43200 } };
}

export function fixtureSession(id) {
  const session = sessions.get(id);
  if (!session) throw new Error('Fixture request has no negotiated session');
  return session;
}

export function routeSession(route) {
  return fixtureSession(route.request().headers()['x-key-id']);
}

export async function handleTransportHandshake(route) {
  const req = route.request();
  const endpoint = new URL(req.url()).pathname.split('/api/v1/')[1];
  if (endpoint === 'heartbeat') {
    const data = routeSession(route).encrypt(JSON.stringify({ login_account: 'fixture', role: 'superAdmin' }));
    await route.fulfill({ json: { code: 0, msg: '', data } });
    return true;
  }
  if (endpoint !== 'public/seckey') return false;
  if (req.method() !== 'POST') throw new Error('Expected POST handshake');
  await route.fulfill({ json: acceptHandshake(req.postDataJSON()) });
  return true;
}
