import { useState } from 'react';
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  FolderOpen,
  Loader2,
  LogIn,
  Share2,
  UserPlus,
  X,
} from 'lucide-react';
import {
  createRoom,
  getIdentity,
  inviteCodeFor,
  inviteUrlFor,
  joinRoom,
  parseInvite,
  saveJoinedRoom,
  saveShareConfig,
  type JoinedRoomConfig,
  type ShareConfig,
  type ShareConfigDraft,
} from '../collab';

type Step = 0 | 1 | 2 | 3 | 4;
type Mode = 'create' | 'join';

type Props = {
  folderPath: string;
  projectId: string;
  onClose: () => void;
  onShared: (cfg: ShareConfig) => void;
  onJoined?: (cfg: JoinedRoomConfig) => void;
  onSignIn: () => void;
  initialMode?: Mode;
  initialInvite?: string;
};

const DEFAULT_DRAFT: ShareConfigDraft = {
  folderPath: '',
  tokenMode: 'host',
  rateLimit: 10,
  permissions: {
    readFiles: true,
    syncFiles: true,
    writeFiles: false,
    runAgent: false,
    viewChatHistory: false,
  },
  requireApproval: true,
};

export function ShareModal({
  folderPath,
  projectId,
  onClose,
  onShared,
  onJoined,
  onSignIn,
  initialMode,
  initialInvite,
}: Props) {
  const [step, setStep] = useState<Step>(0);
  const [mode, setMode] = useState<Mode | null>(initialMode ?? null);
  const [draft, setDraft] = useState<ShareConfigDraft>({ ...DEFAULT_DRAFT, folderPath });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authError, setAuthError] = useState(false);
  const [joinRoomId, setJoinRoomId] = useState(initialInvite ?? '');
  const [joinToken, setJoinToken] = useState('');
  const [joinDisplayName, setJoinDisplayName] = useState('');
  const [joined, setJoined] = useState<JoinedRoomConfig | null>(null);

  const parsed = parseInvite(joinRoomId);
  const effectiveRoomId = parsed.roomId;
  const effectiveToken = parsed.token || joinToken.trim();
  const inviteIsUrl = parsed.token !== '';
  const canJoin = !!effectiveRoomId && !!effectiveToken;

  const next = () => setStep((s) => (s < 3 ? ((s + 1) as Step) : s));
  const back = () => setStep((s) => (s > 0 ? ((s - 1) as Step) : s));

  function isAuthError(err: unknown): boolean {
    const msg = err instanceof Error ? err.message : String(err);
    return msg.includes('401') || msg.includes('Unauthorized') || msg.includes('authentication required');
  }

  function describeJoinError(err: unknown): string {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes('404') || msg.toLowerCase().includes('not found')) {
      return 'Room not found. Double-check the room ID with the host.';
    }
    if (msg.includes('410') || msg.toLowerCase().includes('revoked')) {
      return 'This room has been revoked by the host.';
    }
    if (msg.includes('401') || msg.toLowerCase().includes('invalid token')) {
      return 'Invalid invite token. Ask the host for a fresh invite.';
    }
    return msg;
  }

  async function startCreate() {
    const identity = await getIdentity();
    const result = await createRoom(identity, projectId, draft);
    const cfg: ShareConfig = {
      ...draft,
      sessionId: result.room.id,
      inviteCode: inviteCodeFor(result.inviteToken),
      inviteUrl: inviteUrlFor(result.room.id, result.inviteToken),
      hostId: identity.clientId,
      createdAt: new Date().toISOString(),
    };
    saveShareConfig(cfg);
    onShared(cfg);
  }

  async function startJoin() {
    if (!canJoin) {
      throw new Error('Enter a room ID and invite token (or paste the full invite URL).');
    }
    const identity = await getIdentity();
    const displayName = joinDisplayName.trim() || identity.displayName || 'Guest';
    const guestClientId = `guest_${crypto.randomUUID().slice(0, 8)}`;
    const guestIdentity = { ...identity, clientId: guestClientId };
    const result = await joinRoom(guestIdentity, effectiveRoomId, effectiveToken, displayName);
    const cfg: JoinedRoomConfig = {
      sessionId: result.room.id,
      roomName: result.room.name,
      role: result.participant.role,
      displayName: result.participant.displayName,
      guestClientId,
      hostClientId: result.room.hostClientId,
      joinedAt: new Date().toISOString(),
    };
    saveJoinedRoom(cfg);
    setJoined(cfg);
    onJoined?.(cfg);
  }

  async function start() {
    setBusy(true);
    setError(null);
    setAuthError(false);
    if (mode === 'join') setStep(4);
    try {
      if (mode === 'join') {
        await startJoin();
      } else {
        await startCreate();
      }
    } catch (e) {
      if (isAuthError(e)) {
        setAuthError(true);
        setError(
          mode === 'join'
            ? 'Your session has expired. Please sign in again to join the room.'
            : 'Your session has expired or is invalid. Please sign in again to share your project.',
        );
      } else if (mode === 'join') {
        setError(describeJoinError(e));
      } else {
        setError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setBusy(false);
    }
  }

  function retryJoin() {
    setError(null);
    setAuthError(false);
    setStep(0);
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Share folder">
      <div className="modal-card share-modal">
        <div className="modal-header">
          <h2>
            <Share2 size={16} />
            {step === 0
              ? 'Collaboration'
              : step === 4
                ? 'Joining room'
                : 'Share folder'}
          </h2>
          <button className="modal-close" type="button" onClick={onClose} aria-label="Close">
            <X size={16} />
          </button>
        </div>

        {step > 0 && step < 4 && (
          <div className="share-stepper" aria-label={`Step ${step} of 3`}>
            {[1, 2, 3].map((n) => (
              <div
                key={n}
                className={`share-stepper-seg${n < step ? ' done' : n === step ? ' active' : ''}`}
              />
            ))}
          </div>
        )}

        {step === 0 && (
          <Step0
            mode={mode}
            onModeChange={setMode}
            joinRoomId={joinRoomId}
            onJoinRoomIdChange={setJoinRoomId}
            joinToken={joinToken}
            onJoinTokenChange={setJoinToken}
            joinDisplayName={joinDisplayName}
            onJoinDisplayNameChange={setJoinDisplayName}
            inviteIsUrl={inviteIsUrl}
            parsedRoomId={effectiveRoomId}
          />
        )}
        {step === 1 && <Step1 folderPath={folderPath} draft={draft} onChange={setDraft} />}
        {step === 2 && <Step2 draft={draft} onChange={setDraft} />}
        {step === 3 && <Step3 draft={draft} onChange={setDraft} />}
        {step === 4 && (
          <Step4Status
            busy={busy}
            error={error}
            joined={joined}
            roomId={effectiveRoomId}
          />
        )}

        {error && step !== 4 ? (
          <div className="share-error">
            <p>{error}</p>
            {authError ? (
              <button type="button" className="ghost" onClick={onSignIn}>
                <LogIn size={14} /> Sign in again
              </button>
            ) : null}
          </div>
        ) : null}

        <div className="share-actions">
          {step === 4 ? (
            <>
              {error ? (
                <button type="button" className="ghost" onClick={retryJoin} disabled={busy}>
                  <ChevronLeft size={14} /> Try again
                </button>
              ) : (
                <span />
              )}
              {error && authError ? (
                <button type="button" className="primary" onClick={onSignIn}>
                  <LogIn size={14} /> Sign in again
                </button>
              ) : (
                <button
                  type="button"
                  className="primary"
                  onClick={onClose}
                  disabled={busy}
                >
                  {busy ? <Loader2 size={14} className="spin" /> : null}
                  {busy ? 'Connecting…' : joined ? 'Done' : 'Close'}
                </button>
              )}
            </>
          ) : (
            <>
              {step > 0 && step < 4 ? (
                <button type="button" className="ghost" onClick={back} disabled={busy}>
                  <ChevronLeft size={14} /> Back
                </button>
              ) : (
                <span />
              )}
              {step === 0 ? (
                mode === 'join' ? (
                  <button
                    type="button"
                    className="primary"
                    onClick={start}
                    disabled={busy || !canJoin}
                  >
                    {busy ? <Loader2 size={14} className="spin" /> : <UserPlus size={14} />}
                    {busy ? 'Joining…' : 'Join room'}
                  </button>
                ) : (
                  <button
                    type="button"
                    className="primary"
                    onClick={next}
                    disabled={busy || !mode}
                  >
                    Next <ChevronRight size={14} />
                  </button>
                )
              ) : step < 3 ? (
                <button type="button" className="primary" onClick={next} disabled={busy}>
                  Next <ChevronRight size={14} />
                </button>
              ) : (
                <button type="button" className="primary" onClick={start} disabled={busy}>
                  {busy ? <Loader2 size={14} className="spin" /> : <Share2 size={14} />}
                  {busy ? 'Starting…' : 'Start sharing'}
                </button>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function Step0({
  mode,
  onModeChange,
  joinRoomId,
  onJoinRoomIdChange,
  joinToken,
  onJoinTokenChange,
  joinDisplayName,
  onJoinDisplayNameChange,
  inviteIsUrl,
  parsedRoomId,
}: {
  mode: Mode | null;
  onModeChange: (m: Mode) => void;
  joinRoomId: string;
  onJoinRoomIdChange: (id: string) => void;
  joinToken: string;
  onJoinTokenChange: (t: string) => void;
  joinDisplayName: string;
  onJoinDisplayNameChange: (n: string) => void;
  inviteIsUrl: boolean;
  parsedRoomId: string;
}) {
  return (
    <div className="share-step">
      <p className="share-label">What do you want to do?</p>

      <button
        type="button"
        className={`share-mode-button ${mode === 'create' ? 'selected' : ''}`}
        onClick={() => onModeChange('create')}
      >
        <Share2 size={20} />
        <div className="share-mode-content">
          <strong>Create a room</strong>
          <small>Share your project with others and manage permissions</small>
        </div>
      </button>

      <button
        type="button"
        className={`share-mode-button ${mode === 'join' ? 'selected' : ''}`}
        onClick={() => onModeChange('join')}
      >
        <UserPlus size={20} />
        <div className="share-mode-content">
          <strong>Join a room</strong>
          <small>Connect to an existing shared project</small>
        </div>
      </button>

      {mode === 'join' && (
        <div className="share-join-form">
          <div className="share-group">
            <label className="share-label">Invite URL or room ID</label>
            <input
              type="text"
              className="share-input"
              placeholder="zero://join/<roomId>?token=… or roomId"
              value={joinRoomId}
              onChange={(e) => onJoinRoomIdChange(e.target.value)}
              autoFocus
            />
            {inviteIsUrl ? (
              <p className="share-hint">
                Detected room <code>{parsedRoomId}</code> — token will be read from the URL.
              </p>
            ) : (
              <>
                <label className="share-label share-label-spaced">
                  Invite token
                </label>
                <input
                  type="text"
                  className="share-input"
                  placeholder="Hex token from the host"
                  value={joinToken}
                  onChange={(e) => onJoinTokenChange(e.target.value)}
                />
                <p className="share-hint">
                  Ask the host for the room ID and invite token, or paste the full
                  <code> zero://join/…?token=…</code> URL above.
                </p>
              </>
            )}
          </div>

          <div className="share-group">
            <label className="share-label">Your display name (optional)</label>
            <input
              type="text"
              className="share-input"
              placeholder="How others see you in chat"
              value={joinDisplayName}
              onChange={(e) => onJoinDisplayNameChange(e.target.value)}
            />
          </div>
        </div>
      )}
    </div>
  );
}

function Step4Status({
  busy,
  error,
  joined,
  roomId,
}: {
  busy: boolean;
  error: string | null;
  joined: JoinedRoomConfig | null;
  roomId: string;
}) {
  if (busy) {
    return (
      <div className="share-step share-status">
        <Loader2 size={28} className="spin" />
        <p className="share-status-title">Connecting to room…</p>
        <p className="share-hint">
          Verifying invite token for <code>{roomId}</code>.
        </p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="share-step share-status share-status-error">
        <X size={28} />
        <p className="share-status-title">Couldn’t join the room</p>
        <p className="share-error-text">{error}</p>
      </div>
    );
  }

  if (joined) {
    return (
      <div className="share-step share-status share-status-success">
        <CheckCircle2 size={28} />
        <p className="share-status-title">You’re in</p>
        <dl className="share-status-meta">
          <div>
            <dt>Room</dt>
            <dd><code>{joined.sessionId}</code></dd>
          </div>
          {joined.roomName ? (
            <div>
              <dt>Name</dt>
              <dd>{joined.roomName}</dd>
            </div>
          ) : null}
          <div>
            <dt>Role</dt>
            <dd>{joined.role}</dd>
          </div>
          <div>
            <dt>Display name</dt>
            <dd>{joined.displayName}</dd>
          </div>
        </dl>
        <p className="share-hint">Open the chat sidebar to talk with the host and other guests.</p>
      </div>
    );
  }

  return null;
}

function Step1({
  folderPath,
  draft,
  onChange,
}: {
  folderPath: string;
  draft: ShareConfigDraft;
  onChange: (d: ShareConfigDraft) => void;
}) {
  const entire = !draft.subfolders;
  return (
    <div className="share-step">
      <div className="share-step-intro">
        <p className="share-step-title">Folder scope</p>
        <p className="share-step-subtitle">Choose what to share with collaborators</p>
      </div>

      <div className="share-group">
        <p className="share-label">Project folder</p>
        <div className="share-folder">
          <FolderOpen size={14} /> <span>{folderPath || '(no folder selected)'}</span>
        </div>
      </div>

      <div className="share-group">
        <p className="share-label">Scope</p>
        <label className="share-radio">
          <input
            type="radio"
            checked={entire}
            onChange={() => onChange({ ...draft, subfolders: undefined })}
          />
          <span>Share entire folder</span>
        </label>
        <label className="share-radio">
          <input
            type="radio"
            checked={!entire}
            onChange={() => onChange({ ...draft, subfolders: draft.subfolders ?? [] })}
          />
          <span>Share specific subfolders only</span>
        </label>
        {!entire ? (
          <p className="share-hint">
            Subfolder selection isn't wired yet — guests will see the entire folder
            for now. Coming soon.
          </p>
        ) : null}
      </div>
    </div>
  );
}

function Step2({
  draft,
  onChange,
}: {
  draft: ShareConfigDraft;
  onChange: (d: ShareConfigDraft) => void;
}) {
  return (
    <div className="share-step">
      <div className="share-step-intro">
        <p className="share-step-title">AI access</p>
        <p className="share-step-subtitle">How should guests interact with the AI agent?</p>
      </div>

      <div className="share-group">
        <label className="share-radio share-radio-block">
          <input
            type="radio"
            checked={draft.tokenMode === 'host'}
            onChange={() => onChange({ ...draft, tokenMode: 'host' })}
          />
          <div>
            <strong>Guests use my token & endpoint</strong>
            <small>Guests can send prompts — billed to you.</small>
            {draft.tokenMode === 'host' ? (
              <div className="share-rate">
                Rate limit per guest:&nbsp;
                <input
                  type="number"
                  min={1}
                  max={1000}
                  value={draft.rateLimit ?? 10}
                  onChange={(e) => onChange({ ...draft, rateLimit: Number(e.target.value) || 10 })}
                />
                &nbsp;requests / min
              </div>
            ) : null}
          </div>
        </label>

        <label className="share-radio share-radio-block">
          <input
            type="radio"
            checked={draft.tokenMode === 'guest'}
            onChange={() => onChange({ ...draft, tokenMode: 'guest' })}
          />
          <div>
            <strong>Guests bring their own token & endpoint</strong>
            <small>Guests configure their own AI access. You are not billed.</small>
          </div>
        </label>

        <label className="share-radio share-radio-block">
          <input
            type="radio"
            checked={draft.tokenMode === 'choice'}
            onChange={() => onChange({ ...draft, tokenMode: 'choice' })}
          />
          <div>
            <strong>Let each guest choose</strong>
            <small>Guest decides on join — either option allowed.</small>
          </div>
        </label>
      </div>

      <p className="share-hint">
        Token-mode enforcement is part of the AI proxy work that ships next.
        Your choice is saved with the share so guests see it on join.
      </p>
    </div>
  );
}

function Step3({
  draft,
  onChange,
}: {
  draft: ShareConfigDraft;
  onChange: (d: ShareConfigDraft) => void;
}) {
  const p = draft.permissions;
  const setP = (next: Partial<typeof p>) =>
    onChange({ ...draft, permissions: { ...p, ...next } });

  return (
    <div className="share-step">
      <div className="share-step-intro">
        <p className="share-step-title">Permissions</p>
        <p className="share-step-subtitle">Control what guests can do in your project</p>
      </div>

      <div className="share-group">
        <p className="share-label">File access</p>
        <label className="share-check">
          <input type="checkbox" checked={p.readFiles} onChange={(e) => setP({ readFiles: e.target.checked })} />
          <span>Read files</span>
        </label>
        <label className="share-check">
          <input type="checkbox" checked={p.syncFiles} onChange={(e) => setP({ syncFiles: e.target.checked })} />
          <span>Sync file changes in real time</span>
        </label>
        <label className="share-check">
          <input type="checkbox" checked={p.writeFiles} onChange={(e) => setP({ writeFiles: e.target.checked })} />
          <span>Write files (guests can edit)</span>
        </label>
      </div>

      <div className="share-group">
        <p className="share-label">Agent & chat</p>
        <label className="share-check">
          <input type="checkbox" checked={p.runAgent} onChange={(e) => setP({ runAgent: e.target.checked })} />
          <span>Run agent (guests can trigger agent tasks)</span>
        </label>
        <label className="share-check">
          <input
            type="checkbox"
            checked={p.viewChatHistory}
            onChange={(e) => setP({ viewChatHistory: e.target.checked })}
          />
          <span>View chat history</span>
        </label>
      </div>

      <div className="share-group">
        <p className="share-label">Room access</p>
        <label className="share-check">
          <input
            type="checkbox"
            checked={draft.requireApproval}
            onChange={(e) => onChange({ ...draft, requireApproval: e.target.checked })}
          />
          <span>Require approval before guest joins</span>
        </label>
      </div>
    </div>
  );
}
