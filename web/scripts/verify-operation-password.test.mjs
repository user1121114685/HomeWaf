import assert from 'node:assert/strict';
import test from 'node:test';
import { clearAppOpPassword, getAppOpPassword, saveAppOpPassword } from '../src/utils/appOperationPassword.ts';

test('operation passwords are memory-only, expire after inactivity and cannot be restored from storage', t => {
  t.mock.timers.enable({ apis: ['Date', 'setTimeout'], now: 1000000 });
  const values = new Map([['app_op_password', 'legacy-secret'], ['app_op_password_ts', '1000000']]);
  globalThis.window = {
    sessionStorage: {
      removeItem: key => values.delete(key),
      setItem: () => assert.fail('Secrets must not be persisted'),
      getItem: () => assert.fail('Legacy credentials must not be restored'),
    },
  };
  t.after(() => { clearAppOpPassword(); delete globalThis.window; });
  assert.equal(getAppOpPassword(), '');
  assert.equal(values.size, 0);
  saveAppOpPassword('verified-operation-password');
  assert.equal(getAppOpPassword(), 'verified-operation-password');
  t.mock.timers.tick(4 * 60 * 1000);
  assert.equal(getAppOpPassword(), 'verified-operation-password');
  t.mock.timers.tick(5 * 60 * 1000);
  assert.equal(getAppOpPassword(), '');
  saveAppOpPassword('another-password');
  clearAppOpPassword();
  assert.equal(getAppOpPassword(), '');
});

test('clock rollback and unavailable browser storage do not retain stale credentials', t => {
  t.mock.timers.enable({ apis: ['Date', 'setTimeout'], now: 1000000 });
  globalThis.window = { get sessionStorage() { throw new Error('Storage disabled'); } };
  t.after(() => { clearAppOpPassword(); delete globalThis.window; });
  saveAppOpPassword('verified-operation-password');
  t.mock.timers.setTime(999999);
  assert.equal(getAppOpPassword(), '');
  saveAppOpPassword('new-password');
  assert.equal(getAppOpPassword(), 'new-password');
});
