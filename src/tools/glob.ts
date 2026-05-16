import { execFile } from "node:child_process";
import * as fs from "node:fs/promises";
import type { ToolDefinition, ToolResult } from "./types.js";

export const globTool: ToolDefinition = {
  name: "glob",
  description: "Find files matching a glob pattern. Returns matching file paths sorted by modification time.",
  parameters: {
    type: "object",
    properties: {
      pattern: { type: "string", description: "Glob pattern (e.g. '**/*.ts', 'src/**/*.tsx')" },
      path: { type: "string", description: "Base directory to search from (default: cwd)" },
    },
    required: ["pattern"],
  },
  async execute(args, cwd): Promise<ToolResult> {
    const pattern = args.pattern as string;
    const searchPath = (args.path as string) || cwd;

    return new Promise((resolve) => {
      execFile("find", [searchPath, "-name", pattern.replace("**/", ""), "-type", "f"], {
        cwd,
        timeout: 10000,
        maxBuffer: 256 * 1024,
      }, async (error, stdout) => {
        if (error && !stdout) {
          try {
            const rgArgs = ["--files", "--glob", pattern, searchPath];
            execFile("rg", rgArgs, { cwd, timeout: 10000, maxBuffer: 256 * 1024 }, (_err, rgOut) => {
              const files = rgOut.trim().split("\n").filter(Boolean);
              resolve({ output: files.join("\n") || "No files found", title: `glob: ${pattern} (${files.length} files)` });
            });
          } catch {
            resolve({ output: "No files found", title: `glob: ${pattern} (0 files)` });
          }
          return;
        }
        const files = stdout.trim().split("\n").filter(Boolean).slice(0, 100);
        resolve({ output: files.join("\n") || "No files found", title: `glob: ${pattern} (${files.length} files)` });
      });
    });
  },
};

export const lsTool: ToolDefinition = {
  name: "ls",
  description: "List directory contents with file types. Shows files and subdirectories.",
  parameters: {
    type: "object",
    properties: {
      path: { type: "string", description: "Directory path to list (default: cwd)" },
    },
    required: [],
  },
  async execute(args, cwd): Promise<ToolResult> {
    const dirPath = (args.path as string) || cwd;

    try {
      const entries = await fs.readdir(dirPath, { withFileTypes: true });
      const listing = entries
        .sort((a, b) => {
          if (a.isDirectory() && !b.isDirectory()) return -1;
          if (!a.isDirectory() && b.isDirectory()) return 1;
          return a.name.localeCompare(b.name);
        })
        .map((e) => e.isDirectory() ? `${e.name}/` : e.name)
        .join("\n");
      return { output: listing, title: `ls: ${dirPath} (${entries.length} entries)` };
    } catch (err) {
      return { output: `Error: ${(err as Error).message}`, title: `ls failed: ${dirPath}`, isError: true };
    }
  },
};
