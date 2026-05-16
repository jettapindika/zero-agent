import type { ChatCompletionMessageParam } from "openai/resources/chat/completions.js";
import { client } from "../router.js";
import { OPENAI_TOOLS, TOOL_MAP } from "../tools/index.js";
import type { ToolResult } from "../tools/index.js";

type FunctionToolCall = {
  id: string;
  type: "function";
  function: { name: string; arguments: string };
};

export type AgentEvent =
  | { type: "text_delta"; delta: string }
  | { type: "tool_start"; callId: string; toolName: string; args: Record<string, unknown> }
  | { type: "tool_end"; callId: string; toolName: string; result: ToolResult }
  | { type: "turn_end" }
  | { type: "error"; message: string };

export type AgentOptions = {
  model: string;
  messages: ChatCompletionMessageParam[];
  cwd: string;
  signal?: AbortSignal;
  onEvent: (event: AgentEvent) => void;
  onPermissionRequest?: (toolName: string, args: Record<string, unknown>) => Promise<boolean>;
};

const MAX_TURNS = 25;

export async function runAgentLoop(options: AgentOptions): Promise<ChatCompletionMessageParam[]> {
  const { model, messages, cwd, signal, onEvent, onPermissionRequest } = options;
  const history = [...messages];
  let turns = 0;

  while (turns < MAX_TURNS) {
    if (signal?.aborted) break;
    turns++;

    const stream = await client.chat.completions.create(
      {
        model,
        messages: history,
        tools: OPENAI_TOOLS,
        tool_choice: "auto",
        stream: true,
      },
      { signal }
    );

    let assistantContent = "";
    const toolCalls: FunctionToolCall[] = [];
    const toolCallArgs: Map<number, string> = new Map();

    for await (const chunk of stream) {
      if (signal?.aborted) break;
      const choice = chunk.choices[0];
      if (!choice) continue;

      const delta = choice.delta;

      if (delta?.content) {
        assistantContent += delta.content;
        onEvent({ type: "text_delta", delta: delta.content });
      }

      if (delta?.tool_calls) {
        for (const tc of delta.tool_calls) {
          if (tc.id) {
            toolCalls[tc.index] = {
              id: tc.id,
              type: "function",
              function: { name: tc.function?.name ?? "", arguments: "" },
            };
          }
          if (tc.function?.arguments) {
            const existing = toolCallArgs.get(tc.index) ?? "";
            toolCallArgs.set(tc.index, existing + tc.function.arguments);
          }
          if (tc.function?.name && toolCalls[tc.index]) {
            toolCalls[tc.index]!.function.name = tc.function.name;
          }
        }
      }

      if (choice.finish_reason === "stop") {
        history.push({ role: "assistant", content: assistantContent });
        onEvent({ type: "turn_end" });
        return history;
      }
    }

    for (const [idx, argsStr] of toolCallArgs) {
      if (toolCalls[idx]) {
        toolCalls[idx]!.function.arguments = argsStr;
      }
    }

    const validToolCalls = toolCalls.filter(Boolean);

    if (validToolCalls.length === 0) {
      if (assistantContent) {
        history.push({ role: "assistant", content: assistantContent });
      }
      onEvent({ type: "turn_end" });
      return history;
    }

    history.push({
      role: "assistant",
      content: assistantContent || null,
      tool_calls: validToolCalls,
    } as ChatCompletionMessageParam);

    for (const toolCall of validToolCalls) {
      if (signal?.aborted) break;

      const toolName = toolCall.function.name;
      const tool = TOOL_MAP.get(toolName);

      let args: Record<string, unknown> = {};
      try {
        args = JSON.parse(toolCall.function.arguments || "{}");
      } catch {
        const errorResult: ToolResult = { output: "Failed to parse tool arguments", title: `${toolName} (parse error)`, isError: true };
        onEvent({ type: "tool_start", callId: toolCall.id, toolName, args: {} });
        onEvent({ type: "tool_end", callId: toolCall.id, toolName, result: errorResult });
        history.push({ role: "tool", tool_call_id: toolCall.id, content: errorResult.output });
        continue;
      }

      onEvent({ type: "tool_start", callId: toolCall.id, toolName, args });

      if (tool?.requiresPermission && onPermissionRequest) {
        const allowed = await onPermissionRequest(toolName, args);
        if (!allowed) {
          const denied: ToolResult = { output: "Permission denied by user", title: `${toolName} (denied)`, isError: true };
          onEvent({ type: "tool_end", callId: toolCall.id, toolName, result: denied });
          history.push({ role: "tool", tool_call_id: toolCall.id, content: denied.output });
          continue;
        }
      }

      if (!tool) {
        const notFound: ToolResult = { output: `Unknown tool: ${toolName}`, title: `${toolName} (not found)`, isError: true };
        onEvent({ type: "tool_end", callId: toolCall.id, toolName, result: notFound });
        history.push({ role: "tool", tool_call_id: toolCall.id, content: notFound.output });
        continue;
      }

      try {
        const result = await tool.execute(args, cwd);
        onEvent({ type: "tool_end", callId: toolCall.id, toolName, result });
        const outputTruncated = result.output.length > 50000
          ? result.output.slice(0, 50000) + "\n...(truncated)"
          : result.output;
        history.push({ role: "tool", tool_call_id: toolCall.id, content: outputTruncated });
      } catch (err) {
        const errorResult: ToolResult = { output: `Execution error: ${(err as Error).message}`, title: `${toolName} (error)`, isError: true };
        onEvent({ type: "tool_end", callId: toolCall.id, toolName, result: errorResult });
        history.push({ role: "tool", tool_call_id: toolCall.id, content: errorResult.output });
      }
    }
  }

  if (turns >= MAX_TURNS) {
    onEvent({ type: "error", message: `Agent loop hit max turns (${MAX_TURNS})` });
  }

  onEvent({ type: "turn_end" });
  return history;
}
