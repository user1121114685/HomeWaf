import assert from 'node:assert/strict';
import test from 'node:test';
import fs from 'node:fs';
import { configDefaults, parseProviderConfig, providerValidationError, serializeProviderConfig, fieldVisible, CONFIG_CLEAR } from '../src/pages/waf/notify_channel/provider_config.ts';

const catalogue = JSON.parse(fs.readFileSync(new URL('../../wafnotify/channels/catalog.json', import.meta.url), 'utf8'));
const definition = type => catalogue.find(item => item.type === type);

test('all direct providers have independent defaults and no Android dependency', () => {
  assert.equal(catalogue.length, 14);
  assert.equal(new Set(catalogue.map(item => item.type)).size, 14);
  assert.ok(!catalogue.some(item => ['sms', 'urlscheme'].includes(item.type)));
  const first = configDefaults(definition('bark')), second = configDefaults(definition('bark'));
  first.group = 'different';
  assert.notEqual(first.group, second.group);
});

test('required credentials can remain redacted during an edit, but not during creation or explicit clearing', () => {
  const bark = definition('bark'), config = configDefaults(bark);
  assert.match(providerValidationError(bark, config), /推送地址/);
  assert.equal(providerValidationError(bark, config, { server: true }), '');
  config.server = CONFIG_CLEAR;
  assert.match(providerValidationError(bark, config, { server: true }), /推送地址/);
});

test('socket settings validate only fields belonging to the selected transport', () => {
  const socket = definition('socket');
  const config = { ...configDefaults(socket), address: 'broker.example.com', method: 'TCP' };
  assert.equal(providerValidationError(socket, config), '');
  config.method = 'MQTT';
  assert.match(providerValidationError(socket, config), /发布主题/);
  const path = socket.fields.find(field => field.key === 'path');
  assert.equal(fieldVisible(path, config), false);
  config.uriType = 'wss';
  assert.equal(fieldVisible(path, config), true);
});

test('editing roundtrip preserves options and explicit secret intent', () => {
  const telegram = definition('telegram');
  const config = parseProviderConfig(telegram, '{"chatId":"-100123","messageThreadId":7,"apiToken":""}');
  assert.equal(config.method, 'POST');
  assert.equal(providerValidationError(telegram, config, { apiToken: true }), '');
  config.apiToken = 'new-token';
  config.unrelatedSecret = 'old-bark-token';
  const saved = JSON.parse(serializeProviderConfig(telegram, config));
  assert.equal(saved.apiToken, 'new-token');
  assert.equal(saved.messageThreadId, 7);
  assert.ok(!('unrelatedSecret' in saved));
  assert.throws(() => parseProviderConfig(telegram, '[]'));
  assert.throws(() => parseProviderConfig(telegram, 'null'));
});
