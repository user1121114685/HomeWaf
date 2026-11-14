export interface ProviderField {
  key: string;
  label: string;
  kind: string;
  required?: boolean;
  secret?: boolean;
  default?: string | number | boolean;
  options?: string[];
  hint?: string;
  when?: Record<string, string[]>;
}

export interface ProviderDefinition {
  type: string;
  name: string;
  hint?: string;
  fields: ProviderField[];
}

export const CONFIG_CLEAR = '__SAMWAF_CLEAR__';

export function configDefaults(definition?: ProviderDefinition): Record<string, any> {
  return Object.fromEntries((definition?.fields || []).map((field) => [
    field.key, field.default ?? (field.kind === 'boolean' ? false : ''),
  ]));
}

export function fieldVisible(field: ProviderField, config: Record<string, any>): boolean {
  return Object.entries(field.when || {}).every(([key, values]) => values.includes(String(config[key] ?? '')));
}

export function parseProviderConfig(definition: ProviderDefinition | undefined, raw: string): Record<string, any> {
  const config = raw ? JSON.parse(raw) : {};
  if (!config || Array.isArray(config) || typeof config !== 'object') throw new Error('渠道配置不是有效的 JSON 对象');
  return { ...configDefaults(definition), ...config };
}

export function providerValidationError(
  definition: ProviderDefinition | undefined,
  config: Record<string, any>,
  configuredSecrets: Record<string, boolean> = {},
): string {
  for (const field of definition?.fields || []) {
    if (!fieldVisible(field, config)) continue;
    const value = config[field.key];
    if (field.required && (value === undefined || value === null || String(value).trim() === '' || value === CONFIG_CLEAR)) {
      if (field.secret && configuredSecrets[field.key] && value !== CONFIG_CLEAR) continue;
      return `请填写${field.label}`;
    }
    if (field.kind === 'number' && value !== '' && value !== undefined && value !== null && (!Number.isInteger(Number(value)) || Number(value) < 0)) {
      return `${field.label}必须是非负整数`;
    }
  }
  return '';
}

export function serializeProviderConfig(definition: ProviderDefinition, config: Record<string, any>): string {
  // Persist only fields belonging to this provider, avoiding cross-type leftovers.
  return JSON.stringify(Object.fromEntries(definition.fields.map((field) => [
    field.key, config[field.key] ?? field.default ?? (field.kind === 'boolean' ? false : ''),
  ])));
}
