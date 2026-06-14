import { getSessionToken } from './zero-api';

const API_BASE = 'http://127.0.0.1:8910';
const SHARE_CONFIG_KEY = 'zero:share-config';
const JOINED_ROOM_KEY = 'zero:joined-room';

export type ClientIdentity = {
  clientId: string;
  displayName: string;
};

export type CollabRoom = {
  id: string;
  projectId: string;
  hostClientId: string;
  name?: string;
  status: 'active' | 'revoked' | 'archived';
  defaultRole: 'host' | 'maintainer' | 'prompter' | 'viewer';
  promptReviewMode: 'off' | 'host_only' | 'maintainer_or_host' | 'all';
  allowMaintainerPromptIntercept: boolean;
  allowPromptEditBeforeApproval: boolean;
  requireHostApprovalDangerousTools: boolean;
  autoRunQueue: boolean;
  createdAt: number;
  updatedAt: number;
  revokedAt?: number;
};

export type CreateRoomResult = {
  room: CollabRoom;
  inviteToken: string;
};

export type CollabParticipant = {
  id?: string;
  roomId?: string;
  clientId?: string;
  displayName: string;
  role: 'host' | 'maintainer' | 'prompter' | 'viewer' | string;
  status: 'online' | 'offline' | 'kicked' | string;
};

export type JoinRoomResult = {
  room: CollabRoom;
  participant: CollabParticipant;
};

export type JoinedRoomConfig = {
  sessionId: string;
  roomName?: string;
  role: string;
  displayName: string;
  hostClientId: string;
  joinedAt: string;
};

export type ShareConfig = {
  sessionId: string;
  folderPath: string;
  subfolders?: string[];
  tokenMode: 'host' | 'guest' | 'choice';
  rateLimit?: number;
  permissions: {
    readFiles: boolean;
    syncFiles: boolean;
    writeFiles: boolean;
    runAgent: boolean;
    viewChatHistory: boolean;
  };
  requireApproval: boolean;
  inviteCode: string;
  inviteUrl: string;
  hostId: string;
  createdAt: string;
};

export type ShareConfigDraft = Omit<
  ShareConfig,
  'sessionId' | 'inviteCode' | 'inviteUrl' | 'hostId' | 'createdAt'
>;

async function request<T>(path: string, init: RequestInit & { clientId: string }): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Zero-Client-ID': init.clientId,
    ...(init.headers as Record<string, string> | undefined),
  };
  const token = getSessionToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const { clientId: _ignored, ...rest } = init;
  const response = await fetch(`${API_BASE}${path}`, { ...rest, credentials: 'include', headers });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Collab API HTTP ${response.status}`);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export async function getIdentity(): Promise<ClientIdentity> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getSessionToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const response = await fetch(`${API_BASE}/identity`, { credentials: 'include', headers });
  if (!response.ok) {
    throw new Error(`identity HTTP ${response.status}`);
  }
  return (await response.json()) as ClientIdentity;
}

export function roleFromPermissions(p: ShareConfig['permissions']): CollabRoom['defaultRole'] {
  if (p.runAgent && p.writeFiles) return 'maintainer';
  if (p.writeFiles) return 'prompter';
  return 'viewer';
}

export async function createRoom(
  identity: ClientIdentity,
  projectId: string,
  draft: ShareConfigDraft,
): Promise<CreateRoomResult> {
  return request<CreateRoomResult>('/collab/rooms', {
    method: 'POST',
    clientId: identity.clientId,
    body: JSON.stringify({
      projectId,
      name: draft.folderPath.split('/').pop() ?? '',
      defaultRole: roleFromPermissions(draft.permissions),
      promptReviewMode: draft.requireApproval ? 'host_only' : 'off',
      autoRunQueue: !draft.requireApproval,
    }),
  });
}

export async function joinRoom(
  identity: ClientIdentity,
  roomId: string,
  token: string,
  displayName?: string,
): Promise<JoinRoomResult> {
  return request<JoinRoomResult>(`/collab/rooms/${encodeURIComponent(roomId)}/join`, {
    method: 'POST',
    clientId: identity.clientId,
    body: JSON.stringify({
      token,
      displayName: displayName?.trim() || identity.displayName || 'Guest',
    }),
  });
}

// Accepts: `zero://join/<roomId>?token=<token>`, `https?://.../join/<roomId>?token=<token>`,
// or a bare room ID (caller must supply token separately).
export function parseInvite(input: string): { roomId: string; token: string } {
  const trimmed = (input || '').trim();
  if (!trimmed) return { roomId: '', token: '' };

  // The URL constructor's pathname behavior on custom schemes is inconsistent
  // across runtimes, so split the zero:// form by hand.
  const zeroPrefix = 'zero://join/';
  if (trimmed.toLowerCase().startsWith(zeroPrefix)) {
    const rest = trimmed.slice(zeroPrefix.length);
    const qIdx = rest.indexOf('?');
    const roomId = qIdx === -1 ? rest : rest.slice(0, qIdx);
    const query = qIdx === -1 ? '' : rest.slice(qIdx + 1);
    const params = new URLSearchParams(query);
    return { roomId: roomId.replace(/\/+$/, ''), token: params.get('token') ?? '' };
  }

  if (/^https?:\/\//i.test(trimmed)) {
    try {
      const u = new URL(trimmed);
      const parts = u.pathname.split('/').filter(Boolean);
      const joinIdx = parts.indexOf('join');
      const roomId = joinIdx >= 0 && parts[joinIdx + 1] ? parts[joinIdx + 1] : '';
      return { roomId, token: u.searchParams.get('token') ?? '' };
    } catch {
      return { roomId: '', token: '' };
    }
  }

  return { roomId: trimmed, token: '' };
}

export async function revokeRoom(identity: ClientIdentity, roomId: string): Promise<void> {
  await request<void>(`/collab/rooms/${encodeURIComponent(roomId)}/revoke`, {
    method: 'POST',
    clientId: identity.clientId,
  });
}

export type InterruptResult = {
  cancelled: boolean;
  interruptedActor?: string;
  interruptedNickname?: string;
};

export async function interruptPrompt(
  identity: ClientIdentity,
  roomId: string,
  sessionId: string,
): Promise<InterruptResult> {
  return request<InterruptResult>(
    `/collab/rooms/${encodeURIComponent(roomId)}/sessions/${encodeURIComponent(sessionId)}/interrupt`,
    {
      method: 'POST',
      clientId: identity.clientId,
    },
  );
}

export async function resolveInterrupt(
  identity: ClientIdentity,
  roomId: string,
  requestId: string,
  approve: boolean,
): Promise<InterruptResult> {
  return request<InterruptResult>(
    `/collab/rooms/${encodeURIComponent(roomId)}/interrupt-requests/${encodeURIComponent(requestId)}/resolve`,
    {
      method: 'POST',
      clientId: identity.clientId,
      body: JSON.stringify({ approve }),
    },
  );
}

export function roomEventsUrl(roomId: string): string {
  return `${API_BASE}/collab/rooms/${encodeURIComponent(roomId)}/events`;
}

export function inviteUrlFor(roomId: string, token: string): string {
  return `zero://join/${roomId}?token=${token}`;
}

// Derives the spec's XXXX-XXXX-XXXX human code from the daemon's hex token.
// The full token (not this short code) is what the daemon validates on join;
// the short code exists purely as a human-scannable fingerprint.
export function inviteCodeFor(token: string): string {
  const upper = token.replace(/[^a-fA-F0-9]/g, '').toUpperCase().slice(0, 12).padEnd(12, '0');
  return `${upper.slice(0, 4)}-${upper.slice(4, 8)}-${upper.slice(8, 12)}`;
}

export function saveShareConfig(cfg: ShareConfig) {
  try {
    window.localStorage.setItem(SHARE_CONFIG_KEY, JSON.stringify(cfg));
  } catch {}
}

export function loadShareConfig(): ShareConfig | null {
  try {
    const raw = window.localStorage.getItem(SHARE_CONFIG_KEY);
    return raw ? (JSON.parse(raw) as ShareConfig) : null;
  } catch {
    return null;
  }
}

export function clearShareConfig() {
  try {
    window.localStorage.removeItem(SHARE_CONFIG_KEY);
  } catch {}
}

export function saveJoinedRoom(cfg: JoinedRoomConfig) {
  try {
    window.localStorage.setItem(JOINED_ROOM_KEY, JSON.stringify(cfg));
  } catch {}
}

export function loadJoinedRoom(): JoinedRoomConfig | null {
  try {
    const raw = window.localStorage.getItem(JOINED_ROOM_KEY);
    return raw ? (JSON.parse(raw) as JoinedRoomConfig) : null;
  } catch {
    return null;
  }
}

export function clearJoinedRoom() {
  try {
    window.localStorage.removeItem(JOINED_ROOM_KEY);
  } catch {}
}
