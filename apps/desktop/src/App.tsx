import { Bot, ChevronDown, CircleAlert, CircleCheck, FolderOpen, Loader2, Pencil, Power, Send, Trash2, X } from 'lucide-react';
import { FormEvent, KeyboardEvent, type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityPanel } from './components/ActivityPanel';
import { ModelPickerModal } from './components/ModelPickerModal';
import { SlashPreview } from './components/SlashPreview';
import { TaskPanel } from './components/TaskPanel';
import { useActivityStream } from './activity';
import { useQueueRunner } from './queue';
import { parseSlashCommand, validateModelId, SLASH_HELP_TEXT } from './slash';
import { extractTasks } from './tasks';
import { desktop, type StatusResponse } from './tauri';
import { createMessage, createSession, deleteSession, ensureProject, listMessages, listSessions, renameSession, runSession, updateSession, type Message, type Project, type Session } from './zero-api';

const DEFAULT_MODEL = 'cx/gpt-5.5';
const DEFAULT_AGENT = 'build';

function initialProjectPath() {
  return window.localStorage.getItem('zero.projectPath') || '';
}

export function App() {
  const [server, setServer] = useState<StatusResponse>({ ok: false, status: 'checking', detail: 'Checking zero-server...' });
  const [provider, setProvider] = useState<StatusResponse>({ ok: false, status: 'checking', detail: 'Checking provider...' });
  const [project, setProject] = useState<Project | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [localNotices, setLocalNotices] = useState<{ id: string; level: 'info' | 'error'; text: string }[]>([]);
  const [prompt, setPrompt] = useState('');
  const [projectPath, setProjectPath] = useState(initialProjectPath);
  const [busy, setBusy] = useState(false);
  const [sending, setSending] = useState(false);
  const [sendingStartedAt, setSendingStartedAt] = useState<number>(0);
  const [error, setError] = useState<string | null>(null);
  const [queueWarning, setQueueWarning] = useState<string | null>(null);
  const [modelPickerOpen, setModelPickerOpen] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const messagesContainerRef = useRef<HTMLDivElement | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId) ?? sessions[0] ?? null,
    [activeSessionId, sessions],
  );

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
    if (server.ok && !project) {
      bootstrapProject().catch((err: Error) => setError(err.message));
    }
  }, [bootstrapProject, project, server.ok]);

  useEffect(() => {
    if (activeSession?.id) {
      refreshMessages(activeSession.id).catch((err: Error) => setError(err.message));
    }
  }, [activeSession?.id, refreshMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [messages.length, sending, localNotices.length]);

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

  function pushNotice(level: 'info' | 'error', text: string) {
    const id = `note_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    setLocalNotices((current) => [...current, { id, level, text }]);
    setTimeout(() => {
      setLocalNotices((current) => current.filter((n) => n.id !== id));
    }, 6000);
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

  // Extract task markers from assistant messages.
  const tasks = useMemo(() => extractTasks(messages), [messages]);
  const showTaskPanel = tasks.length > 0;

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

  async function handleRenameSession(session: Session) {
    if (!project) return;
    const title = window.prompt('Rename session', session.title)?.trim();
    if (!title || title === session.title) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await renameSession(session.id, title);
      setSessions((current) => current.map((item) => (item.id === updated.id ? updated : item)));
      await refreshSessions(project.id);
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

  async function handleUseProject(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setProject(null);
    setSessions([]);
    setMessages([]);
    setActiveSessionId(null);
    try {
      await bootstrapProject();
    } catch (err) {
      setError((err as Error).message);
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
    if (text === '') return;

    // Slash commands run locally and never enter the queue.
    if (text.startsWith('/')) {
      const handled = await handleSlashCommand(text);
      if (handled) {
        setPrompt('');
        return;
      }
    }

    const result = queue.enqueue(text);
    if (!result.ok) {
      setQueueWarning(result.reason);
      setTimeout(() => setQueueWarning(null), 4000);
      return;
    }
    setPrompt('');
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
    if (event.key === 'Tab' && prompt.trim().startsWith('/') && !prompt.includes(' ')) {
      event.preventDefault();
      const head = prompt.trim().toLowerCase();
      const candidates = ['/model', '/agent', '/clear', '/help'].filter((c) => c.startsWith(head));
      if (candidates.length === 1) {
        setPrompt(`${candidates[0]} `);
      }
    }
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Zero Desktop</p>
          <h1>Local-first coding agent</h1>
        </div>
        <p className="header-note">Local daemon starts automatically. Pick a project, create a session, then chat.</p>
      </header>

      <section className="status-grid">
        <StatusCard title="Zero server" status={server} />
        <StatusCard title="Provider" status={provider} />
        <div className="card compact">
          <Power size={18} />
          <div>
            <p className="label">Session</p>
            <p className="value">{activeSession ? `${activeSession.agent} · ${activeSession.model}` : 'No active session'}</p>
            <p className="detail">{project?.path ?? 'Select a project path'}</p>
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

      <section className={showTaskPanel ? 'workspace has-tasks' : 'workspace'}>
        <aside className="sessions">
          <form className="project-form" onSubmit={handleUseProject}>
            <label htmlFor="project-path">Project path</label>
            <input
              id="project-path"
              onChange={(event) => setProjectPath(event.target.value)}
              placeholder="/absolute/path/to/project"
              value={projectPath}
            />
            <div className="project-actions">
              <button disabled={!server.ok || busy} onClick={handleChooseProject} type="button">
                <FolderOpen size={16} /> Choose folder
              </button>
              <button disabled={!server.ok || projectPath.trim() === '' || busy} type="submit">Use project</button>
            </div>
          </form>
          <div className="panel-header">
            <h2>Sessions</h2>
            <button type="button" onClick={handleNewSession} disabled={!project || busy}>New</button>
          </div>
          {sessions.length === 0 ? <p className="muted">Start the server, then create a session.</p> : null}
          {sessions.map((session) => (
            <div className={session.id === activeSession?.id ? 'session-row active' : 'session-row'} key={session.id}>
              <button
                className="session"
                onClick={() => setActiveSessionId(session.id)}
                type="button"
              >
                <span>{session.title}</span>
                <small>{session.agent} · {session.model}</small>
              </button>
              <div className="session-actions">
                <button aria-label={`Rename ${session.title}`} disabled={busy} onClick={() => handleRenameSession(session)} type="button">
                  <Pencil size={14} />
                </button>
                <button aria-label={`Delete ${session.title}`} disabled={busy} onClick={() => handleDeleteSession(session)} type="button">
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))}
        </aside>

        <section className="chat">
          <div className="messages" ref={messagesContainerRef}>
            {messages.length === 0 ? <EmptyState server={server} provider={provider} /> : messages.map((message) => <MessageCard key={message.id} message={message} />)}
            {sending ? <ActivityPanel items={activity} startedAt={sendingStartedAt} /> : null}
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
          <div className="composer-wrap">
            <SlashPreview
              input={prompt}
              onPick={(name) => {
                setPrompt(`${name} `);
                composerRef.current?.focus();
              }}
            />
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
            <button disabled={!activeSession || prompt.trim() === ''} type="submit">
              {sending ? <Loader2 className="spin" size={16} /> : <Send size={16} />} {sending ? 'Running' : 'Send'}
            </button>
          </form>
          </div>
        </section>
        {showTaskPanel ? <TaskPanel tasks={tasks} /> : null}
      </section>
      <ModelPickerModal
        currentModel={activeSession?.model ?? ''}
        onClose={() => setModelPickerOpen(false)}
        onSelect={(modelId) => {
          if (activeSession) void applyModel(activeSession.id, modelId);
        }}
        open={modelPickerOpen}
      />
    </main>
  );
}

function StatusCard({ title, status }: { title: string; status: StatusResponse }) {
  return (
    <div className={status.ok ? 'card ok' : 'card bad'}>
      {status.ok ? <CircleCheck size={20} /> : <CircleAlert size={20} />}
      <div>
        <p className="label">{title}</p>
        <p className="value">{status.ok ? 'Connected' : status.status}</p>
        <p className="detail">{status.detail}</p>
      </div>
    </div>
  );
}

function EmptyState({ server, provider }: { server: StatusResponse; provider: StatusResponse }) {
  return (
    <div className="empty">
      <Bot size={42} />
      <h2>Ready for Zero</h2>
      <p>{server.ok ? 'Create a session and send a prompt.' : 'Start zero-server before creating a session.'}</p>
      {!provider.ok ? <p className="hint">Provider is offline. Start 9router or set ZERO_ROUTER_BASE_URL before running prompts.</p> : null}
    </div>
  );
}

function MessageCard({ message }: { message: Message }) {
  const parts = splitThinking(message.parts.map((part) => part.text ?? part.type).join('\n'));

  return (
    <article className={message.role === 'user' ? 'message user' : 'message assistant'}>
      <p className="role">{message.role}</p>
      {parts.thinking.length > 0 ? <ThinkingBlock lines={parts.thinking} /> : null}
      <div className="message-content">
        {parseMessageBlocks(parts.answer).map((block, index) => (
          block.type === 'code'
            ? <CodeBlock block={block} key={`${message.id}-${index}`} />
            : <MessageLine key={`${message.id}-${index}`} line={block.text} />
        ))}
      </div>
    </article>
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

  return (
    <figure className={isDiff ? 'code-panel diff-panel' : 'code-panel'}>
      <figcaption>
        <span>{isDiff ? 'Modified code' : 'Code'}</span>
        <small>{language}</small>
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

function MessageLine({ line }: { line: string }) {
  if (line.trim() === '') return <div className="message-spacer" />;
  const trimmed = line.trimStart();
  const isList = /^[-*]\s+/.test(trimmed);
  const isNumbered = /^\d+[.)]\s+/.test(trimmed);

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
