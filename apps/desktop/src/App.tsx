import { Bot, Check, ChevronDown, CircleAlert, CircleCheck, Copy, File as FileIcon, FileSpreadsheet, FileText, Film, FolderOpen, Headphones, Image as ImageIcon, Loader2, Paperclip, Pencil, Power, Presentation, Send, Share2, Trash2, X } from 'lucide-react';
import { FormEvent, KeyboardEvent, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityPanel } from './components/ActivityPanel';
import { CollabBar } from './components/CollabBar';
import { CollabChatBar } from './components/CollabChatBar';
import { PromptInterruptWarning } from './components/PromptInterruptWarning';
import { InterruptRequestCard } from './components/InterruptRequestCard';
import { FilesList } from './components/FilesList';
import { LoginView } from './components/LoginView';
import { ModelPickerModal } from './components/ModelPickerModal';
import { PermissionCard } from './components/PermissionCard';
import { ShareModal } from './components/ShareModal';
import { SidePanel } from './components/SidePanel';
import { SlashPreview } from './components/SlashPreview';
import { PlayfulGreeting } from './components/PlayfulGreeting';
import { ToolPart } from './components/ToolPart';
import { TypingIndicator } from './components/TypingIndicator';
import { UserChip } from './components/UserChip';
import {
  loadShareConfig,
  loadJoinedRoom,
  clearJoinedRoom,
  requestSession,
  type ShareConfig,
  type JoinedRoomConfig,
  getIdentity,
  type ClientIdentity,
} from './collab';
import { MessageBody, shouldUseStructuredRenderer } from './chat/MessageBody';
import { useActivityStream } from './activity';
import { useActivePrompt } from './useActivePrompt';
import { usePendingPermissions } from './permissions';
import { useCurrentUser } from './auth';
import { useQueueRunner } from './queue';
import { parseSlashCommand, validateModelId, SLASH_HELP_TEXT } from './slash';
import { extractTasks } from './tasks';
import { extractTouchedFiles } from './files';
import { type DroppedFile, type DroppedKind } from './dropped-files';
import { useFileDrop } from './use-file-drop';
import { desktop, type StatusResponse } from './tauri';
import { cancelSession, createMessage, createSession, deleteSession, ensureProject, getSessionToken, listMessages, listSessions, renameSession, runSession, updateSession, type AuthUser, type Message, type Part, type Project, type Session } from './zero-api';

const DEFAULT_MODEL = 'cx/gpt-5.5';
const DEFAULT_AGENT = 'build';

function initialProjectPath() {
  return window.localStorage.getItem('zero.projectPath') || '';
}

type AttachmentEntry = {
  path: string;
  name: string;
  kind: DroppedKind;
  status: 'uploading' | 'done' | 'error';
  attachmentId?: string;
  error?: string;
};

function attachmentEntryFromDropped(file: DroppedFile): AttachmentEntry {
  return { path: file.path, name: file.name, kind: file.kind, status: 'uploading' };
}

// folderDisplayName returns just the basename of an absolute path so the
// sidebar shows the project name instead of the full filesystem path.
// Trailing slashes are tolerated. Empty input becomes a placeholder.
function folderDisplayName(path: string): string {
  const trimmed = path.trim().replace(/[/\\]+$/, '');
  if (trimmed === '') return 'No folder selected';
  const sep = trimmed.lastIndexOf('/') >= 0 ? '/' : '\\';
  const idx = trimmed.lastIndexOf(sep);
  return idx >= 0 && idx < trimmed.length - 1 ? trimmed.slice(idx + 1) : trimmed;
}

// App is the public entry. It gates on auth state: shows a centered LoginView
// when the daemon says we're signed out, a skeleton while we're checking, and
// the full shell once signed in. AppShell holds the existing chat UI; nothing
// in there changes its single-user mental model.
export function App() {
  const auth = useCurrentUser();

  if (auth.state.status === 'loading') {
    return (
      <div className="login-shell">
        <main className="login-card">
          <p className="login-sub">Connecting to local daemon…</p>
        </main>
      </div>
    );
  }

  if (auth.state.status === 'signed_out') {
    return <LoginView onSignIn={auth.signIn} reason={auth.state.reason} />;
  }

  return (
    <AppShell
      currentUser={auth.state.user}
      isDev={auth.state.isDev}
      onSignOut={auth.signOut}
      onSignIn={auth.signIn}
    />
  );
}

type AppShellProps = {
  currentUser: AuthUser;
  isDev: boolean;
  onSignOut: () => Promise<void>;
  onSignIn: () => void;
};

function AppShell({ currentUser, isDev, onSignOut, onSignIn }: AppShellProps) {
  const [server, setServer] = useState<StatusResponse>({ ok: false, status: 'checking', detail: 'Checking zero-server...' });
  const [provider, setProvider] = useState<StatusResponse>({ ok: false, status: 'checking', detail: 'Checking provider...' });
  const [project, setProject] = useState<Project | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [localNotices, setLocalNotices] = useState<{ id: string; level: 'info' | 'error'; text: string }[]>([]);
  const [prompt, setPrompt] = useState('');
  const [attachments, setAttachments] = useState<AttachmentEntry[]>([]);
  const [projectPath, setProjectPath] = useState(initialProjectPath);
  const [busy, setBusy] = useState(false);
  const [sending, setSending] = useState(false);
  const [sendingStartedAt, setSendingStartedAt] = useState<number>(0);
  const [error, setError] = useState<string | null>(null);
  const [queueWarning, setQueueWarning] = useState<string | null>(null);
  const [modelPickerOpen, setModelPickerOpen] = useState(false);
  const [shareModalOpen, setShareModalOpen] = useState(false);
  const [shareConfig, setShareConfig] = useState<ShareConfig | null>(() => loadShareConfig());
  const [joinedRoom, setJoinedRoom] = useState<JoinedRoomConfig | null>(() => loadJoinedRoom());
  const [pendingJoinInvite, setPendingJoinInvite] = useState<string | null>(null);
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameDraft, setRenameDraft] = useState('');
  const [identity, setIdentity] = useState<ClientIdentity | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const messagesContainerRef = useRef<HTMLDivElement | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId) ?? sessions[0] ?? null,
    [activeSessionId, sessions],
  );

  const collabRoomId = shareConfig?.sessionId ?? joinedRoom?.sessionId ?? null;
  const isCollabActive = !!shareConfig || !!joinedRoom;
  const isCollabHost = !!shareConfig;
  const collabSessionId = activeSession?.id ?? null;
  const collabSelfId = joinedRoom?.guestClientId ?? identity?.clientId ?? null;

  const activePromptState = useActivePrompt(
    collabRoomId,
    collabSelfId,
    isCollabActive,
  );
  const showInterruptWarning =
    isCollabActive &&
    !!activePromptState.active &&
    !activePromptState.isSelf &&
    !!collabRoomId &&
    !!activePromptState.active?.sessionId;

  const refreshStatus = useCallback(async () => {
    const [nextServer, nextProvider] = await Promise.allSettled([
      desktop.serverStatus(),
      desktop.providerStatus(),
    ]);
    setServer(nextServer.status === 'fulfilled' ? nextServer.value : { ok: false, status: 'error', detail: String(nextServer.reason) });
    setProvider(nextProvider.status === 'fulfilled' ? nextProvider.value : { ok: false, status: 'error', detail: String(nextProvider.reason) });
  }, []);

  const refreshSessions = useCallback(async (projectId: string) => {
    const nextSessions = await listSessions(projectId);
    setSessions(nextSessions);
    setActiveSessionId((current) => current ?? nextSessions[0]?.id ?? null);
  }, []);

  const refreshMessages = useCallback(async (sessionId: string) => {
    setMessages(await listMessages(sessionId));
  }, []);

  const bootstrapProject = useCallback(async (pathOverride?: string) => {
    const trimmedPath = (pathOverride ?? projectPath).trim();
    if (!server.ok || trimmedPath === '') return;
    window.localStorage.setItem('zero.projectPath', trimmedPath);
    const nextProject = await ensureProject(trimmedPath, 'Zero Desktop');
    setProject(nextProject);
    await refreshSessions(nextProject.id);
  }, [projectPath, refreshSessions, server.ok]);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  useEffect(() => {
    getIdentity()
      .then(setIdentity)
      .catch(() => {});
  }, []);

  // Consume any pending project handed off by `zero <path>` on the CLI.
  // Runs once: the Tauri command deletes the handoff file as it reads it,
  // so re-mounts (HMR, navigations) won't re-apply a stale path.
  useEffect(() => {
    desktop
      .consumePendingProject()
      .then((path) => {
        if (path && path.trim()) {
          setProjectPath(path.trim());
        }
      })
      .catch(() => {});
  }, []);

  // Wire `zero://join/<id>?token=…` deep links into the ShareModal.
  // Cold-start delivery (macOS/Windows shell launching the app via the URL)
  // arrives through the deep-link plugin's `getCurrent`. Runtime delivery
  // (already-running app re-activated by a click, single-instance argv on
  // Windows/Linux) is forwarded by `main.rs` as a `zero://deep-link` event.
  useEffect(() => {
    let cleanup: (() => void) | null = null;
    let cancelled = false;

    function handleUrl(url: string) {
      if (!url) return;
      const lower = url.toLowerCase();
      if (!lower.startsWith('zero://join/')) return;
      setPendingJoinInvite(url);
      setShareModalOpen(true);
    }

    (async () => {
      try {
        const [{ getCurrent, onOpenUrl }, { listen }] = await Promise.all([
          import('@tauri-apps/plugin-deep-link'),
          import('@tauri-apps/api/event'),
        ]);

        const startUrls = await getCurrent().catch(() => null);
        if (!cancelled && startUrls && startUrls.length > 0) {
          handleUrl(startUrls[0]!);
        }

        const offPlugin = await onOpenUrl((urls) => {
          if (urls && urls.length > 0) handleUrl(urls[0]!);
        });

        const offBridge = await listen<string>('zero://deep-link', (e) => {
          if (typeof e.payload === 'string') handleUrl(e.payload);
        });

        cleanup = () => {
          offPlugin();
          offBridge();
        };
      } catch {
        // No-op when running outside Tauri (vite dev in a browser).
      }
    })();

    return () => {
      cancelled = true;
      cleanup?.();
    };
  }, []);

  useEffect(() => {
    if (server.ok && !project) {
      bootstrapProject().catch((err: Error) => setError(err.message));
    }
  }, [bootstrapProject, project, server.ok]);

  useEffect(() => {
    if (activeSession?.id) {
      refreshMessages(activeSession.id).catch((err: Error) => setError(err.message));
    }
  }, [activeSession?.id, refreshMessages]);

  // Keep newest message in view when the window resizes.
  useEffect(() => {
    const node = messagesContainerRef.current;
    if (!node || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => {
      messagesEndRef.current?.scrollIntoView({ behavior: 'auto', block: 'end' });
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  // Composer textarea auto-resize, capped at 130 px.
  useEffect(() => {
    const el = composerRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(130, el.scrollHeight)}px`;
  }, [prompt]);

  // pushNotice is the single entry point for transient banners. The rules:
  //   - Errors stack (an error is rare and worth seeing; capped to 3 below).
  //   - Info notices NEVER stack: a new info replaces any existing info so
  //     rapid agent cycling (Tab → plan → explore → build) shows just one
  //     bar at a time instead of pushing the layout down.
  //   - Duplicate of the same text is a no-op.
  //   - Auto-dismiss: 1.6s for info (quick acknowledgment), 6s for errors.
  function pushNotice(level: 'info' | 'error', text: string) {
    const id = `note_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    setLocalNotices((current) => {
      // Same text already on screen — keep it; resetting would re-trigger
      // the timer for no user benefit.
      if (current.some((n) => n.text === text && n.level === level)) {
        return current;
      }
      let next = current;
      if (level === 'info') {
        // Replace, not append — only one info notice visible at a time.
        next = current.filter((n) => n.level !== 'info');
      } else {
        // Cap errors so a bad loop can't blow up the screen.
        next = current.slice(-2);
      }
      return [...next, { id, level, text }];
    });
    const ttl = level === 'info' ? 1600 : 6000;
    setTimeout(() => {
      setLocalNotices((current) => current.filter((n) => n.id !== id));
    }, ttl);
  }

  const runOne = useCallback(
    async (text: string) => {
      const session = sessions.find((s) => s.id === activeSessionId) ?? null;
      if (!session) return;
      setSending(true);
      setSendingStartedAt(Date.now());
      setError(null);
      try {
        await createMessage(session.id, 'user', text);
        await refreshMessages(session.id);
        await runSession(session.id);
        await refreshMessages(session.id);
        // Backend may have auto-renamed the session from a generic title;
        // pull the latest sidebar state so the new title shows up.
        if (project) await refreshSessions(project.id);
        await refreshStatus();
      } catch (err) {
        setError((err as Error).message);
        await refreshStatus();
      } finally {
        setSending(false);
      }
    },
    [activeSessionId, project, refreshMessages, refreshSessions, refreshStatus, sessions],
  );

  const queue = useQueueRunner({ sessionId: activeSessionId, send: runOne, busy: sending });

  // Live activity stream (server-sent events) while sending.
  const activity = useActivityStream(activeSessionId, sending);
  const permissions = usePendingPermissions(activeSessionId);

  // Native drag-drop: capture OS file paths from the desktop window.
  const fileDrop = useFileDrop();
  fileDrop.onDropped((files) => {
    if (!activeSessionId) {
      setError('Create or open a session before attaching files.');
      return;
    }
    const seen = new Set(attachments.map((a) => a.path));
    const fresh = files.filter((f) => !seen.has(f.path));
    if (fresh.length === 0) return;
    setAttachments((prev) => [...prev, ...fresh.map(attachmentEntryFromDropped)]);
    composerRef.current?.focus();
    void uploadAttachmentsForPaths(activeSessionId, fresh.map((f) => f.path));
  });

  async function uploadAttachmentsForPaths(sessionId: string, paths: string[]) {
    try {
      const uploaded = await desktop.uploadAttachments(sessionId, paths, getSessionToken());
      setAttachments((prev) =>
        prev.map((entry) => {
          if (!paths.includes(entry.path)) return entry;
          const match = uploaded.find((u) => u.origName === entry.name);
          if (!match) return { ...entry, status: 'error', error: 'upload returned no match' };
          return { ...entry, status: 'done', attachmentId: match.id };
        }),
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setAttachments((prev) =>
        prev.map((entry) =>
          paths.includes(entry.path) ? { ...entry, status: 'error', error: message } : entry,
        ),
      );
      setError(`Attachment upload failed: ${message}`);
    }
  }

  function removeAttachment(path: string) {
    setAttachments((prev) => prev.filter((f) => f.path !== path));
  }

  // Auto-scroll to the newest content whenever it grows: messages list change,
  // sending toggle, local notices, activity rows, or live token stream.
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [
    messages.length,
    sending,
    localNotices.length,
    activity.items.length,
    activity.live.text,
  ]);

  // Extract task markers + touched files from assistant messages.
  const tasks = useMemo(
    () => extractTasks(messages, activity.live.text),
    [messages, activity.live.text],
  );
  const touchedFiles = useMemo(() => extractTouchedFiles(messages), [messages]);
  const showSidePanel = activeSession !== null;

  // Esc-Esc within 500ms cancels the running task and clears the queue.
  const lastEscRef = useRef<number>(0);
  useEffect(() => {
    function onKey(event: globalThis.KeyboardEvent) {
      if (event.key !== 'Escape') return;
      // Don't hijack Esc inside a modal/input where the user is editing slash text.
      const target = event.target as HTMLElement | null;
      const insideModal = target?.closest('.modal-card');
      if (insideModal) return;
      const now = Date.now();
      if (now - lastEscRef.current < 500) {
        lastEscRef.current = 0;
        if (sending && activeSessionId) {
          void cancelSession(activeSessionId)
            .then((res) => {
              if (res.cancelled) {
                pushNotice('info', 'Run cancelled.');
              }
            })
            .catch((err) => pushNotice('error', `Cancel failed: ${(err as Error).message}`));
        }
        if (queue.queued.length > 0) {
          queue.clear();
          pushNotice('info', 'Queue cleared.');
        }
      } else {
        lastEscRef.current = now;
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sending, activeSessionId, queue.queued.length, queue.clear]);

  async function handleNewSession() {
    if (!project) return;
    setBusy(true);
    setError(null);
    try {
      const session = await createSession({ projectId: project.id, title: 'Desktop session', model: DEFAULT_MODEL, agent: DEFAULT_AGENT });
      setActiveSessionId(session.id);
      await refreshSessions(project.id);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function handleRequestSession() {
    if (!collabRoomId || !collabSelfId) return;
    setBusy(true);
    setError(null);
    try {
      const guestIdentity = { clientId: collabSelfId, displayName: joinedRoom?.displayName ?? 'Guest' };
      await requestSession(guestIdentity, collabRoomId);
      pushNotice('info', 'Session request sent to host. Waiting for approval.');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function handleRenameSession(session: Session) {
    if (!project) return;
    setRenamingId(session.id);
    setRenameDraft(session.title);
    setError(null);
  }

  function cancelRename() {
    setRenamingId(null);
    setRenameDraft('');
  }

  async function commitRename(session: Session) {
    if (!project) {
      cancelRename();
      return;
    }
    const title = renameDraft.trim();
    if (!title || title === session.title) {
      cancelRename();
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await renameSession(session.id, title);
      setSessions((current) => current.map((item) => (item.id === updated.id ? updated : item)));
      cancelRename();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function handleDeleteSession(session: Session) {
    if (!project) return;
    const confirmed = window.confirm(`Delete session "${session.title}"?`);
    if (!confirmed) return;
    setBusy(true);
    setError(null);
    try {
      await deleteSession(session.id);
      const remaining = sessions.filter((item) => item.id !== session.id);
      setSessions(remaining);
      setActiveSessionId((current) => (current === session.id ? remaining[0]?.id ?? null : current));
      if (activeSession?.id === session.id) {
        setMessages([]);
      }
      await refreshSessions(project.id);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function handleChooseProject() {
    setError(null);
    try {
      const selected = await desktop.chooseProjectFolder();
      if (!selected) return;
      setProjectPath(selected);
      setProject(null);
      setSessions([]);
      setMessages([]);
      setActiveSessionId(null);
      await bootstrapProject(selected);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function applyModel(sessionId: string, modelId: string) {
    try {
      const updated = await updateSession(sessionId, { model: modelId });
      setSessions((current) => current.map((s) => (s.id === updated.id ? updated : s)));
      pushNotice('info', `Model changed to ${modelId} (applies to next turn).`);
    } catch (err) {
      pushNotice('error', (err as Error).message);
    }
  }

  async function applyAgent(sessionId: string, agentId: string) {
    try {
      const updated = await updateSession(sessionId, { agent: agentId });
      setSessions((current) => current.map((s) => (s.id === updated.id ? updated : s)));
      pushNotice('info', `Agent → ${agentId}.`);
    } catch (err) {
      pushNotice('error', (err as Error).message);
    }
  }

  async function handleSlashCommand(text: string): Promise<boolean> {
    const cmd = parseSlashCommand(text);
    if (!cmd) return false;
    if (!activeSession) {
      pushNotice('error', 'Select or create a session first.');
      return true;
    }
    switch (cmd.name) {
      case 'help': {
        pushNotice('info', SLASH_HELP_TEXT);
        return true;
      }
      case 'clear': {
        queue.clear();
        pushNotice('info', 'Prompt queue cleared.');
        return true;
      }
      case 'model': {
        const modelId = cmd.args[0];
        if (!modelId) {
          // No args -> open the visual model picker modal.
          setModelPickerOpen(true);
          return true;
        }
        const invalid = validateModelId(modelId);
        if (invalid) {
          pushNotice('error', invalid.message);
          return true;
        }
        await applyModel(activeSession.id, modelId);
        return true;
      }
      case 'agent': {
        const agentId = cmd.args[0];
        if (!agentId) {
          pushNotice('error', 'Usage: /agent <build|plan|explore>');
          return true;
        }
        try {
          const updated = await updateSession(activeSession.id, { agent: agentId });
          setSessions((current) => current.map((s) => (s.id === updated.id ? updated : s)));
          pushNotice('info', `Agent mode changed to ${agentId}.`);
        } catch (err) {
          pushNotice('error', (err as Error).message);
        }
        return true;
      }
      default:
        return false;
    }
  }

  async function handleSend(event: FormEvent) {
    event.preventDefault();
    if (!activeSession) return;
    const text = prompt.trim();
    if (text === '' && attachments.length === 0) return;

    if (attachments.some((a) => a.status === 'uploading')) {
      setQueueWarning('Wait for attachments to finish uploading.');
      setTimeout(() => setQueueWarning(null), 4000);
      return;
    }
    if (attachments.some((a) => a.status === 'error')) {
      setQueueWarning('Remove failed attachments before sending.');
      setTimeout(() => setQueueWarning(null), 4000);
      return;
    }

    if (text.startsWith('/')) {
      const handled = await handleSlashCommand(text);
      if (handled) {
        setPrompt('');
        return;
      }
    }

    if (text === '' && attachments.length > 0) {
      setQueueWarning('Add a prompt describing what to do with the attachment.');
      setTimeout(() => setQueueWarning(null), 4000);
      return;
    }

    const result = queue.enqueue(text);
    if (!result.ok) {
      setQueueWarning(result.reason);
      setTimeout(() => setQueueWarning(null), 4000);
      return;
    }
    setPrompt('');
    setAttachments([]);
    setQueueWarning(null);
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      event.currentTarget.form?.requestSubmit();
      return;
    }
    if (event.key === 'Escape' && prompt.startsWith('/')) {
      event.preventDefault();
      setPrompt('');
      return;
    }
    if (event.key === 'Tab') {
      // Tab inside a slash-only token: complete it.
      if (prompt.trim().startsWith('/') && !prompt.includes(' ')) {
        event.preventDefault();
        const head = prompt.trim().toLowerCase();
        const candidates = ['/model', '/agent', '/clear', '/help'].filter((c) => c.startsWith(head));
        if (candidates.length === 1) {
          setPrompt(`${candidates[0]} `);
        }
        return;
      }
      // Plain Tab cycles agent on the active session.
      if (!event.shiftKey && activeSession && prompt === '') {
        event.preventDefault();
        const order: Array<'build' | 'plan' | 'explore'> = ['build', 'plan', 'explore'];
        const currentIdx = order.indexOf(activeSession.agent as 'build' | 'plan' | 'explore');
        const next = order[(currentIdx + 1) % order.length];
        void applyAgent(activeSession.id, next);
      }
    }
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Zero Desktop</p>
          <PlayfulGreeting />
        </div>
        <UserChip user={currentUser} isDev={isDev} onSignOut={onSignOut} />
      </header>

      {shareConfig ? (
        <CollabBar config={shareConfig} onStopped={() => setShareConfig(null)} />
      ) : null}

      <section className="status-grid">
        <StatusCard title="Zero server" status={server} />
        <StatusCard title="Provider" status={provider} />
        <div className="card session-card">
          <div className="card-stack">
            <p className="label">
              <Power size={11} />
              Session
            </p>
            <p className="value">
              {activeSession ? (
                <>
                  <span className="session-agent">{activeSession.agent}</span>
                  <span className="session-sep">·</span>
                  <span className="session-model">{activeSession.model}</span>
                </>
              ) : (
                <span className="session-empty">No active session</span>
              )}
            </p>
            <p className="detail" title={project?.path ?? ''}>
              {project?.path ?? 'Select a project path'}
            </p>
          </div>
        </div>
      </section>

      <div className="notice-stack">
        {error ? <div className="error"><CircleAlert size={16} /> {error}</div> : null}
        {localNotices.map((notice) => (
          <div className={notice.level === 'error' ? 'error' : 'notice'} key={notice.id}>
            <CircleCheck size={16} />
            <span style={{ whiteSpace: 'pre-wrap' }}>{notice.text}</span>
          </div>
        ))}
      </div>

      <section className={showSidePanel ? 'workspace has-tasks' : 'workspace'}>
        <aside className="sessions">
          <div className="project-form">
            <label className="project-form-label">Working folder</label>
            <div
              aria-readonly="true"
              className={projectPath.trim() ? 'project-folder' : 'project-folder empty'}
              title={projectPath || 'No project selected'}
            >
              <FolderOpen size={14} />
              <span className="project-folder-name">{folderDisplayName(projectPath)}</span>
            </div>
            <div className="project-actions">
              <button disabled={!server.ok || busy} onClick={handleChooseProject} type="button">
                <FolderOpen size={16} /> Choose folder
              </button>
              {joinedRoom ? (
                <button
                  onClick={() => { clearJoinedRoom(); setJoinedRoom(null); }}
                  type="button"
                >
                  <X size={16} /> Leave room
                </button>
              ) : (
                <button
                  disabled={!server.ok || busy || !project || !!shareConfig}
                  onClick={() => setShareModalOpen(true)}
                  title={shareConfig ? 'Already sharing — stop in the bar above' : 'Share this folder with another Zero user'}
                  type="button"
                >
                  <Share2 size={16} /> {shareConfig ? 'Sharing…' : 'Share folder'}
                </button>
              )}
            </div>
          </div>
          <FilesList 
            files={touchedFiles} 
            attachments={attachments.map(a => ({
              name: a.name,
              kind: a.kind,
              status: a.status
            }))}
          />
          <div className="panel-header">
            <h2>Sessions</h2>
            {!joinedRoom || isCollabHost ? (
              <button type="button" onClick={handleNewSession} disabled={!project || busy}>New</button>
            ) : (
              <button
                type="button"
                className="request-session-btn"
                onClick={handleRequestSession}
                disabled={busy}
              >
                Request Session
              </button>
            )}
          </div>
          {sessions.length === 0 ? (
            <p className="muted">
              {!server.ok
                ? 'Waiting for the local daemon to come online…'
                : !project
                  ? 'Click "Choose folder" to pick a project.'
                  : 'No sessions yet. Click "New" to start one.'}
            </p>
          ) : null}
          {sessions.map((session) => {
            const isEditing = renamingId === session.id;
            const rowClass = session.id === activeSession?.id ? 'session-row active' : 'session-row';
            return (
              <div className={isEditing ? `${rowClass} editing` : rowClass} key={session.id}>
                {isEditing ? (
                  <SessionRenameInput
                    initialValue={renameDraft}
                    busy={busy}
                    agent={session.agent}
                    model={session.model}
                    onChange={setRenameDraft}
                    onCommit={() => void commitRename(session)}
                    onCancel={cancelRename}
                  />
                ) : (
                  <button
                    className="session"
                    onClick={() => setActiveSessionId(session.id)}
                    onDoubleClick={() => handleRenameSession(session)}
                    type="button"
                  >
                    <span>{session.title}</span>
                    <small>{session.agent} · {session.model}</small>
                  </button>
                )}
                <div className="session-actions">
                  {isEditing ? (
                    <>
                      <button
                        aria-label="Save title"
                        disabled={busy}
                        onClick={() => void commitRename(session)}
                        type="button"
                      >
                        <Check size={14} />
                      </button>
                      <button
                        aria-label="Cancel rename"
                        disabled={busy}
                        onClick={cancelRename}
                        type="button"
                      >
                        <X size={14} />
                      </button>
                    </>
                  ) : (
                    <>
                      <button aria-label={`Rename ${session.title}`} disabled={busy} onClick={() => handleRenameSession(session)} type="button">
                        <Pencil size={14} />
                      </button>
                      <button aria-label={`Delete ${session.title}`} disabled={busy} onClick={() => handleDeleteSession(session)} type="button">
                        <Trash2 size={14} />
                      </button>
                    </>
                  )}
                </div>
              </div>
            );
          })}
        </aside>

        <section className="chat">
          {sending ? (
            <div className="chat-banner" role="status" aria-live="polite">
              Press <kbd>Esc</kbd> twice to abort the run.
            </div>
          ) : null}
          <div className="messages" ref={messagesContainerRef}>
            {messages.length === 0 ? <EmptyState server={server} provider={provider} /> : messages.map((message) => <MessageCard key={message.id} message={message} userName={currentUser.displayName} />)}
            {sending && !activity.live.text ? <TypingIndicator /> : null}
            {sending ? <ActivityPanel items={activity.items} /> : null}
            {sending && activity.live.text ? <LiveAssistantCard text={activity.live.text} /> : null}
            {permissions.pending.map((req) => (
              <PermissionCard
                key={req.id}
                request={req}
                onResolve={(decision) => permissions.resolve(req.id, decision)}
              />
            ))}
            {activePromptState.interruptRequests.map((req) => (
              <InterruptRequestCard
                key={req.id}
                request={req}
                onResolved={() => activePromptState.dismissRequest(req.id)}
              />
            ))}
            <div ref={messagesEndRef} />
          </div>
          {queue.queued.length > 0 || queueWarning ? (
            <div className="queue-chips" aria-live="polite">
              {queue.queued.map((text, index) => (
                <span className="queue-chip" key={index}>
                  <span title={text}>{text.length > 60 ? `${text.slice(0, 60)}…` : text}</span>
                  <button aria-label="Remove queued prompt" onClick={() => queue.remove(index)} type="button">
                    <X size={12} />
                  </button>
                </span>
              ))}
              {queueWarning ? <span className="queue-warning">{queueWarning}</span> : null}
            </div>
          ) : null}
          <div className={`composer-wrap${fileDrop.isDragOver ? ' is-drag-over' : ''}`}>
            {showInterruptWarning && collabRoomId && activePromptState.active ? (
              <PromptInterruptWarning
                actorNickname={activePromptState.active.actorNickname}
                roomId={collabRoomId}
                sessionId={activePromptState.active.sessionId || collabSessionId || ''}
              />
            ) : null}
            <SlashPreview
              input={prompt}
              onPick={(name) => {
                setPrompt(`${name} `);
                composerRef.current?.focus();
              }}
            />
            {attachments.length > 0 ? (
              <AttachmentChips files={attachments} onRemove={removeAttachment} />
            ) : null}
            {fileDrop.isDragOver ? (
              <div className="composer-drop-overlay" aria-hidden="true">
                <Paperclip size={18} /> <span>Drop files to attach paths</span>
              </div>
            ) : null}
          <form className="composer" onSubmit={handleSend}>
            <textarea
              ref={composerRef}
              disabled={!activeSession}
              onKeyDown={handleComposerKeyDown}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={activeSession ? 'Ask Zero... Enter to send, Shift+Enter newline, /help for commands' : 'Create or select a session first'}
              rows={2}
              value={prompt}
            />
            <button disabled={!activeSession || (prompt.trim() === '' && attachments.length === 0)} type="submit">
              {sending ? <Loader2 className="spin" size={16} /> : <Send size={16} />} {sending ? 'Running' : 'Send'}
            </button>
          </form>
          </div>
        </section>
        {showSidePanel && activeSession ? (
          <SidePanel
            isDev={isDev}
            pending={permissions.pending}
            onResolvePermission={permissions.resolve}
            onCancel={() => {
              if (activeSession) {
                void cancelSession(activeSession.id).then((res) => {
                  if (res.cancelled) pushNotice('info', 'Run cancelled.');
                });
              }
            }}
            onModelClick={() => setModelPickerOpen(true)}
            onRenameSession={() => handleRenameSession(activeSession)}
            onSendQueueClear={() => {
              queue.clear();
              pushNotice('info', 'Queue cleared.');
            }}
            sending={sending}
            sessionAgent={activeSession.agent}
            sessionId={activeSession.id}
            sessionModel={activeSession.model}
            startedAt={sendingStartedAt}
            tasks={tasks}
          />
        ) : null}
      </section>
      <ModelPickerModal
        currentModel={activeSession?.model ?? ''}
        onClose={() => setModelPickerOpen(false)}
        onSelect={(modelId) => {
          if (activeSession) void applyModel(activeSession.id, modelId);
        }}
        open={modelPickerOpen}
      />
      {shareModalOpen && project ? (
        <ShareModal
          folderPath={projectPath}
          projectId={project.id}
          onClose={() => {
            setShareModalOpen(false);
            setPendingJoinInvite(null);
          }}
          onShared={(cfg) => {
            setShareConfig(cfg);
            setShareModalOpen(false);
            setPendingJoinInvite(null);
          }}
          onJoined={(cfg) => {
            setJoinedRoom(cfg);
            setPendingJoinInvite(null);
          }}
          onSignIn={onSignIn}
          initialMode={pendingJoinInvite ? 'join' : undefined}
          initialInvite={pendingJoinInvite ?? undefined}
        />
      ) : null}
      <CollabChatBar
        roomId={collabRoomId}
        selfId={collabSelfId}
        isActive={isCollabActive}
        displayName={joinedRoom?.displayName ?? currentUser.displayName}
        activePromptNickname={
          activePromptState.active && !activePromptState.isSelf
            ? activePromptState.active.actorNickname
            : null
        }
      />
    </main>
  );
}

function SessionRenameInput({
  initialValue,
  busy,
  agent,
  model,
  onChange,
  onCommit,
  onCancel,
}: {
  initialValue: string;
  busy: boolean;
  agent: string;
  model: string;
  onChange: (next: string) => void;
  onCommit: () => void;
  onCancel: () => void;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, []);
  return (
    <div className="session session-editing">
      <input
        ref={inputRef}
        className="session-rename-input"
        defaultValue={initialValue}
        disabled={busy}
        maxLength={120}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            onCommit();
          } else if (e.key === 'Escape') {
            e.preventDefault();
            onCancel();
          }
        }}
        onBlur={onCommit}
        spellCheck={false}
        type="text"
      />
      <small>{agent} · {model}</small>
    </div>
  );
}

function StatusCard({ title, status }: { title: string; status: StatusResponse }) {
  return (
    <div className={status.ok ? 'card ok' : 'card bad'}>
      <div className="card-stack">
        <p className="label">
          <span className="status-dot" aria-hidden="true" />
          {title}
        </p>
        <p className="value">{status.ok ? 'Connected' : status.status}</p>
        {status.ok ? null : <p className="detail">{status.detail}</p>}
      </div>
    </div>
  );
}

function iconForKind(kind: DroppedKind) {
  switch (kind) {
    case 'IMG': return ImageIcon;
    case 'VID': return Film;
    case 'AUD': return Headphones;
    case 'PDF': return FileText;
    case 'DOC': return FileText;
    case 'XLS': return FileSpreadsheet;
    case 'PPT': return Presentation;
    case 'TXT': return FileText;
    default:    return FileIcon;
  }
}

function AttachmentChips({
  files,
  onRemove,
}: {
  files: AttachmentEntry[];
  onRemove: (path: string) => void;
}) {
  return (
    <div className="attachment-chips" aria-label="Attached files">
      {files.map((f) => {
        const Icon = iconForKind(f.kind);
        const chipClass =
          f.status === 'error'
            ? 'attachment-chip attachment-chip-error'
            : f.status === 'uploading'
              ? 'attachment-chip attachment-chip-uploading'
              : 'attachment-chip';
        const titleText = f.status === 'error' ? `${f.path}\n\n${f.error ?? 'upload failed'}` : f.path;
        return (
          <span className={chipClass} key={f.path} title={titleText}>
            {f.status === 'uploading' ? (
              <Loader2 size={12} className="attachment-chip-icon spin" />
            ) : (
              <Icon size={12} className="attachment-chip-icon" />
            )}
            <span className="attachment-chip-tag">{f.kind}</span>
            <span className="attachment-chip-name">
              {f.name}
              {f.status === 'uploading' ? <span className="attachment-chip-status"> · uploading…</span> : null}
              {f.status === 'error' ? <span className="attachment-chip-status"> · failed</span> : null}
            </span>
            <button
              aria-label={`Remove ${f.name}`}
              className="attachment-chip-remove"
              onClick={() => onRemove(f.path)}
              type="button"
            >
              <X size={12} />
            </button>
          </span>
        );
      })}
    </div>
  );
}

function EmptyState({ server, provider }: { server: StatusResponse; provider: StatusResponse }) {
  return (
    <div className="empty">
      <Bot size={42} />
      <h2>Ready for Zero</h2>
      <p>{server.ok ? 'Create a session and send a prompt.' : 'Start zero-server before creating a session.'}</p>
      {!provider.ok ? <p className="hint">Provider is offline. Set <code>ZERO_ROUTER_BASE_URL</code> + <code>ZERO_ROUTER_API_KEY</code> in <code>~/.config/zero/.env</code> to point at any OpenAI-compatible endpoint, then run <code>zero restart</code>.</p> : null}
    </div>
  );
}

function MessageCard({ message, userName }: { message: Message; userName?: string }) {
  const isAssistant = message.role !== 'user';
  const runs = groupParts(message.parts);
  const callsByCallId = buildCallIndex(message.parts);
  const copyableText = runs
    .filter((r): r is Extract<PartRun, { kind: 'text' }> => r.kind === 'text')
    .map((r) => r.text)
    .join('\n')
    .trim();

  const displayRole = isAssistant ? 'Zero' : (userName || 'You');

  return (
    <article className={message.role === 'user' ? 'message user' : 'message assistant'}>
      <header className="message-head">
        <p className="role">{displayRole}</p>
        {isAssistant && copyableText !== '' ? (
          <CopyButton text={copyableText} label="Copy response" />
        ) : null}
      </header>
      <div className="message-content">
        {runs.map((run, idx) =>
          run.kind === 'tool' ? (
            <ToolPart
              key={`${message.id}-tool-${idx}`}
              part={run.part}
              originCall={
                run.part.toolCallId ? callsByCallId.get(run.part.toolCallId) : undefined
              }
              renderCode={(language, lines) => (
                <CodeBlock block={{ type: 'code', language, lines }} />
              )}
            />
          ) : (
            <TextRunRender
              key={`${message.id}-text-${idx}`}
              keyPrefix={`${message.id}-text-${idx}`}
              text={run.text}
              isAssistant={isAssistant}
            />
          ),
        )}
      </div>
    </article>
  );
}

function buildCallIndex(parts: Part[]): Map<string, Part> {
  const index = new Map<string, Part>();
  for (const part of parts) {
    if (part.type === 'tool_call' && part.toolCallId) {
      index.set(part.toolCallId, part);
    }
  }
  return index;
}

type PartRun =
  | { kind: 'text'; text: string }
  | { kind: 'tool'; part: Part };

function groupParts(parts: Part[]): PartRun[] {
  const runs: PartRun[] = [];
  let textBuf: string[] = [];

  const flush = () => {
    if (textBuf.length > 0) {
      runs.push({ kind: 'text', text: textBuf.join('\n') });
      textBuf = [];
    }
  };

  for (const part of parts) {
    if (part.type === 'tool_call' || part.type === 'tool_result') {
      flush();
      runs.push({ kind: 'tool', part });
      continue;
    }
    if (typeof part.text === 'string' && part.text.length > 0) {
      textBuf.push(part.text);
    }
  }
  flush();
  return runs;
}

function TextRunRender({
  keyPrefix,
  text,
  isAssistant,
}: {
  keyPrefix: string;
  text: string;
  isAssistant: boolean;
}) {
  const parts = splitThinking(text);
  const answerText = parts.answer.join('\n');
  const useStructured = isAssistant && shouldUseStructuredRenderer(answerText);

  return (
    <>
      {parts.thinking.length > 0 ? <ThinkingBlock lines={parts.thinking} /> : null}
      {useStructured ? (
        <MessageBody
          text={answerText}
          renderInline={(t) => renderInlineRichText(t)}
          renderCode={(language, lines) => (
            <CodeBlock block={{ type: 'code', language, lines }} />
          )}
          renderProseLine={(line) => <MessageLine line={line} />}
        />
      ) : (
        parseMessageBlocks(parts.answer).map((block, index) =>
          block.type === 'code' ? (
            <CodeBlock block={block} key={`${keyPrefix}-${index}`} />
          ) : (
            <MessageLine key={`${keyPrefix}-${index}`} line={block.text} />
          ),
        )
      )}
    </>
  );
}

function LiveAssistantCard({ text }: { text: string }) {
  const parts = splitThinking(text);
  const answerText = parts.answer.join('\n');
  const useStructured = shouldUseStructuredRenderer(answerText);
  return (
    <article className="message assistant live">
      <header className="message-head">
        <p className="role">Zero <span className="live-pill">streaming</span></p>
      </header>
      {parts.thinking.length > 0 ? <ThinkingBlock lines={parts.thinking} /> : null}
      <div className="message-content">
        {useStructured ? (
          <MessageBody
            text={answerText}
            renderInline={(t) => renderInlineRichText(t)}
            renderCode={(language, lines) => (
              <CodeBlock block={{ type: 'code', language, lines }} />
            )}
            renderProseLine={(line) => <MessageLine line={line} />}
          />
        ) : (
          parseMessageBlocks(parts.answer).map((block, index) => (
            block.type === 'code'
              ? <CodeBlock block={block} key={`live-${index}`} />
              : <MessageLine key={`live-${index}`} line={block.text} />
          ))
        )}
      </div>
    </article>
  );
}

function CopyButton({ text, label = 'Copy', size = 14 }: { text: string; label?: string; size?: number }) {
  const [copied, setCopied] = useState(false);
  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      console.error('clipboard write failed', err);
    }
  }
  return (
    <button aria-label={label} className="copy-button" onClick={handleCopy} title={label} type="button">
      {copied ? <Check size={size} /> : <Copy size={size} />}
      <span>{copied ? 'Copied' : 'Copy'}</span>
    </button>
  );
}

type MessageBlock =
  | { type: 'text'; text: string }
  | { type: 'code'; language: string; lines: string[] };

function parseMessageBlocks(lines: string[]): MessageBlock[] {
  const blocks: MessageBlock[] = [];
  let codeLanguage = '';
  let codeLines: string[] = [];

  for (const line of lines) {
    const fence = line.match(/^```\s*([\w.+-]*)\s*$/);
    if (fence) {
      if (codeLines.length > 0 || codeLanguage) {
        blocks.push({ type: 'code', language: codeLanguage, lines: codeLines });
        codeLanguage = '';
        codeLines = [];
      } else {
        codeLanguage = fence[1] || 'text';
      }
      continue;
    }

    if (codeLanguage) {
      codeLines.push(line);
    } else {
      blocks.push({ type: 'text', text: line });
    }
  }

  if (codeLanguage || codeLines.length > 0) {
    blocks.push({ type: 'code', language: codeLanguage || 'text', lines: codeLines });
  }

  return blocks;
}

function CodeBlock({ block }: { block: Extract<MessageBlock, { type: 'code' }> }) {
  const language = block.language || 'text';
  const isDiff = /^(diff|patch)$/i.test(language) || block.lines.some((line) => /^[-+]/.test(line));
  const code = block.lines.join('\n');

  return (
    <figure className={isDiff ? 'code-panel diff-panel' : 'code-panel'}>
      <figcaption>
        <span>{isDiff ? 'Modified code' : 'Code'}</span>
        <div className="code-meta">
          <small>{language}</small>
          <CopyButton text={code} label="Copy code" />
        </div>
      </figcaption>
      <pre>
        {block.lines.map((line, index) => {
          const kind = isDiff && line.startsWith('+') ? 'added' : isDiff && line.startsWith('-') ? 'removed' : 'context';
          return <code className={`code-line ${kind}`} key={index}>{line || ' '}</code>;
        })}
      </pre>
    </figure>
  );
}

function splitThinking(text: string) {
  const lines = text.split(/\r?\n/);
  const thinking: string[] = [];
  const answer: string[] = [];
  let inThinking = false;

  for (const line of lines) {
    const trimmed = line.trim();
    if (/^<(thinking|think)>$/i.test(trimmed) || /^thinking:?$/i.test(trimmed)) {
      inThinking = true;
      continue;
    }
    if (/^<\/(thinking|think)>$/i.test(trimmed) || /^answer:?$/i.test(trimmed)) {
      inThinking = false;
      continue;
    }
    if (/^(thought|reasoning|process):/i.test(trimmed)) {
      thinking.push(line.replace(/^(thought|reasoning|process):\s*/i, ''));
      continue;
    }
    if (inThinking) {
      thinking.push(line);
    } else {
      answer.push(line);
    }
  }

  return { thinking, answer };
}

function ThinkingBlock({ lines }: { lines: string[] }) {
  const visible = lines.slice(0, 20);
  const hiddenCount = Math.max(0, lines.length - visible.length);

  return (
    <details className="thinking-block">
      <summary><ChevronDown size={14} /> Thinking process <span>{visible.length}{hiddenCount ? `+${hiddenCount}` : ''} lines</span></summary>
      <ol>
        {visible.map((line, index) => <li key={index}>{line || '...'}</li>)}
      </ol>
      {hiddenCount ? <p className="thinking-more">{hiddenCount} more lines hidden to keep this readable.</p> : null}
    </details>
  );
}

// SECTION_TITLE matches lines like "TASK ANALYSIS:", "Plan:", "ROOT CAUSE:" —
// 1-4 capitalized words ending with a colon, optionally followed by content.
const SECTION_TITLE_RE = /^([A-Z][A-Za-z0-9 _\-]{1,40}:)(\s|$)/;

// CONFIRMATION_LINE matches lines where Zero is asking for confirmation:
//   "Also confirm: may I run java -version?"
//   "Confirm: shall I proceed?"
//   "Approve?", "Should I continue?", "Ready to implement. Shall I proceed?"
const CONFIRMATION_RE = /(?:^(?:also\s+)?confirm[: ]|^(?:may|can|should|shall)\s+i\b|\bapprove\?|\bshall i proceed\?|\bproceed\?)/i;

function MessageLine({ line }: { line: string }) {
  if (line.trim() === '') return <div className="message-spacer" />;
  const trimmed = line.trimStart();
  const isList = /^[-*]\s+/.test(trimmed);
  const isNumbered = /^\d+[.)]\s+/.test(trimmed);
  const sectionMatch = trimmed.match(SECTION_TITLE_RE);
  const isConfirm = CONFIRMATION_RE.test(trimmed) && trimmed.includes('?');

  if (sectionMatch) {
    const title = sectionMatch[1];
    const rest = trimmed.slice(title.length);
    return (
      <p className="message-line section-title">
        <strong>{title}</strong>
        {rest ? renderInlineRichText(rest) : null}
      </p>
    );
  }

  if (isConfirm) {
    return (
      <p className="message-line confirm-line">
        {renderInlineRichText(line)}
      </p>
    );
  }

  return (
    <p className={isList || isNumbered ? 'message-line list-line' : 'message-line'}>
      {renderInlineRichText(line)}
    </p>
  );
}

function renderInlineRichText(text: string) {
  const nodes: ReactNode[] = [];
  const pattern = /(==([^=]+)==|\[highlight\]([\s\S]+?)\[\/highlight\]|\[color=(red|green|blue|yellow|purple|gray)\]([\s\S]+?)\[\/color\]|`([^`]+)`)/gi;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text))) {
    if (match.index > lastIndex) {
      nodes.push(<span key={`t-${lastIndex}`}>{text.slice(lastIndex, match.index)}</span>);
    }
    if (match[2]) nodes.push(<mark key={`m-${match.index}`}>{match[2]}</mark>);
    else if (match[3]) nodes.push(<mark key={`h-${match.index}`}>{match[3]}</mark>);
    else if (match[4] && match[5]) nodes.push(<span className={`text-${match[4].toLowerCase()}`} key={`c-${match.index}`}>{match[5]}</span>);
    else if (match[6]) nodes.push(<code key={`code-${match.index}`}>{match[6]}</code>);
    lastIndex = pattern.lastIndex;
  }

  if (lastIndex < text.length) {
    nodes.push(<span key={`t-${lastIndex}`}>{text.slice(lastIndex)}</span>);
  }

  return nodes.length > 0 ? nodes : text;
}
