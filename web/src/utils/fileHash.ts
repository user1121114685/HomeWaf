// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex } from '@noble/hashes/utils.js';

export const hashBytes = (data: ArrayBuffer) => bytesToHex(sha256(new Uint8Array(data)));
