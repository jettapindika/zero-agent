import { execFile } from "node:child_process";
import type { ToolDefinition, ToolResult } from "./types.js";

export const bashTool: ToolDefinition = {
  name: "bash",
  description: "Execute a bash command. Returns stdout and stderr. Commands run in the working directory.",
  parameters: {
    type: "object",
    properties: {
      command: { type: "string", description: "The bash command to execute" },
      timeout: { type: "number", description: "Timeout in milliseconds (default: 30000)" },
      workdir: { type: "string", description: "Working directory for the command" },
    },
    required: ["command"],
  },
  requiresPermission: true,
  async execute(args, cwd): Promise<ToolResult> {
    const command = args.command as string;
    const timeout = (args.timeout as number) || 30000;
    const workdir = (args.workdir as string) || cwd;

    return new Promise((resolve) => {
      const proc = execFile("bash", ["-c", command], {
        cwd: workdir,
        timeout,
        maxBuffer: 1024 * 1024,
        env: { ...process.env, TERM: "dumb", NO_COLOR: "1" },
      }, (error, stdout, stderr) => {
        const output = [stdout, stderr].filter(Boolean).join("\n").trim();
        const exitCode = error?.code ?? (proc.exitCode ?? 0);

        if (error && error.killed) {
          resolve({ output: `Command timed out after ${timeout}ms\n${output}`, title: `$ ${command} (timeout)`, isError: true });
          return;
        }

        const title = exitCode === 0 ? `$ ${command}` : `$ ${command} (exit ${exitCode})`;
        resolve({ output: output || "(no output)", title, isError: exitCode !== 0 });
      });
    });
  },
};
