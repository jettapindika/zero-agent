import React, { createContext, useCallback, useMemo, useRef, useState } from "react";
import { client, DEFAULT_MODEL, AVAILABLE_MODELS } from "../router.js";

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

type Props = {
  children: React.ReactNode;
};

type ChatMessage = { role: "user" | "assistant" | "system"; content: string };

export function PiSessionProvider({ children }: Props) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [activeToolCall] = useState<ToolCallInfo | null>(null);
  const [sessions, setSessions] = useState<SessionSummary[]>([{ id: "default", title: "New Session" }]);
  const [activeSession, setActiveSession] = useState<SessionSummary | null>({ id: "default", title: "New Session" });
  const [model, setModel] = useState(DEFAULT_MODEL);
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const historyRef = useRef<ChatMessage[]>([
    {
      role: "system",
      content: "You are a helpful coding assistant running inside a terminal UI called pi-opencode. Respond conversationally and concisely. You do NOT have access to any tools — do not output tool_use XML or attempt to call functions. Just answer directly with text. Use markdown for code blocks when showing code."
    }
  ]);

  const prompt = useCallback(async (text: string) => {
    const userMsg: Message = { id: `user-${Date.now()}`, role: "user", content: text };
    setMessages((prev) => [...prev, userMsg]);
    historyRef.current.push({ role: "user", content: text });

    const assistantId = `assistant-${Date.now()}`;
    setMessages((prev) => [...prev, { id: assistantId, role: "assistant", content: "" }]);
    setIsStreaming(true);
    setLatencyMs(null);

    const controller = new AbortController();
    abortRef.current = controller;
    const startTime = Date.now();
    let firstToken = true;
    let fullContent = "";

    try {
      const stream = await client.chat.completions.create(
        {
          model,
          messages: historyRef.current,
          stream: true,
        },
        { signal: controller.signal }
      );

      for await (const chunk of stream) {
        const delta = chunk.choices[0]?.delta?.content ?? "";
        if (delta) {
          if (firstToken) {
            setLatencyMs(Date.now() - startTime);
            firstToken = false;
          }
          fullContent += delta;
          setMessages((prev) =>
            prev.map((m) => (m.id === assistantId ? { ...m, content: fullContent } : m))
          );
        }
      }

      historyRef.current.push({ role: "assistant", content: fullContent });
    } catch (err: unknown) {
      if ((err as Error).name === "AbortError") {
        historyRef.current.push({ role: "assistant", content: fullContent + " [aborted]" });
      } else {
        const errorMsg = (err as Error).message ?? "Unknown error";
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantId ? { ...m, content: fullContent + `\n\n[Error: ${errorMsg}]` } : m
          )
        );
        historyRef.current.push({ role: "assistant", content: fullContent });
      }
    } finally {
      setIsStreaming(false);
      abortRef.current = null;
    }
  }, [model]);

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
    setActiveSession(sessions.find((s) => s.id === id) ?? null);
  }, [sessions]);

  const newSession = useCallback(async () => {
    const session = { id: `session-${Date.now()}`, title: `Session ${sessions.length + 1}` };
    setSessions((prev) => [...prev, session]);
    setActiveSession(session);
    setMessages([]);
    historyRef.current = [];
  }, [sessions.length]);

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
