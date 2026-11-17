// An operation password may be reused briefly within this document, never persisted.
const idleTimeout = 5 * 60 * 1000;
let password = '';
let lastUsed = 0;
let expiryTimer: ReturnType<typeof setTimeout> | undefined;

function removeLegacyPassword() {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.removeItem('app_op_password');
    window.sessionStorage.removeItem('app_op_password_ts');
  } catch {
    // Storage can be disabled; the in-memory credential lifecycle still works.
  }
}

export function clearAppOpPassword() {
  password = '';
  lastUsed = 0;
  if (expiryTimer !== undefined) clearTimeout(expiryTimer);
  expiryTimer = undefined;
  removeLegacyPassword();
}

function touchPassword() {
  lastUsed = Date.now();
  if (expiryTimer !== undefined) clearTimeout(expiryTimer);
  expiryTimer = setTimeout(clearAppOpPassword, idleTimeout);
}

export function getAppOpPassword(): string {
  removeLegacyPassword();
  if (!password) return '';
  const idle = Date.now() - lastUsed;
  if (idle < 0 || idle >= idleTimeout) {
    clearAppOpPassword();
    return '';
  }
  touchPassword();
  return password;
}

export function saveAppOpPassword(value: string) {
  clearAppOpPassword();
  password = value;
  if (password) touchPassword();
}

removeLegacyPassword();
if (typeof window !== 'undefined') window.addEventListener('pagehide', clearAppOpPassword);
