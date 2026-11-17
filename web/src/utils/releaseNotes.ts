export type ReleaseNoteGroup = { version: string; items: string[] };

// Release metadata is untrusted text. Render these strings with Vue interpolation.
export function releaseNoteItems(description: string): string[] {
  const result: string[] = [];
  for (const raw of String(description || '').split(/\r?\n/)) {
    let text = raw.trim().replace(/^(?:[-*+] |\d+\. )/, '').replace(/^#+\s*/, '');
    if (!text || /^(?:changelog|what'?s changed|更新内容|更新说明)$/i.test(text) || /^(?:HomeWaf\s+)?v\d+\.\d+\.\d+[^\s]*$/i.test(text)) continue;
    text = text.replace(/^(?:\[([a-f\d]{7,40})\]\([^)]*\)|[a-f\d]{7,40})\s+/i, '');
    text = text.replace(/^(?:feat|fix|perf|refactor|chore|docs|test|ci|build)(?:\([^)]*\))?!?:\s*/i, '');
    text = text.replace(/\[([^\]]+)\]\([^)]*\)/g, '$1').trim();
    if (text && !result.includes(text)) result.push(text);
  }
  return result;
}

export function releaseNoteGroups(version: string, description: string, notes?: ReleaseNoteGroup[]): ReleaseNoteGroup[] {
  if (!Array.isArray(notes) || !notes.length) return [{ version, items: releaseNoteItems(description) }];
  return notes.map(note => ({ version: String(note.version), items: releaseNoteItems((note.items || []).join('\n')) }));
}
