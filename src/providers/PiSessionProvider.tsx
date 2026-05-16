import React, { createContext, useCallback, useMemo, useRef, useState } from "react";
import type { ChatCompletionMessageParam } from "openai/resources/chat/completions.js";
import { runAgentLoop } from "../agent/loop.js";
import type { AgentEvent } from "../agent/loop.js";
import { DEFAULT_MODEL, AVAILABLE_MODELS } from "../router.js";
import { listSessions, loadSession, saveSession } from "../session/storage.js";
import type { SavedSession } from "../session/storage.js";

export type ToolCallInfo = {
  id: string;
  toolName: string;
  args: Record<string, unknown>;
  result?: string;
  isError?: boolean;
  durationMs?: number;
  status: "pending" | "running" | "done" | "error";
};

export type Message = {
  id: string;
  role: "user" | "assistant" | "tool";
  content: string;
  thinking?: string;
  toolCalls?: ToolCallInfo[];
};

export type SessionSummary = {
  id: string;
  title: string;
};

export type PiSessionValue = {
  messages: Message[];
  isStreaming: boolean;
  activeToolCall: ToolCallInfo | null;
  sessions: SessionSummary[];
  activeSession: SessionSummary | null;
  model: string;
  thinkingLevel: string;
  latencyMs: number | null;
  prompt: (text: string) => Promise<void>;
  abort: () => void;
  cycleModel: () => void;
  switchSession: (id: string) => Promise<void>;
  newSession: () => Promise<void>;
  setModel: (value: string) => void;
};

export const PiSessionContext = createContext<PiSessionValue | null>(null);

type Props = {
  children: React.ReactNode;
  storage?: SessionStorage;
};

type SessionStorage = {
  listSessions: () => Promise<Array<{ id: string; title: string; updatedAt: number }>>;
  loadSession: (id: string) => Promise<SavedSession | null>;
  saveSession: (session: SavedSession) => Promise<void>;
};

const defaultStorage: SessionStorage = {
  listSessions,
  loadSession,
  saveSession,
};

const SYSTEM_PROMPT = `You are a helpful coding assistant running inside a terminal UI called pi-opencode. You have access to tools for reading, writing, editing files, running bash commands, searching with grep, and finding files with glob.

Guidelines:
- Use tools to explore the codebase before making changes
- Read files before editing them
- Make minimal, focused changes
- Run tests after making changes when possible
- Use bash for git operations, running scripts, installing packages
- Prefer editing existing files over writing new ones
- When showing code, use markdown code blocks with language tags`;

export function PiSessionProvider({ children, storage = defaultStorage }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [activeToolCall, setActiveToolCall] = useState<ToolCallInfo | null>(null);
  const [sessions, setSessions] = useState<SessionSummary[]>([{ id: "default", title: "New Session" }]);
  const [activeSession, setActiveSession] = useState<SessionSummary | null>({ id: "default", title: "New Session" });
  const [model, setModel] = useState(DEFAULT_MODEL);
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const historyRef = useRef<ChatCompletionMessageParam[]>([
    { role: "system", content: SYSTEM_PROMPT }
  ]);
  const activeSessionRef = useRef<SessionSummary | null>({ id: "default", title: "New Session" });
  const createdAtRef = useRef<number>(Date.now());

  const persistCurrentSession = useCallback(async (nextMessages: Message[], nextHistory: ChatCompletionMessageParam[]) => {
    const session = activeSessionRef.current;
    if (!session) return;
    await storage.saveSession({
      id: session.id,
      title: session.title,
      createdAt: createdAtRef.current,
      updatedAt: Date.now(),
      uiMessages: nextMessages,
      modelMessages: nextHistory,
    });
  }, [storage]);

  React.useEffect(() => {
    let cancelled = false;

    async function hydrate() {
      const saved = await storage.listSessions();
      if (cancelled) return;
      if (saved.length === 0) return;

      const summaries = saved.map((session) => ({ id: session.id, title: session.title }));
      setSessions(summaries);
      const latest = await storage.loadSession(saved[0]!.id);
      if (cancelled || !latest) return;

      const summary = { id: latest.id, title: latest.title };
      activeSessionRef.current = summary;
      createdAtRef.current = latest.createdAt;
      setActiveSession(summary);
      setMessages(latest.uiMessages);
      historyRef.current = latest.modelMessages.length > 0 ? latest.modelMessages : [{ role: "system", content: SYSTEM_PROMPT }];
    }

    void hydrate();
    return () => { cancelled = true; };
  }, [storage]);

  const prompt = useCallback(async (text: string) => {
    const userMsg: Message = { id: `user-${Date.now()}`, role: "user", content: text };
    const beforePromptMessages = [...messages, userMsg];
    setMessages((prev) => [...prev, userMsg]);
    historyRef.current.push({ role: "user", content: text });

    const assistantId = `assistant-${Date.now()}`;
    setMessages((prev) => [...prev, { id: assistantId, role: "assistant", content: "", toolCalls: [] }]);
    setIsStreaming(true);
    setLatencyMs(null);
    setActiveToolCall(null);

    const controller = new AbortController();
    abortRef.current = controller;
    const startTime = Date.now();
    let firstToken = true;
    let fullContent = "";
    const toolCalls: ToolCallInfo[] = [];
    const toolStartedAt = new Map<string, number>();

    const onEvent = (event: AgentEvent) => {
      switch (event.type) {
        case "text_delta":
          if (firstToken) {
            setLatencyMs(Date.now() - startTime);
            firstToken = false;
          }
          fullContent += event.delta;
          setMessages((prev) =>
            prev.map((m) => (m.id === assistantId ? { ...m, content: fullContent } : m))
          );
          break;

        case "tool_start": {
          const tc: ToolCallInfo = {
            id: event.callId,
            toolName: event.toolName,
            args: event.args,
            status: "running",
          };
          toolStartedAt.set(event.callId, Date.now());
          toolCalls.push(tc);
          setActiveToolCall(tc);
          setMessages((prev) =>
            prev.map((m) => (m.id === assistantId ? { ...m, toolCalls: [...toolCalls] } : m))
          );
          break;
        }

        case "tool_end": {
          const idx = toolCalls.findIndex((t) => t.id === event.callId);
          if (idx >= 0) {
            toolCalls[idx] = {
              ...toolCalls[idx]!,
              result: event.result.output,
              isError: event.result.isError,
              status: event.result.isError ? "error" : "done",
              durationMs: Date.now() - (toolStartedAt.get(event.callId) ?? startTime),
            };
          }
          const updated = toolCalls[idx] ?? null;
          setActiveToolCall(updated);
          setMessages((prev) =>
            prev.map((m) => (m.id === assistantId ? { ...m, toolCalls: [...toolCalls] } : m))
          );
          break;
        }

        case "turn_end":
          break;

        case "error":
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, content: fullContent + `\n\n[Error: ${event.message}]` } : m
            )
          );
          break;
      }
    };

    try {
      const updatedHistory = await runAgentLoop({
        model,
        messages: historyRef.current,
        cwd: process.cwd(),
        signal: controller.signal,
        onEvent,
      });
      historyRef.current = updatedHistory;
      await persistCurrentSession(
        [...beforePromptMessages, { id: assistantId, role: "assistant", content: fullContent, toolCalls: [...toolCalls] }],
        updatedHistory
      );
    } catch (err: unknown) {
      if ((err as Error).name !== "AbortError") {
        const errorMsg = (err as Error).message ?? "Unknown error";
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantId ? { ...m, content: fullContent + `\n\n[Error: ${errorMsg}]` } : m
          )
        );
      }
    } finally {
      setIsStreaming(false);
      abortRef.current = null;
    }
  }, [model, messages, persistCurrentSession]);

  const abort = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  const cycleModel = useCallback(() => {
    setModel((current) => {
      const idx = AVAILABLE_MODELS.findIndex((m) => m.value === current);
      const next = AVAILABLE_MODELS[(idx + 1) % AVAILABLE_MODELS.length];
      return next?.value ?? DEFAULT_MODEL;
    });
  }, []);

  const switchSession = useCallback(async (id: string) => {
    const loaded = await storage.loadSession(id);
    const summary = sessions.find((s) => s.id === id) ?? (loaded ? { id: loaded.id, title: loaded.title } : null);
    activeSessionRef.current = summary;
    setActiveSession(summary);
    if (!loaded) return;
    createdAtRef.current = loaded.createdAt;
    setMessages(loaded.uiMessages);
    historyRef.current = loaded.modelMessages.length > 0 ? loaded.modelMessages : [{ role: "system", content: SYSTEM_PROMPT }];
  }, [sessions, storage]);

  const newSession = useCallback(async () => {
    await persistCurrentSession(messages, historyRef.current);
    const session = { id: `session-${Date.now()}`, title: `Session ${sessions.length + 1}` };
    setSessions((prev) => [...prev, session]);
    setActiveSession(session);
    activeSessionRef.current = session;
    createdAtRef.current = Date.now();
    setMessages([]);
    historyRef.current = [{ role: "system", content: SYSTEM_PROMPT }];
    await storage.saveSession({
      id: session.id,
      title: session.title,
      createdAt: createdAtRef.current,
      updatedAt: Date.now(),
      uiMessages: [],
      modelMessages: historyRef.current,
    });
  }, [messages, persistCurrentSession, sessions.length, storage]);

  const value = useMemo<PiSessionValue>(
    () => ({
      messages,
      isStreaming,
      activeToolCall,
      sessions,
      activeSession,
      model,
      thinkingLevel: "default",
      latencyMs,
      prompt,
      abort,
      cycleModel,
      switchSession,
      newSession,
      setModel,
    }),
    [messages, isStreaming, activeToolCall, sessions, activeSession, model, latencyMs, prompt, abort, cycleModel, switchSession, newSession]
  );

  return <PiSessionContext value={value}>{children}</PiSessionContext>;
}
