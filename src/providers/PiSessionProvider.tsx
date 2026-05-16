import React, { createContext, useEffect, useMemo, useRef, useState } from "react";

export type ToolCallInfo = {
  id: string;
  toolName: string;
  args: Record<string, unknown>;
  result?: string;
  isError?: boolean;
  durationMs?: number;
  status: "running" | "done" | "error";
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

type SessionLike = {
  messages?: Array<{ id: string; role: string; content: string }>;
  isStreaming?: boolean;
  subscribe: (listener: (event: Record<string, unknown>) => void) => () => void;
  prompt: (text: string) => Promise<void>;
  abort: () => void;
  cycleModel: () => void;
  dispose?: () => void;
};

type SessionFactory = () => Promise<{ session: SessionLike; extensionsResult: unknown }>;
type SessionManagerFactory = () => Promise<{ list: () => Promise<SessionSummary[]> }>;

type Props = {
  children: React.ReactNode;
  createSession?: SessionFactory;
  createSessionManager?: SessionManagerFactory;
};

export function PiSessionProvider({ children, createSession, createSessionManager }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [activeToolCall, setActiveToolCall] = useState<ToolCallInfo | null>(null);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [activeSession, setActiveSession] = useState<SessionSummary | null>(null);
  const [model, setModel] = useState("default");
  const [thinkingLevel] = useState("default");
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const sessionRef = useRef<SessionLike | null>(null);
  const promptStartedAtRef = useRef<number | null>(null);
  const firstDeltaAtRef = useRef<number | null>(null);
  const toolStartedAtRef = useRef(new Map<string, number>());

  useEffect(() => {
    let unsub: (() => void) | undefined;

    async function bootstrap() {
      const sessionManager = createSessionManager
        ? await createSessionManager()
        : { list: async () => [] as SessionSummary[] };
      setSessions(await sessionManager.list());

      if (!createSession) return;

      const result = await createSession();
      sessionRef.current = result.session;

      unsub = result.session.subscribe((event: Record<string, unknown>) => {
        const type = event.type as string;

        switch (type) {
          case "agent_start":
            setIsStreaming(true);
            firstDeltaAtRef.current = null;
            break;

          case "message_start":
            if (event.role === "assistant") {
              setMessages((current) => [
                ...current,
                { id: String(event.messageId ?? `a-${Date.now()}`), role: "assistant", content: "", toolCalls: [] }
              ]);
            }
            break;

          case "message_update": {
            const assistantEvent = event.assistantMessageEvent as { type?: string; delta?: string } | undefined;
            if (assistantEvent?.type === "text_delta" && assistantEvent.delta) {
              if (firstDeltaAtRef.current === null && promptStartedAtRef.current !== null) {
                firstDeltaAtRef.current = Date.now();
                setLatencyMs(firstDeltaAtRef.current - promptStartedAtRef.current);
              }
              const delta = assistantEvent.delta;
              setMessages((current) => {
                const last = current[current.length - 1];
                if (!last || last.role !== "assistant") return current;
                return [...current.slice(0, -1), { ...last, content: last.content + delta }];
              });
            }
            break;
          }

          case "tool_execution_start": {
            const toolCall: ToolCallInfo = {
              id: String(event.toolCallId),
              toolName: String(event.toolName),
              args: (event.args as Record<string, unknown>) ?? {},
              status: "running"
            };
            toolStartedAtRef.current.set(toolCall.id, Date.now());
            setActiveToolCall(toolCall);
            setMessages((current) => {
              const lastAssistant = [...current].reverse().find((m) => m.role === "assistant");
              if (!lastAssistant) return current;
              return current.map((m) =>
                m.id === lastAssistant.id
                  ? { ...m, toolCalls: [...(m.toolCalls ?? []), toolCall] }
                  : m
              );
            });
            break;
          }

          case "tool_execution_end": {
            const id = String(event.toolCallId);
            const startedAt = toolStartedAtRef.current.get(id) ?? Date.now();
            const duration = Date.now() - startedAt;
            const status = event.isError ? "error" as const : "done" as const;
            const result = String(event.result ?? "");

            setActiveToolCall((current) =>
              current?.id === id
                ? { ...current, result, isError: Boolean(event.isError), durationMs: duration, status }
                : current
            );
            setMessages((current) =>
              current.map((m) => ({
                ...m,
                toolCalls: m.toolCalls?.map((tc) =>
                  tc.id === id
                    ? { ...tc, result, isError: Boolean(event.isError), durationMs: duration, status }
                    : tc
                )
              }))
            );
            break;
          }

          case "agent_end":
            setIsStreaming(false);
            break;
        }
      });
    }

    void bootstrap();
    return () => { unsub?.(); };
  }, [createSession, createSessionManager]);

  const value = useMemo<PiSessionValue>(
    () => ({
      messages,
      isStreaming,
      activeToolCall,
      sessions,
      activeSession,
      model,
      thinkingLevel,
      latencyMs,
      async prompt(text: string) {
        promptStartedAtRef.current = Date.now();
        setMessages((current) => [...current, { id: `user-${Date.now()}`, role: "user", content: text }]);
        await sessionRef.current?.prompt(text);
      },
      abort() {
        sessionRef.current?.abort();
      },
      cycleModel() {
        sessionRef.current?.cycleModel();
      },
      async switchSession(id: string) {
        setActiveSession(sessions.find((s) => s.id === id) ?? null);
      },
      async newSession() {
        const session = { id: `session-${Date.now()}`, title: `Session ${sessions.length + 1}` };
        setSessions((current) => [...current, session]);
        setActiveSession(session);
        setMessages([]);
      },
      setModel
    }),
    [messages, isStreaming, activeToolCall, sessions, activeSession, model, thinkingLevel, latencyMs]
  );

  return <PiSessionContext value={value}>{children}</PiSessionContext>;
}
