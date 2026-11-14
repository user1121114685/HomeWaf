import test from 'node:test';
import assert from 'node:assert/strict';
import { reactive } from 'vue';
import { fieldDraftFor, fieldsInvalid, updateJSONDraft } from '../src/pages/waf/pipeline/fieldDraft.ts';

const routes = { path: 'routes', label: '路由', kind: 'json-array', help: '' };
const response = { path: 'response', label: '响应', kind: 'json', help: '' };

test('invalid JSON survives remount without replacing the last valid policy', () => {
  const model = reactive({ routes: [{ hosts: ['nas.test'] }] });
  const original = structuredClone(JSON.parse(JSON.stringify(model.routes)));
  updateJSONDraft(model, routes, '[unfinished');
  assert.deepEqual(JSON.parse(JSON.stringify(model.routes)), original);
  assert.equal(fieldsInvalid(model, [routes]), true);
  // A remounted Fields resolves exactly this same port model, not a new draft.
  const remounted = fieldDraftFor(model);
  assert.equal(remounted.jsonDraft.routes, '[unfinished');
  updateJSONDraft(model, routes, '[{"hosts":["updated.test"]}]');
  assert.equal(fieldsInvalid(model, [routes]), false);
  assert.equal(model.routes[0].hosts[0], 'updated.test');
});

test('field groups sharing a model report only their own validation errors', () => {
  const model = reactive({ routes: [], response: {} });
  updateJSONDraft(model, routes, '{}');
  assert.equal(fieldsInvalid(model, [routes]), true);
  assert.equal(fieldsInvalid(model, [response]), false);
  updateJSONDraft(model, response, '{"compression":{"is_enable":true}}');
  assert.equal(fieldsInvalid(model, [routes]), true);
  assert.equal(fieldsInvalid(model, [response]), false);
});

test('a reopened port or applied template never inherits another model draft', () => {
  const previous = reactive({ routes: [] }), next = reactive({ routes: [] });
  updateJSONDraft(previous, routes, 'broken');
  fieldDraftFor(previous).linesDraft.trusted_proxies = '192.0.2.0/24\n';
  assert.equal(fieldsInvalid(next, [routes]), false);
  assert.deepEqual(fieldDraftFor(next).jsonDraft, {});
  assert.deepEqual(fieldDraftFor(next).linesDraft, {});
});

test('JSON array/object/null constraints remain enforced when repeatedly edited', () => {
  const model = reactive({ routes: [], response: {} });
  updateJSONDraft(model, routes, 'null');
  assert.equal(fieldsInvalid(model, [routes]), true);
  updateJSONDraft(model, routes, '[]');
  assert.equal(fieldsInvalid(model, [routes]), false);
  updateJSONDraft(model, response, '[]');
  assert.equal(fieldsInvalid(model, [response]), true);
  updateJSONDraft(model, { ...response, nullable: true }, 'null');
  assert.equal(fieldsInvalid(model, [response]), false);
  assert.equal(model.response, null);
});
