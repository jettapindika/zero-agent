const API_BASE = 'http://127.0.0.1:8910';

export type Part = {
  id: string;
  messageId: string;
  type: string;
  orderNum: number;
  text?: string | null;
  toolName?: string | null;
  toolCallId?: string | null;
  toolArgsJson?: string | null;
  toolResultJson?: string | null;
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

// In-memory session token captured from the auth.signed_in SSE event.
// Stored alongside (not replacing) cookie credentials because the system
// browser sets cookies on its own jar; the Tauri webview can't read those.
// Sent as Authorization: Bearer <value> on every request — daemon accepts
// either carrier and verifies the same HMAC.
let sessionToken: string | null = null;
const TOKEN_STORAGE_KEY = 'zero.auth.sessionToken';

try {
  const cached = window.localStorage.getItem(TOKEN_STORAGE_KEY);
  if (cached) sessionToken = cached;
} catch {
  /* localStorage unavailable; fine */
}

export function setSessionToken(token: string | null) {
  sessionToken = token;
  try {
    if (token) window.localStorage.setItem(TOKEN_STORAGE_KEY, token);
    else window.localStorage.removeItem(TOKEN_STORAGE_KEY);
  } catch {
    /* ignore */
  }
}

export function getSessionToken(): string | null {
  return sessionToken;
}

async function tryRefreshToken(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/me`, { credentials: 'include' });
    if (!res.ok) return false;
    const data = await res.json();
    if (data.sessionToken) {
      setSessionToken(data.sessionToken);
      return true;
    }
    return false;
  } catch {
    return false;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> | undefined),
  };
  if (sessionToken) {
    headers['Authorization'] = `Bearer ${sessionToken}`;
  }
  let response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  });

  if (response.status === 401 && sessionToken) {
    const refreshed = await tryRefreshToken();
    if (refreshed) {
      headers['Authorization'] = `Bearer ${sessionToken}`;
      response = await fetch(`${API_BASE}${path}`, {
        ...init,
        credentials: 'include',
        headers,
      });
    }
  }

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

export async function cancelSession(sessionId: string): Promise<{ cancelled: boolean }> {
  return request<{ cancelled: boolean }>(`/sessions/${encodeURIComponent(sessionId)}/cancel`, { method: 'POST' });
}

export type ProviderModel = {
  id: string;
  name: string;
};

export async function listModels(): Promise<ProviderModel[]> {
  return request<ProviderModel[]>('/providers/models');
}

export type PermissionRequest = {
  id: string;
  sessionId: string;
  toolName: string;
  args: Record<string, unknown>;
  decision: string;
};

export async function listPermissions(sessionId: string): Promise<PermissionRequest[]> {
  return request<PermissionRequest[]>(`/sessions/${encodeURIComponent(sessionId)}/permissions`);
}

export async function resolvePermission(
  sessionId: string,
  permissionId: string,
  decision: 'allow_once' | 'always_allow' | 'deny',
): Promise<void> {
  await request<void>(
    `/sessions/${encodeURIComponent(sessionId)}/permissions/${encodeURIComponent(permissionId)}`,
    { method: 'POST', body: JSON.stringify({ decision }) },
  );
}

export type AuthUser = {
  id: string;
  googleId: string;
  email: string;
  displayName: string;
  avatarUrl: string;
  role: string;
  createdAt: number;
  updatedAt: number;
};

export type AuthMeResponse = {
  user: AuthUser;
  isDev: boolean;
  sessionId: string;
  sessionToken?: string;
  expiresAtMs: number;
};

export class AuthRequiredError extends Error {
  constructor() {
    super('authentication required');
    this.name = 'AuthRequiredError';
  }
}

// authMe is special-cased: a 401 is the normal "not signed in" state, not an
// error worth bubbling. Caller distinguishes via AuthRequiredError.
//
// When the daemon was launched without ZERO_AUTH_ENABLED, every /auth/* route
// returns 503 with {"error":"auth disabled"}. In that single-user mode there
// is no real account to fetch, so we synthesize a stable local identity and
// return it as if the user were signed in. App.tsx therefore renders the main
// shell without ever hitting LoginView.
export async function authMe(): Promise<AuthMeResponse> {
  const headers: Record<string, string> = {};
  if (sessionToken) headers['Authorization'] = `Bearer ${sessionToken}`;
  const response = await fetch(`${API_BASE}/auth/me`, { credentials: 'include', headers });
  if (response.status === 401 || response.status === 404) {
    throw new AuthRequiredError();
  }
  if (response.status === 503) {
    const body = await response.clone().json().catch(() => ({} as { error?: string }));
    if (body && (body as { error?: string }).error === 'auth disabled') {
      return localSingleUserAuth();
    }
  }
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `auth/me HTTP ${response.status}`);
  }
  const body = (await response.json()) as AuthMeResponse;
  if (body.sessionToken && body.sessionToken !== sessionToken) {
    setSessionToken(body.sessionToken);
  }
  return body;
}

function localSingleUserAuth(): AuthMeResponse {
  const now = Date.now();
  return {
    user: {
      id: 'local',
      googleId: '',
      email: 'local@zero',
      displayName: 'Local',
      avatarUrl: '',
      role: 'user',
      createdAt: now,
      updatedAt: now,
    },
    isDev: false,
    sessionId: 'local',
    expiresAtMs: now + 365 * 24 * 60 * 60 * 1000,
  };
}

export async function authLogout(): Promise<void> {
  const headers: Record<string, string> = {};
  if (sessionToken) headers['Authorization'] = `Bearer ${sessionToken}`;
  await fetch(`${API_BASE}/auth/logout`, { method: 'POST', credentials: 'include', headers });
  setSessionToken(null);
}

export const AUTH_START_URL = `${API_BASE}/auth/google/start`;
