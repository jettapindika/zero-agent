import { useCallback, useEffect, useState } from 'react';
import { getSessionToken } from './zero-api';

const API_BASE = 'http://127.0.0.1:8910';

export type ActivePrompt = {
  actorClientId: string;
  actorNickname: string;
  sessionId: string;
};

export type PendingInterruptRequest = {
  id: string;
  roomId: string;
  sessionId: string;
  requesterId: string;
  requesterNickname: string;
  ownerId: string;
  ownerNickname: string;
};

type Participant = {
  clientId: string;
  displayName: string;
};

export type ActivePromptState = {
  active: ActivePrompt | null;
  isSelf: boolean;
  interruptRequests: PendingInterruptRequest[];
  dismissRequest: (id: string) => void;
};

export function useActivePrompt(
  roomId: string | null,
  selfClientId: string | null,
  active: boolean,
): ActivePromptState {
  const [activePrompt, setActivePrompt] = useState<ActivePrompt | null>(null);
  const [participants, setParticipants] = useState<Map<string, string>>(new Map());
  const [interruptRequests, setInterruptRequests] = useState<PendingInterruptRequest[]>([]);

  useEffect(() => {
    if (!roomId || !active) {
      setActivePrompt(null);
      setParticipants(new Map());
      setInterruptRequests([]);
      return;
    }

    const token = getSessionToken();
    const headers: Record<string, string> = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;

    fetch(`${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/participants`, {
      credentials: 'include',
      headers,
    })
      .then((res) => (res.ok ? res.json() : []))
      .then((list: Participant[]) => {
        if (!Array.isArray(list)) return;
        const map = new Map<string, string>();
        for (const p of list) map.set(p.clientId, p.displayName);
        setParticipants(map);
      })
      .catch(() => {});
  }, [roomId, active]);

  useEffect(() => {
    if (!roomId || !active) return;

    const url = `${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/events`;
    const source = new EventSource(url, { withCredentials: true });

    function lookupNick(clientId: string): string {
      return participants.get(clientId) || clientId.slice(0, 8);
    }

    function handleStarted(event: MessageEvent) {
      try {
        const data = JSON.parse(event.data);
        const payload = data.payload || {};
        const actorId = payload.actorClientId || '';
        const sessionId = payload.sessionId || data.sessionId || '';
        if (!actorId) return;
        setActivePrompt({
          actorClientId: actorId,
          actorNickname: lookupNick(actorId),
          sessionId,
        });
      } catch {}
    }

    function handleCleared() {
      setActivePrompt(null);
    }

    function handleInterruptRequested(event: MessageEvent) {
      try {
        const data = JSON.parse(event.data);
        const payload = data.payload || {};
        if (payload.ownerId !== selfClientId) return;
        setInterruptRequests((prev) => [
          ...prev.filter((r) => r.id !== payload.id),
          {
            id: payload.id,
            roomId: payload.roomId,
            sessionId: payload.sessionId,
            requesterId: payload.requesterId,
            requesterNickname: payload.requesterNickname,
            ownerId: payload.ownerId,
            ownerNickname: payload.ownerNickname,
          },
        ]);
      } catch {}
    }

    function handleInterruptResolved(event: MessageEvent) {
      try {
        const data = JSON.parse(event.data);
        const payload = data.payload || {};
        setInterruptRequests((prev) => prev.filter((r) => r.id !== payload.id));
      } catch {}
    }

    const startedTypes = ['prompt.started'];
    const clearedTypes = [
      'prompt.completed',
      'prompt.cancelled',
      'prompt.failed',
      'prompt.interrupted',
    ];

    startedTypes.forEach((t) => source.addEventListener(t, handleStarted as EventListener));
    clearedTypes.forEach((t) => source.addEventListener(t, handleCleared as EventListener));
    source.addEventListener('collab.interrupt.requested', handleInterruptRequested as EventListener);
    source.addEventListener('collab.interrupt.approved', handleInterruptResolved as EventListener);
    source.addEventListener('collab.interrupt.rejected', handleInterruptResolved as EventListener);

    return () => {
      startedTypes.forEach((t) => source.removeEventListener(t, handleStarted as EventListener));
      clearedTypes.forEach((t) => source.removeEventListener(t, handleCleared as EventListener));
      source.removeEventListener('collab.interrupt.requested', handleInterruptRequested as EventListener);
      source.removeEventListener('collab.interrupt.approved', handleInterruptResolved as EventListener);
      source.removeEventListener('collab.interrupt.rejected', handleInterruptResolved as EventListener);
      source.close();
    };
  }, [roomId, active, participants, selfClientId]);

  const dismissRequest = useCallback((id: string) => {
    setInterruptRequests((prev) => prev.filter((r) => r.id !== id));
  }, []);

  const isSelf = activePrompt?.actorClientId === selfClientId;
  return { active: activePrompt, isSelf: !!isSelf, interruptRequests, dismissRequest };
}
