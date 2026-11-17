import { randomInt } from 'node:crypto';

// Windows can allocate IRC ports for listen(0), which browsers/fetch reject.
// Retry occupied or OS-reserved ports without exposing fixtures off loopback.
export async function listenLoopbackForTest(server) {
  for (let attempt = 0; attempt < 32; attempt++) {
    try {
      return await new Promise((resolve, reject) => {
        const failed = (error) => { server.off('listening', ready); reject(error); };
        const ready = () => { server.off('error', failed); resolve(server.address().port); };
        server.once('error', failed);
        server.once('listening', ready);
        server.listen(randomInt(20000, 60000), '127.0.0.1');
      });
    } catch (error) {
      if (error.code !== 'EADDRINUSE' && error.code !== 'EACCES') throw error;
    }
  }
  throw new Error('Could not allocate a loopback test port');
}
