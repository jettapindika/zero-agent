import type { ToolDefinition } from "./types.js";
import { toOpenAITool } from "./types.js";
import { readTool } from "./read.js";
import { writeTool } from "./write.js";
import { editTool } from "./edit.js";
import { bashTool } from "./bash.js";
import { grepTool } from "./grep.js";
import { globTool, lsTool } from "./glob.js";

export const ALL_TOOLS: ToolDefinition[] = [
  readTool,
  writeTool,
  editTool,
  bashTool,
  grepTool,
  globTool,
  lsTool,
];

export const TOOL_MAP = new Map<string, ToolDefinition>(
  ALL_TOOLS.map((t) => [t.name, t])
);

export const OPENAI_TOOLS = ALL_TOOLS.map(toOpenAITool);

export { toOpenAITool } from "./types.js";
export type { ToolDefinition, ToolResult } from "./types.js";
