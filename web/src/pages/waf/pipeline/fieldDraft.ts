import { reactive } from 'vue';
import { setAt, type Field } from './model.ts';

export interface FieldDraft {
  jsonDraft: Record<string, string>;
  jsonErrors: Record<string, string>;
  linesDraft: Record<string, string>;
}

// Each port editor owns a cloned model. Weak keys preserve unfinished text when
// advanced panels unmount, without retaining a closed editor or serializing its
// invalid drafts into a saved policy. Fields sharing a model share text by path.
const drafts = new WeakMap<object, FieldDraft>();
export function fieldDraftFor(model: object): FieldDraft {
  let draft = drafts.get(model);
  if (!draft) {
    draft = reactive({ jsonDraft: {}, jsonErrors: {}, linesDraft: {} });
    drafts.set(model, draft);
  }
  return draft;
}

export function updateJSONDraft(model: Record<string, any>, field: Field, text: string): void {
  const draft = fieldDraftFor(model), array = field.kind === 'json-array';
  draft.jsonDraft[field.path] = text;
  try {
    const value = JSON.parse(text);
    if (!(value === null && field.nullable) && (!value || typeof value !== 'object' || Array.isArray(value) !== array)) throw new Error();
    setAt(model, field.path, value);
    delete draft.jsonErrors[field.path];
  } catch {
    draft.jsonErrors[field.path] = `请输入合法 JSON ${array ? '数组' : '对象'}；修正前不能保存。`;
  }
}

export function fieldsInvalid(model: object, definitions: Field[]): boolean {
  const errors = fieldDraftFor(model).jsonErrors;
  return definitions.some(field => !!errors[field.path]);
}
