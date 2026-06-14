"use client";

import { Bot, PanelLeft, Send, TerminalSquare } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createMessage, createSession, ensureProject, listMessages, listSessions, runSession, subscribeProjectEvents } from "@/lib/api";
import type { Message } from "@/lib/api-types";
import { useAppStore } from "@/lib/store";

const projectPath = "/workspace";
const projectName = "Zero";

export function AppShell() {
  const { projectId, sessions, activeSessionId, messages, setProjectId, setSessions, setActiveSessionId, setMessages, applyEvent } = useAppStore();
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refreshSessions = useCallback(async (nextProjectId: string) => {
    const nextSessions = await listSessions(nextProjectId);
    setSessions(nextSessions);
  }, [setSessions]);

  const activeSession = useMemo(() => sessions.find((session) => session.id === activeSessionId) ?? sessions[0], [activeSessionId, sessions]);

  const refreshMessages = useCallback(async (sessionId: string) => {
    const nextMessages = await listMessages(sessionId);
    setMessages(nextMessages);
  }, [setMessages]);

  useEffect(() => {
    let active = true;
    ensureProject(projectPath, projectName)
      .then((project) => {
        if (!active) return;
        setProjectId(project.id);
        return refreshSessions(project.id);
      })
      .catch((err: Error) => setError(err.message));
    return () => {
      active = false;
    };
  }, [refreshSessions, setProjectId]);

  useEffect(() => {
    if (!projectId) return;
    return subscribeProjectEvents(projectId, (event) => {
      applyEvent(event);
      refreshSessions(projectId).catch((err: Error) => setError(err.message));
      if (event.sessionId) refreshMessages(event.sessionId).catch((err: Error) => setError(err.message));
    });
  }, [applyEvent, projectId, refreshMessages, refreshSessions]);

  useEffect(() => {
    if (!activeSessionId && activeSession?.id) setActiveSessionId(activeSession.id);
  }, [activeSession?.id, activeSessionId, setActiveSessionId]);

  useEffect(() => {
    if (!activeSession?.id) return;
    refreshMessages(activeSession.id).catch((err: Error) => setError(err.message));
  }, [activeSession?.id, refreshMessages]);

  async function handleNewSession() {
    if (!projectId) return;
    setBusy(true);
    setError(null);
    try {
      const session = await createSession({ projectId, title: "New session", model: "anthropic/claude-sonnet-4-5", agent: "build" });
      setActiveSessionId(session.id);
      await refreshSessions(projectId);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function handleSendPrompt() {
    if (!activeSession?.id || prompt.trim() === "") return;
    setBusy(true);
    setError(null);
    try {
      await createMessage(activeSession.id, "user", prompt.trim());
      setPrompt("");
      await runSession(activeSession.id);
      await refreshMessages(activeSession.id);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="grid min-h-screen grid-rows-[auto_1fr_auto] bg-[#0b1020] text-slate-100">
      <header className="flex items-center justify-between border-b border-slate-800 bg-slate-950/80 px-4 py-3">
        <div className="flex items-center gap-3">
          <TerminalSquare className="h-5 w-5 text-cyan-300" aria-hidden="true" />
          <div>
            <h1 className="text-sm font-semibold tracking-wide">Zero</h1>
            <p className="text-xs text-slate-400">{activeSession?.model ?? "no model"} · {activeSession?.agent ?? "build"}</p>
          </div>
        </div>
        <button className="rounded-md border border-cyan-400/40 px-3 py-1 text-sm text-cyan-200 hover:bg-cyan-400/10 focus:outline focus:outline-2 focus:outline-cyan-300 disabled:opacity-50" onClick={handleNewSession} disabled={!projectId || busy}>
          New session
        </button>
      </header>

      <section className="grid min-h-0 grid-cols-[18rem_1fr_22rem]">
        <aside className="min-h-0 border-r border-slate-800 bg-slate-950/50 p-3">
          <div className="mb-3 flex items-center gap-2 text-xs uppercase tracking-widest text-slate-500"><PanelLeft className="h-4 w-4" aria-hidden="true" /> Sessions</div>
          <div className="space-y-2" aria-label="Sessions">
            {sessions.map((session) => (
              <button key={session.id} className={`w-full rounded-lg border px-3 py-2 text-left text-sm focus:outline focus:outline-2 focus:outline-cyan-300 ${session.id === activeSession?.id ? "border-cyan-400/50 bg-cyan-400/10" : "border-slate-800 bg-slate-900/60 hover:bg-slate-800/70"}`} onClick={() => setActiveSessionId(session.id)}>
                <span className="block truncate font-medium">{session.title}</span>
                <span className="block truncate text-xs text-slate-500">{session.agent}</span>
              </button>
            ))}
          </div>
        </aside>

        <section className="min-h-0 overflow-y-auto p-4" aria-live="polite">
          <div className="mx-auto flex max-w-4xl flex-col gap-3">
            {error ? <div className="rounded-lg border border-red-400/40 bg-red-400/10 p-3 text-sm text-red-200">{error}</div> : null}
            {messages.length === 0 ? <EmptyState /> : messages.map((message) => <MessageBubble key={message.id} message={message} />)}
          </div>
        </section>

        <aside className="min-h-0 border-l border-slate-800 bg-slate-950/50 p-4">
          <h2 className="mb-3 text-sm font-semibold">Details</h2>
          <dl className="space-y-3 text-sm text-slate-300">
            <div><dt className="text-xs uppercase text-slate-500">Project</dt><dd>{projectId ?? "loading"}</dd></div>
            <div><dt className="text-xs uppercase text-slate-500">Session</dt><dd className="break-all">{activeSession?.id ?? "none"}</dd></div>
            <div><dt className="text-xs uppercase text-slate-500">Messages</dt><dd>{messages.length}</dd></div>
          </dl>
        </aside>
      </section>

      <footer className="border-t border-slate-800 bg-slate-950 p-3">
        <form className="mx-auto flex max-w-5xl gap-2" onSubmit={(event) => { event.preventDefault(); void handleSendPrompt(); }}>
          <label className="sr-only" htmlFor="prompt">Prompt</label>
          <input id="prompt" className="min-h-11 flex-1 rounded-lg border border-slate-700 bg-slate-900 px-4 text-sm outline-none placeholder:text-slate-500 focus:border-cyan-300" placeholder={activeSession ? "Ask Zero..." : "Create a session first"} value={prompt} onChange={(event) => setPrompt(event.target.value)} disabled={!activeSession || busy} />
          <button className="inline-flex min-h-11 items-center gap-2 rounded-lg bg-cyan-300 px-4 text-sm font-semibold text-slate-950 hover:bg-cyan-200 disabled:cursor-not-allowed disabled:opacity-50" disabled={!activeSession || prompt.trim() === "" || busy} type="submit"><Send className="h-4 w-4" aria-hidden="true" /> Send</button>
        </form>
      </footer>
    </main>
  );
}

function EmptyState() {
  return <div className="flex min-h-80 flex-col items-center justify-center rounded-xl border border-dashed border-slate-800 bg-slate-950/40 text-center"><Bot className="mb-3 h-8 w-8 text-cyan-300" aria-hidden="true" /><h2 className="text-lg font-semibold">Ready for a prompt</h2><p className="mt-1 max-w-md text-sm text-slate-400">Create or select a session, then send a message. Streaming agent output lands here once the agent loop is wired.</p></div>;
}

function MessageBubble({ message }: { message: Message }) {
  const isUser = message.role === "user";
  return <article className={`rounded-xl border p-4 ${isUser ? "border-cyan-400/30 bg-cyan-400/10" : "border-slate-800 bg-slate-900/70"}`}><div className="mb-2 text-xs uppercase tracking-widest text-slate-500">{message.role}</div><div className="rounded-lg border border-slate-700 bg-slate-800/80 p-3 text-sm leading-6 text-slate-100 shadow-sm">{message.parts.map((part) => <p key={part.id}>{part.text ?? part.type}</p>)}</div></article>;
}
