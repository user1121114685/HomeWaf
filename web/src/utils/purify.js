// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// src/utils/purify.js
import DOMPurify from 'dompurify';

export function purifyHtml(content) {
  return DOMPurify.sanitize(content);
}
