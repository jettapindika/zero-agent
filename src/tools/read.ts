import * as fs from "node:fs/promises";
import * as path from "node:path";
import type { ToolDefinition, ToolResult } from "./types.js";

export const readTool: ToolDefinition = {
  name: "read",
  description: "Read a file or list a directory. Returns file contents with line numbers, or directory listing.",
  parameters: {
    type: "object",
    properties: {
      filePath: { type: "string", description: "Absolute path to the file or directory to read" },
      offset: { type: "number", description: "Line number to start from (1-indexed). Default: 1" },
      limit: { type: "number", description: "Maximum lines to read. Default: 2000" },
    },
    required: ["filePath"],
  },
  async execute(args, _cwd): Promise<ToolResult> {
    const filePath = args.filePath as string;
    const offset = (args.offset as number) || 1;
    const limit = (args.limit as number) || 2000;

    try {
      const stat = await fs.stat(filePath);

      if (stat.isDirectory()) {
        const entries = await fs.readdir(filePath, { withFileTypes: true });
        const listing = entries
          .map((e) => e.isDirectory() ? `${e.name}/` : e.name)
          .join("\n");
        return { output: listing, title: `Listed ${entries.length} entries in ${path.basename(filePath)}/` };
      }

      const content = await fs.readFile(filePath, "utf-8");
      const lines = content.split("\n");
      const sliced = lines.slice(offset - 1, offset - 1 + limit);
      const numbered = sliced.map((line, i) => `${offset + i}: ${line}`).join("\n");
      const total = lines.length;

      let title = `Read ${path.basename(filePath)} (${total} lines)`;
      if (offset > 1 || sliced.length < total) {
        title = `Read ${path.basename(filePath)} lines ${offset}-${offset + sliced.length - 1} of ${total}`;
      }

      return { output: numbered, title };
    } catch (err) {
      return { output: `Error: ${(err as Error).message}`, title: `Failed to read ${filePath}`, isError: true };
    }
  },
};
