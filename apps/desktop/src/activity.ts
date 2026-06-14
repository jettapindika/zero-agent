// Live activity stream for the chat. Translates server-sent bus events into a
// capped buffer of UI activity items the ActivityPanel renders below the
// sticky loader.

import { useEffect, useRef, useState } from 'react';

const API_BASE = 'http://127.0.0.1:8910';
const MAX_ITEMS = 200;

export type ActivityKind = 'thinking' | 'tool_call' | 'tool_result' | 'text' | 'status';

export type ActivityItem = {
  id: string;
  kind: ActivityKind;
  label: string;
  detail?: string;
  status: 'pending' | 'done' | 'error';
  startedAt: number;
  finishedAt?: number;
};

type BusEvent = {
  type: string;
  projectId?: string;
  sessionId?: string;
  payload?: unknown;
};

function newId() {
  return `act_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function appendCapped(buffer: ActivityItem[], next: ActivityItem): ActivityItem[] {
  const merged = [...buffer, next];
  if (merged.length <= MAX_ITEMS) return merged;
  return merged.slice(merged.length - MAX_ITEMS);
}

function updateById(buffer: ActivityItem[], id: string, patch: Partial<ActivityItem>): ActivityItem[] {
  return buffer.map((item) => (item.id === id ? { ...item, ...patch } : item));
}

function shortenName(name: string, args: string): string {
  let parsed: Record<string, unknown> = {};
  try {
    parsed = JSON.parse(args || '{}');
  } catch {
    /* ignore — args may be partial during streaming */
  }
  const path = (parsed.path as string) || (parsed.command as string) || (parsed.pattern as string) || '';
  if (path) return `${name} · ${path}`;
  return name;
}

function summarizeResult(raw: unknown): string {
  if (typeof raw === 'string') return raw.slice(0, 240);
  if (raw && typeof raw === 'object') {
    try {
      return JSON.stringify(raw).slice(0, 240);
    } catch {
      return '[unserializable]';
    }
  }
  return '';
}

// Detect lines the model marks as thinking/reasoning. Keep this conservative
// so normal answer text never gets routed into the activity panel.
const THINKING_LINE_RE = /^\s*(?:<think(?:ing)?>|thinking:|thought:|reasoning:|process:)/i;
const THINKING_END_RE = /<\/think(?:ing)?>/i;

export function useActivityStream(sessionId: string | null, active: boolean) {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const callIdToActivityId = useRef<Map<string, string>>(new Map());
  const thinkingState = useRef<{ activityId: string | null; buffer: string; inBlock: boolean }>({
    activityId: null,
    buffer: '',
    inBlock: false,
  });

  useEffect(() => {
    if (!sessionId || !active) {
      setItems([]);
      callIdToActivityId.current.clear();
      thinkingState.current = { activityId: null, buffer: '', inBlock: false };
      return;
    }

    const url = `${API_BASE}/events?sessionId=${encodeURIComponent(sessionId)}`;
    const source = new EventSource(url);

    function handle(event: MessageEvent) {
      let parsed: BusEvent;
      try {
        parsed = JSON.parse(event.data) as BusEvent;
      } catch {
        return;
      }
      if (parsed.sessionId && parsed.sessionId !== sessionId) return;

      switch (parsed.type) {
        case 'tool.thinking': {
          // Plumbing only — we do not render a synthetic "step N/M" row.
          // Real thinking text comes from the model via part.delta below.
          break;
        }
        case 'tool.started': {
          const payload = (parsed.payload || {}) as { name?: string; args?: string };
          const name = payload.name || 'tool';
          const id = newId();
          callIdToActivityId.current.set(`${name}:${payload.args ?? ''}`, id);
          setItems((current) =>
            appendCapped(current, {
              id,
              kind: 'tool_call',
              label: shortenName(name, payload.args || ''),
              detail: payload.args,
              status: 'pending',
              startedAt: Date.now(),
            }),
          );
          break;
        }
        case 'tool.completed': {
          const payload = (parsed.payload || {}) as { name?: string; result?: unknown };
          const last = [...callIdToActivityId.current.entries()].reverse().find(([key]) => key.startsWith(`${payload.name ?? ''}:`));
          const targetId = last?.[1];
          if (targetId) callIdToActivityId.current.delete(last![0]);
          setItems((current) => {
            const id = targetId ?? newId();
            const exists = current.find((item) => item.id === id);
            if (!exists) {
              return appendCapped(current, {
                id,
                kind: 'tool_result',
                label: payload.name ?? 'tool',
                detail: summarizeResult(payload.result),
                status: 'done',
                startedAt: Date.now(),
                finishedAt: Date.now(),
              });
            }
            return updateById(current, id, {
              kind: 'tool_result',
              detail: summarizeResult(payload.result),
              status: 'done',
              finishedAt: Date.now(),
            });
          });
          break;
        }
        case 'tool.failed': {
          const payload = (parsed.payload || {}) as { name?: string; error?: string };
          const last = [...callIdToActivityId.current.entries()].reverse().find(([key]) => key.startsWith(`${payload.name ?? ''}:`));
          const targetId = last?.[1];
          if (targetId) callIdToActivityId.current.delete(last![0]);
          setItems((current) => {
            const id = targetId ?? newId();
            const exists = current.find((item) => item.id === id);
            const next: ActivityItem = {
              id,
              kind: 'tool_result',
              label: payload.name ?? 'tool',
              detail: payload.error || 'tool failed',
              status: 'error',
              startedAt: Date.now(),
              finishedAt: Date.now(),
            };
            return exists ? updateById(current, id, { ...next }) : appendCapped(current, next);
          });
          break;
        }
        case 'session.status': {
          const payload = (parsed.payload || {}) as { status?: string };
          if (payload.status === 'idle') {
            // Finalize any open thinking row and reset state.
            thinkingState.current = { activityId: null, buffer: '', inBlock: false };
          }
          break;
        }
        case 'part.delta': {
          const payload = (parsed.payload || {}) as { delta?: string };
          const delta = payload.delta || '';
          if (!delta) break;

          const state = thinkingState.current;
          // Open a thinking block if we see a marker.
          if (!state.inBlock && THINKING_LINE_RE.test(delta)) {
            state.inBlock = true;
            const activityId = newId();
            state.activityId = activityId;
            state.buffer = delta.replace(THINKING_LINE_RE, '').trim();
            setItems((current) =>
              appendCapped(current, {
                id: activityId,
                kind: 'thinking',
                label: 'Thinking',
                detail: state.buffer.slice(0, 200),
                status: 'pending',
                startedAt: Date.now(),
              }),
            );
            break;
          }
          // Inside a thinking block: accumulate detail and check for end marker.
          if (state.inBlock && state.activityId) {
            state.buffer += delta;
            if (THINKING_END_RE.test(delta)) {
              const finishedDetail = state.buffer.replace(THINKING_END_RE, '').trim().slice(0, 200);
              setItems((current) =>
                updateById(current, state.activityId!, {
                  detail: finishedDetail,
                  status: 'done',
                  finishedAt: Date.now(),
                }),
              );
              state.activityId = null;
              state.buffer = '';
              state.inBlock = false;
            } else {
              setItems((current) =>
                updateById(current, state.activityId!, {
                  detail: state.buffer.slice(-200),
                }),
              );
            }
          }
          break;
        }
        default:
          break;
      }
    }

    const eventTypes = ['tool.thinking', 'tool.started', 'tool.completed', 'tool.failed', 'session.status', 'part.delta'];
    eventTypes.forEach((type) => source.addEventListener(type, handle as EventListener));
    // Fallback for unnamed events.
    source.addEventListener('message', handle);

    return () => {
      eventTypes.forEach((type) => source.removeEventListener(type, handle as EventListener));
      source.removeEventListener('message', handle);
      source.close();
    };
  }, [sessionId, active]);

  return items;
}
