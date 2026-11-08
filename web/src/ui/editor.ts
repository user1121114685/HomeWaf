// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

import { EditorView, keymap } from '@codemirror/view';
import { EditorState, Compartment } from '@codemirror/state';
import { basicSetup } from 'codemirror';
import { sql } from '@codemirror/lang-sql';
import { autocompletion, startCompletion } from '@codemirror/autocomplete';

// Only the small editor contract used by HomeWaf is adapted; CodeMirror 5 is not shipped.
export default {
  Pos: (line: number, ch: number) => ({ line, ch }),
  fromTextArea(textarea: HTMLTextAreaElement, options: any = {}) {
    const listeners = new Map<string, Set<Function>>();
    let hint = options.hintOptions?.hint;
    const parent = document.createElement('div');
    parent.className = 'CodeMirror homewaf-code-editor';
    textarea.hidden = true;
    textarea.after(parent);
    const readonly = new Compartment();
    let view: EditorView;
    const dispatch = (name: string, ...args: any[]) => {
      for (const fn of listeners.get(name) || []) fn(api, ...args);
    };
    const offset = (position: any) => view.state.doc.line(position.line + 1).from + position.ch;
    const api = {
      getValue: () => view.state.doc.toString(),
      setValue: (value: string) =>
        view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: String(value ?? '') } }),
      getCursor: () => {
        const position = view.state.selection.main.head;
        const line = view.state.doc.lineAt(position);
        return { line: line.number - 1, ch: position - line.from };
      },
      getLine: (line: number) => view.state.doc.line(line + 1).text,
      replaceSelection: (value: string) => view.dispatch(view.state.replaceSelection(value)),
      focus: () => view.focus(),
      refresh: () => view.requestMeasure(),
      on: (name: string, fn: Function) => {
        if (!listeners.has(name)) listeners.set(name, new Set());
        listeners.get(name)!.add(fn);
      },
      off: (name: string, fn: Function) => listeners.get(name)?.delete(fn),
      showHint: (settings: any) => {
        hint = settings?.hint || hint;
        startCompletion(view);
      },
      setOption: (key: string, value: any) => {
        if (key === 'readOnly') view.dispatch({ effects: readonly.reconfigure(EditorState.readOnly.of(!!value)) });
      },
      toTextArea: () => {
        textarea.value = view.state.doc.toString();
        view.destroy();
        parent.remove();
        textarea.hidden = false;
        listeners.clear();
      },
      getWrapperElement: () => parent,
    };
    view = new EditorView({
      parent,
      state: EditorState.create({
        doc: textarea.value || '',
        extensions: [
          basicSetup,
          sql(),
          readonly.of(EditorState.readOnly.of(!!options.readOnly)),
          EditorView.lineWrapping,
          keymap.of([
            {
              key: 'Ctrl-Space',
              run: () => {
                api.showHint(options.hintOptions);
                return true;
              },
            },
          ]),
          autocompletion({
            override: [
              () => {
                if (!hint) return null;
                const result = hint(api);
                return {
                  from: offset(result.from),
                  to: offset(result.to),
                  options: (result.list || []).map((item: any) =>
                    typeof item === 'string'
                      ? { label: item }
                      : { label: item.text, displayLabel: item.displayText, detail: item.desc, apply: item.text },
                  ),
                };
              },
            ],
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              textarea.value = update.state.doc.toString();
              dispatch('change');
              update.changes.iterChanges((_a, _b, _c, _d, inserted) =>
                dispatch('inputRead', { text: inserted.toString().split('\n') }),
              );
            }
            if (update.selectionSet) dispatch('cursorActivity');
          }),
          EditorView.theme({
            '&': { fontSize: '14px', minHeight: '180px' },
            '.cm-scroller': { fontFamily: 'ui-monospace,monospace', overflow: 'auto' },
          }),
        ],
      }),
    });
    return api;
  },
};
