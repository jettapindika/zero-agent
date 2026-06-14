// Task extraction. The agent can emit task markers in its assistant messages;
// the desktop renders them in a side panel without persisting anything new.
// Markers (case-insensitive, one per line):
//
//   [task] Build the login flow
//   [task: Verify schema] optional notes
//   [task-done] Build the login flow
//   [task-doing] Verify schema
//
// The same title across markers updates the same task's status.

import type { Message } from './zero-api';

export type TaskStatus = 'todo' | 'doing' | 'done';

export type Task = {
  id: string;
  title: string;
  status: TaskStatus;
  notes?: string;
};

const TASK_RE = /^\s*\[task(?:-(doing|done))?(?::\s*([^\]]+))?\]\s*(.*)$/i;

function normalizeTitle(value: string): string {
  return value.trim().toLowerCase().replace(/\s+/g, ' ');
}

export function extractTasks(messages: Message[]): Task[] {
  const byKey = new Map<string, Task>();
  for (const message of messages) {
    if (message.role !== 'assistant') continue;
    for (const part of message.parts) {
      const text = part.text;
      if (!text) continue;
      for (const line of text.split(/\r?\n/)) {
        const match = line.match(TASK_RE);
        if (!match) continue;
        const status: TaskStatus = match[1] === 'done' ? 'done' : match[1] === 'doing' ? 'doing' : 'todo';
        // Title can come from `[task: Title]` capture (group 2), or from the
        // trailing text after `[task]` (group 3). Notes follow when both exist.
        let title = (match[2] ?? '').trim();
        let notes = (match[3] ?? '').trim();
        if (!title && notes) {
          title = notes;
          notes = '';
        }
        if (!title) continue;
        const key = normalizeTitle(title);
        const existing = byKey.get(key);
        if (existing) {
          // Status transitions only move forward (todo -> doing -> done).
          const order: Record<TaskStatus, number> = { todo: 0, doing: 1, done: 2 };
          if (order[status] >= order[existing.status]) existing.status = status;
          if (notes && !existing.notes) existing.notes = notes;
        } else {
          byKey.set(key, { id: `task_${key}`, title, status, notes: notes || undefined });
        }
      }
    }
  }
  return [...byKey.values()];
}
