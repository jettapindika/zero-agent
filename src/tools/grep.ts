import { execFile } from "node:child_process";
import type { ToolDefinition, ToolResult } from "./types.js";

export const grepTool: ToolDefinition = {
  name: "grep",
  description: "Search file contents using a regex pattern. Returns matching lines with file paths and line numbers.",
  parameters: {
    type: "object",
    properties: {
      pattern: { type: "string", description: "Regex pattern to search for" },
      path: { type: "string", description: "Directory or file to search in (default: cwd)" },
      include: { type: "string", description: "File glob pattern to include (e.g. '*.ts')" },
      limit: { type: "number", description: "Maximum number of matches to return (default: 100)" },
    },
    required: ["pattern"],
  },
  async execute(args, cwd): Promise<ToolResult> {
    const pattern = args.pattern as string;
    const searchPath = (args.path as string) || cwd;
    const include = args.include as string | undefined;
    const limit = (args.limit as number | undefined) ?? 100;

    const rgArgs = ["--line-number", "--no-heading", "--color=never", `--max-count=${limit}`];
    if (include) rgArgs.push("--glob", include);
    rgArgs.push(pattern, searchPath);

    return new Promise((resolve) => {
      execFile("rg", rgArgs, { cwd, timeout: 15000, maxBuffer: 512 * 1024 }, (error, stdout, stderr) => {
        if (error && !stdout) {
          if (error.code === 1) {
            resolve({ output: "No matches found", title: `grep: ${pattern} (0 matches)` });
            return;
          }
          resolve({ output: stderr || error.message, title: `grep failed: ${pattern}`, isError: true });
          return;
        }
        const lines = stdout.trim().split("\n").filter(Boolean);
        resolve({ output: stdout.trim(), title: `grep: ${pattern} (${lines.length} matches)` });
      });
    });
  },
};
