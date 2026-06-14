import { useCallback, useEffect, useRef, useState } from 'react';
import { getSessionToken } from './zero-api';

const API_BASE = 'http://127.0.0.1:8910';

export type ChatMessage = {
  id: string;
  roomId: string;
  fromId: string;
  nickname: string;
  role: 'host' | 'guest' | 'maintainer' | 'prompter' | 'viewer' | 'system';
  text: string;
  timestamp: string;
};

export type ChatState = {
  messages: ChatMessage[];
  unread: number;
  isOpen: boolean;
  openPanel: () => void;
  closePanel: () => void;
  sendMessage: (text: string) => void;
};

export function useCollabChat(roomId: string | null, selfId: string | null, active: boolean, displayName?: string): ChatState {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [unread, setUnread] = useState(0);
  const [isOpen, setIsOpen] = useState(false);
  const isOpenRef = useRef(isOpen);
  const selfIdRef = useRef(selfId);

  useEffect(() => {
    isOpenRef.current = isOpen;
  }, [isOpen]);

  useEffect(() => {
    selfIdRef.current = selfId;
  }, [selfId]);

  useEffect(() => {
    if (!roomId || !selfId || !active) {
      setMessages([]);
      setUnread(0);
      return;
    }

    const url = `${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/events`;
    const source = new EventSource(url, { withCredentials: true });

    function handleChatMessage(event: MessageEvent) {
      let parsed: ChatMessage;
      try {
        const data = JSON.parse(event.data);
        parsed = data.payload || data;
      } catch {
        return;
      }

      if (parsed.fromId === selfIdRef.current) return;

      setMessages((prev) => [...prev, parsed]);

      if (!isOpenRef.current) {
        setUnread((prev) => prev + 1);
      }
    }

    source.addEventListener('collab.chat.message', handleChatMessage as EventListener);

    const token = getSessionToken();
    const headers: Record<string, string> = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;

    fetch(`${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/chat`, {
      credentials: 'include',
      headers,
    })
      .then((res) => res.ok ? res.json() : [])
      .then((history: ChatMessage[]) => {
        if (Array.isArray(history)) {
          setMessages(history);
        }
      })
      .catch(() => {});

    return () => {
      source.removeEventListener('collab.chat.message', handleChatMessage as EventListener);
      source.close();
    };
  }, [roomId, selfId, active]);

  const sendMessage = useCallback(
    async (text: string) => {
      if (!roomId || !selfId || !text.trim()) return;

      const optimistic: ChatMessage = {
        id: `local_${Date.now()}`,
        roomId,
        fromId: selfId,
        nickname: displayName || 'You',
        role: 'host',
        text: text.trim(),
        timestamp: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, optimistic]);

      const token = getSessionToken();
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        'X-Zero-Client-ID': selfId,
      };
      if (token) headers['Authorization'] = `Bearer ${token}`;

      try {
        const res = await fetch(`${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/chat`, {
          method: 'POST',
          credentials: 'include',
          headers,
          body: JSON.stringify({ text: text.trim() }),
        });
        if (res.ok) {
          const saved: ChatMessage = await res.json();
          setMessages((prev) => prev.map((m) => m.id === optimistic.id ? saved : m));
        }
      } catch {}
    },
    [roomId, selfId],
  );

  const openPanel = useCallback(() => {
    setIsOpen(true);
    setUnread(0);
  }, []);

  const closePanel = useCallback(() => {
    setIsOpen(false);
  }, []);

  return { messages, unread, isOpen, openPanel, closePanel, sendMessage };
}
