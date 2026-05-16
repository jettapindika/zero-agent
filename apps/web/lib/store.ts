import { create } from "zustand";
import type { BusEvent, Message, Part, Session } from "./api-types";

type AppState = {
  projectId: string | null;
  sessions: Session[];
  activeSessionId: string | null;
  messages: Message[];
  parts: Record<string, Part>;
  streaming: boolean;
  pendingPermissions: unknown[];
  selectedToolCallId: string | null;
  setProjectId: (projectId: string) => void;
  setSessions: (sessions: Session[]) => void;
  setActiveSessionId: (sessionId: string | null) => void;
  setMessages: (messages: Message[]) => void;
  applyEvent: (event: BusEvent) => void;
};

export const useAppStore = create<AppState>((set, get) => ({
  projectId: null,
  sessions: [],
  activeSessionId: null,
  messages: [],
  parts: {},
  streaming: false,
  pendingPermissions: [],
  selectedToolCallId: null,
  setProjectId: (projectId) => set({ projectId }),
  setSessions: (sessions) => set({ sessions }),
  setActiveSessionId: (activeSessionId) => set({ activeSessionId }),
  setMessages: (messages) => {
    const parts: Record<string, Part> = {};
    for (const message of messages) {
      for (const part of message.parts) {
        parts[part.id] = part;
      }
    }
    set({ messages, parts });
  },
  applyEvent: (event) => {
    if (event.type === "session.created") {
      const session = event.payload as Session;
      set({ sessions: [session, ...get().sessions] });
    }
    if (event.type === "session.updated") {
      const session = event.payload as Session;
      set({ sessions: get().sessions.map((item) => (item.id === session.id ? session : item)) });
    }
    if (event.type === "session.deleted") {
      const session = event.payload as Session;
      set({ sessions: get().sessions.filter((item) => item.id !== session.id) });
    }
    if (event.type === "message.created" && event.sessionId === get().activeSessionId) {
      const payload = event.payload as { message: Omit<Message, "parts">; parts: Part[] };
      set({ messages: [...get().messages, { ...payload.message, parts: payload.parts }] });
    }
    if (event.type === "part.delta" && event.sessionId === get().activeSessionId) {
      const payload = event.payload as { messageId: string; delta: string };
      set({
        streaming: true,
        messages: get().messages.map((message) => {
          if (message.id !== payload.messageId) return message;
          const streamPartId = `${message.id}:stream`;
          const existing = message.parts.find((part) => part.id === streamPartId);
          if (existing) {
            return { ...message, parts: message.parts.map((part) => part.id === streamPartId ? { ...part, text: `${part.text ?? ""}${payload.delta}` } : part) };
          }
          return { ...message, parts: [...message.parts, { id: streamPartId, messageId: message.id, type: "text", orderNum: message.parts.length, text: payload.delta, isError: false, createdAt: event.createdAt }] };
        }),
      });
    }
    if (event.type === "part.created" || event.type === "session.status") {
      set({ streaming: false });
    }
  },
}));
