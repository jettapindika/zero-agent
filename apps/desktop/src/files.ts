// extractTouchedFiles scans assistant messages for tool_call parts and returns a
// deduped list of file paths that Zero has read/written/edited in this session.

import type { Message } from './zero-api';

export type TouchedFile = {
  path: string;
  ops: ('read' | 'write' | 'edit' | 'glob' | 'grep' | 'walk' | 'ls' | 'fetch')[];
  lastTouched: number;
};

const READ_TOOLS = new Set(['read', 'glob', 'grep', 'walk', 'ls', 'fetch']);
const WRITE_TOOLS = new Set(['write', 'edit']);

const TOOLS_WHERE_PATTERN_IS_REGEX = new Set(['grep']);

function pickPath(tool: string, args: unknown): string | null {
  if (!args || typeof args !== 'object') return null;
  const a = args as Record<string, unknown>;
  const candidates = ['path', 'file', 'filename', 'url'];
  if (!TOOLS_WHERE_PATTERN_IS_REGEX.has(tool)) candidates.push('pattern');
  for (const key of candidates) {
    const v = a[key];
    if (typeof v === 'string' && v.trim() !== '') return v;
  }
  return null;
}

export function extractTouchedFiles(messages: Message[]): TouchedFile[] {
  const map = new Map<string, TouchedFile>();
  for (const message of messages) {
    if (message.role !== 'assistant') continue;
    for (const part of message.parts) {
      if (part.type !== 'tool_call') continue;
      const tool = part.toolName ?? '';
      if (!tool) continue;
      let args: unknown = {};
      try {
        args = JSON.parse(part.toolArgsJson || '{}');
      } catch {
        continue;
      }
      const path = pickPath(tool, args);
      if (!path) continue;
      if (!READ_TOOLS.has(tool) && !WRITE_TOOLS.has(tool)) continue;
      const op = tool as TouchedFile['ops'][number];
      const existing = map.get(path);
      const ts = part.createdAt ?? Date.now();
      if (existing) {
        if (!existing.ops.includes(op)) existing.ops.push(op);
        existing.lastTouched = Math.max(existing.lastTouched, ts);
      } else {
        map.set(path, { path, ops: [op], lastTouched: ts });
      }
    }
  }
  return [...map.values()].sort((a, b) => b.lastTouched - a.lastTouched);
}
