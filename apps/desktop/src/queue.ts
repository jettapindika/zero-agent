// useQueueRunner — sequential prompt queue per session, persisted in
// sessionStorage so a window resize/refresh does not lose pending prompts.

import { useCallback, useEffect, useRef, useState } from 'react';

export const MAX_QUEUE_LENGTH = 10;

export type QueueState = Record<string, string[]>;

const STORAGE_KEY = 'zero.queue.v1';

function loadAll(): QueueState {
  try {
    const raw = window.sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as QueueState;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function persistAll(state: QueueState) {
  try {
    window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    /* sessionStorage may be unavailable; ignore */
  }
}

export type QueueRunnerOptions = {
  sessionId: string | null;
  send: (text: string) => Promise<void>;
  busy: boolean;
};

export type QueueRunner = {
  queued: string[];
  enqueue: (text: string) => { ok: true } | { ok: false; reason: string };
  remove: (index: number) => void;
  clear: () => void;
  inFlight: string | null;
};

export function useQueueRunner({ sessionId, send, busy }: QueueRunnerOptions): QueueRunner {
  const [all, setAll] = useState<QueueState>(loadAll);
  const [inFlight, setInFlight] = useState<string | null>(null);
  const draining = useRef(false);

  // Persist whenever queue state changes.
  useEffect(() => {
    persistAll(all);
  }, [all]);

  const queued = sessionId ? all[sessionId] ?? [] : [];

  const enqueue = useCallback(
    (text: string): { ok: true } | { ok: false; reason: string } => {
      const trimmed = text.trim();
      if (!sessionId) return { ok: false, reason: 'No active session' };
      if (trimmed === '') return { ok: true }; // silent no-op per default
      const current = all[sessionId] ?? [];
      if (current.length >= MAX_QUEUE_LENGTH) {
        return { ok: false, reason: `Queue full (max ${MAX_QUEUE_LENGTH}). Wait for current to finish.` };
      }
      setAll((state) => ({ ...state, [sessionId]: [...(state[sessionId] ?? []), trimmed] }));
      return { ok: true };
    },
    [sessionId, all],
  );

  const remove = useCallback(
    (index: number) => {
      if (!sessionId) return;
      setAll((state) => {
        const current = state[sessionId] ?? [];
        if (index < 0 || index >= current.length) return state;
        const next = [...current.slice(0, index), ...current.slice(index + 1)];
        return { ...state, [sessionId]: next };
      });
    },
    [sessionId],
  );

  const clear = useCallback(() => {
    if (!sessionId) return;
    setAll((state) => ({ ...state, [sessionId]: [] }));
  }, [sessionId]);

  // Drain loop — when not busy and there is a head item, send it.
  useEffect(() => {
    if (!sessionId) return;
    if (busy) return;
    if (draining.current) return;
    const queue = all[sessionId] ?? [];
    if (queue.length === 0) return;

    const head = queue[0];
    draining.current = true;
    setInFlight(head);
    setAll((state) => ({ ...state, [sessionId]: (state[sessionId] ?? []).slice(1) }));

    void send(head).finally(() => {
      draining.current = false;
      setInFlight(null);
    });
  }, [all, busy, send, sessionId]);

  return { queued, enqueue, remove, clear, inFlight };
}
