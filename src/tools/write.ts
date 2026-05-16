import * as fs from "node:fs/promises";
import * as path from "node:path";
import type { ToolDefinition, ToolResult } from "./types.js";

export const writeTool: ToolDefinition = {
  name: "write",
  description: "Write content to a file. Creates the file and parent directories if they don't exist. Overwrites existing content.",
  parameters: {
    type: "object",
    properties: {
      filePath: { type: "string", description: "Absolute path to the file to write" },
      content: { type: "string", description: "The content to write to the file" },
    },
    required: ["filePath", "content"],
  },
  requiresPermission: true,
  async execute(args, _cwd): Promise<ToolResult> {
    const filePath = args.filePath as string;
    const content = args.content as string;

    try {
      await fs.mkdir(path.dirname(filePath), { recursive: true });
      await fs.writeFile(filePath, content, "utf-8");
      const lines = content.split("\n").length;
      return { output: `Wrote ${lines} lines to ${filePath}`, title: `Wrote ${path.basename(filePath)}` };
    } catch (err) {
      return { output: `Error: ${(err as Error).message}`, title: `Failed to write ${filePath}`, isError: true };
    }
  },
};
