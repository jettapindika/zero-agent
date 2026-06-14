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

export function extractTasks(messages: Message[], liveAssistantText?: string): Task[] {
  const byKey = new Map<string, Task>();

  const ingest = (text: string) => {
    for (const line of text.split(/\r?\n/)) {
      const match = line.match(TASK_RE);
      if (!match) continue;
      const status: TaskStatus = match[1] === 'done' ? 'done' : match[1] === 'doing' ? 'doing' : 'todo';
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
        const order: Record<TaskStatus, number> = { todo: 0, doing: 1, done: 2 };
        if (order[status] >= order[existing.status]) existing.status = status;
        if (notes && !existing.notes) existing.notes = notes;
      } else {
        byKey.set(key, { id: `task_${key}`, title, status, notes: notes || undefined });
      }
    }
  };

  for (const message of messages) {
    if (message.role !== 'assistant') continue;
    for (const part of message.parts) {
      if (part.text) ingest(part.text);
    }
  }
  if (liveAssistantText && liveAssistantText.length > 0) {
    ingest(liveAssistantText);
  }
  return [...byKey.values()];
}
