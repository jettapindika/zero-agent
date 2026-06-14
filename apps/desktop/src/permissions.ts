// usePendingPermissions polls /sessions/{id}/permissions and exposes the
// current pending list plus a resolve helper. Hoisted out of SidePanel so
// the same data can drive (a) the SidePanel tab, (b) the in-chat permission
// cards rendered next to assistant messages, and (c) the tab badge — without
// each surface running its own poll loop.

import { useCallback, useEffect, useState } from 'react';
import { listPermissions, resolvePermission, type PermissionRequest } from './zero-api';

const POLL_MS = 1500;

export type PermissionDecision = 'allow_once' | 'always_allow' | 'deny';

export type ResolvedRecord = {
  id: string;
  decision: PermissionDecision;
  decidedAtMs: number;
};

export type UsePendingPermissions = {
  pending: PermissionRequest[];
  resolved: ResolvedRecord[];
  resolve: (id: string, decision: PermissionDecision) => Promise<void>;
};

export function usePendingPermissions(sessionId: string | null): UsePendingPermissions {
  const [pending, setPending] = useState<PermissionRequest[]>([]);
  const [resolved, setResolved] = useState<ResolvedRecord[]>([]);

  useEffect(() => {
    if (!sessionId) {
      setPending([]);
      return;
    }
    let cancelled = false;
    async function poll() {
      if (!sessionId) return;
      try {
        const list = await listPermissions(sessionId);
        if (cancelled) return;
        setPending(list);
      } catch {
        /* ignore — SSE / 401 transient */
      }
    }
    void poll();
    const id = window.setInterval(poll, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [sessionId]);

  const resolve = useCallback(
    async (id: string, decision: PermissionDecision) => {
      if (!sessionId) return;
      try {
        await resolvePermission(sessionId, id, decision);
      } catch {
        /* still optimistically update the UI */
      }
      setPending((current) => current.filter((r) => r.id !== id));
      setResolved((current) => [
        ...current.slice(-19), // keep last 20
        { id, decision, decidedAtMs: Date.now() },
      ]);
    },
    [sessionId],
  );

  return { pending, resolved, resolve };
}
