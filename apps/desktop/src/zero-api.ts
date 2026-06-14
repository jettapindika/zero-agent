const API_BASE = 'http://127.0.0.1:8910';

export type Part = {
  id: string;
  messageId: string;
  type: string;
  orderNum: number;
  text?: string | null;
  isError: boolean;
  createdAt: number;
};

export type Message = {
  id: string;
  sessionId: string;
  role: string;
  createdAt: number;
  parts: Part[];
};

export type Session = {
  id: string;
  projectId: string;
  title: string;
  model: string;
  agent: string;
  createdAt: number;
  updatedAt: number;
};

export type Project = {
  id: string;
  path: string;
  name: string;
  createdAt: number;
  updatedAt: number;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Zero API returned HTTP ${response.status}`);
  }

  if (response.status === 204 || response.headers.get('content-length') === '0') {
    return undefined as T;
  }

  const text = await response.text();
  if (text === '') {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}

export async function ensureProject(path: string, name: string): Promise<Project> {
  return request<Project>('/projects/ensure', {
    method: 'POST',
    body: JSON.stringify({ path, name }),
  });
}

export async function listSessions(projectId: string): Promise<Session[]> {
  return request<Session[]>(`/sessions?projectId=${encodeURIComponent(projectId)}`);
}

export async function createSession(input: Pick<Session, 'projectId' | 'title' | 'model' | 'agent'>): Promise<Session> {
  return request<Session>('/sessions', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export type SessionPatch = {
  title?: string;
  model?: string;
  agent?: string;
};

export async function updateSession(sessionId: string, patch: SessionPatch): Promise<Session> {
  return request<Session>(`/sessions/${encodeURIComponent(sessionId)}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function renameSession(sessionId: string, title: string): Promise<Session> {
  return updateSession(sessionId, { title });
}

export async function deleteSession(sessionId: string): Promise<void> {
  await request<void>(`/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' });
}

export async function listMessages(sessionId: string): Promise<Message[]> {
  return request<Message[]>(`/sessions/${encodeURIComponent(sessionId)}/messages`);
}

export async function createMessage(sessionId: string, role: string, text: string): Promise<void> {
  await request(`/sessions/${encodeURIComponent(sessionId)}/messages`, {
    method: 'POST',
    body: JSON.stringify({ role, text }),
  });
}

export async function runSession(sessionId: string): Promise<void> {
  await request<void>(`/sessions/${encodeURIComponent(sessionId)}/run`, { method: 'POST' });
}
