import type { BusEvent, EnsureProjectResult, Message, Session } from "./api-types";

const API_BASE = process.env.NEXT_PUBLIC_ZERO_API_URL ?? "http://localhost:8910";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export async function ensureProject(path: string, name: string): Promise<EnsureProjectResult> {
  return request<EnsureProjectResult>("/projects/ensure", {
    method: "POST",
    body: JSON.stringify({ path, name }),
  });
}

export async function listSessions(projectId: string): Promise<Session[]> {
  return request<Session[]>(`/sessions?projectId=${encodeURIComponent(projectId)}`);
}

export async function createSession(input: Pick<Session, "projectId" | "title" | "model" | "agent">): Promise<Session> {
  return request<Session>("/sessions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function listMessages(sessionId: string): Promise<Message[]> {
  return request<Message[]>(`/sessions/${encodeURIComponent(sessionId)}/messages`);
}

export async function createMessage(sessionId: string, role: Message["role"], text: string): Promise<{ message: Omit<Message, "parts">; parts: Message["parts"] }> {
  return request(`/sessions/${encodeURIComponent(sessionId)}/messages`, {
    method: "POST",
    body: JSON.stringify({ role, text }),
  });
}

export async function runSession(sessionId: string): Promise<void> {
  await request<void>(`/sessions/${encodeURIComponent(sessionId)}/run`, { method: "POST" });
}

export function subscribeProjectEvents(projectId: string, onEvent: (event: BusEvent) => void): () => void {
  const source = new EventSource(`${API_BASE}/events?projectId=${encodeURIComponent(projectId)}`);
  source.onmessage = (message) => onEvent(JSON.parse(message.data) as BusEvent);
  const types = ["session.created", "session.updated", "session.deleted", "message.created", "message.updated", "part.delta", "tool.started", "tool.completed", "permission.required", "permission.resolved", "session.status"];
  for (const type of types) {
    source.addEventListener(type, (message) => onEvent(JSON.parse((message as MessageEvent).data) as BusEvent));
  }
  return () => source.close();
}
