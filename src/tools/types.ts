import type { ChatCompletionTool } from "openai/resources/chat/completions.js";

export type ToolDefinition = {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
  execute: (args: Record<string, unknown>, cwd: string) => Promise<ToolResult>;
  requiresPermission?: boolean;
};

export type ToolResult = {
  output: string;
  title: string;
  isError?: boolean;
};

export function toOpenAITool(tool: ToolDefinition): ChatCompletionTool {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.parameters,
    },
  };
}
