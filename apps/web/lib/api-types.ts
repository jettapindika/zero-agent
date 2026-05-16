export type Session = {
  id: string;
  projectId: string;
  title: string;
  model: string;
  agent: string;
  createdAt: number;
  updatedAt: number;
};

export type Message = {
  id: string;
  sessionId: string;
  role: "user" | "assistant" | "tool" | "system";
  createdAt: number;
  updatedAt: number;
  parts: Part[];
};

export type Part = {
  id: string;
  messageId: string;
  type: "text" | "tool" | string;
  orderNum: number;
  text?: string;
  toolName?: string;
  toolCallId?: string;
  toolArgsJson?: string;
  toolResultJson?: string;
  isError: boolean;
  createdAt: number;
};

export type BusEvent<T = unknown> = {
  id: string;
  type: string;
  roomId?: string;
  projectId?: string;
  sessionId?: string;
  payload: T;
  createdAt: number;
};

export type EnsureProjectResult = {
  id: string;
  path: string;
  name: string;
  createdAt: number;
  updatedAt: number;
};
