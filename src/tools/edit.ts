import * as fs from "node:fs/promises";
import * as path from "node:path";
import type { ToolDefinition, ToolResult } from "./types.js";

export const editTool: ToolDefinition = {
  name: "edit",
  description: "Edit a file by replacing an exact string match with new content. The oldString must match exactly (including whitespace and indentation).",
  parameters: {
    type: "object",
    properties: {
      filePath: { type: "string", description: "Absolute path to the file to modify" },
      oldString: { type: "string", description: "The exact text to find and replace" },
      newString: { type: "string", description: "The replacement text" },
      replaceAll: { type: "boolean", description: "Replace all occurrences (default: false)" },
    },
    required: ["filePath", "oldString", "newString"],
  },
  requiresPermission: true,
  async execute(args, _cwd): Promise<ToolResult> {
    const filePath = args.filePath as string;
    const oldString = args.oldString as string;
    const newString = args.newString as string;
    const replaceAll = (args.replaceAll as boolean) ?? false;

    try {
      const content = await fs.readFile(filePath, "utf-8");

      if (!content.includes(oldString)) {
        return { output: "Error: oldString not found in file content", title: `Edit failed: ${path.basename(filePath)}`, isError: true };
      }

      const occurrences = content.split(oldString).length - 1;
      if (occurrences > 1 && !replaceAll) {
        return {
          output: `Error: Found ${occurrences} matches for oldString. Use replaceAll: true or provide more context to make it unique.`,
          title: `Edit failed: ${path.basename(filePath)}`,
          isError: true,
        };
      }

      const updated = replaceAll
        ? content.replaceAll(oldString, newString)
        : content.replace(oldString, newString);

      await fs.writeFile(filePath, updated, "utf-8");
      const replacements = replaceAll ? occurrences : 1;
      return { output: `Replaced ${replacements} occurrence(s) in ${filePath}`, title: `Edited ${path.basename(filePath)}` };
    } catch (err) {
      return { output: `Error: ${(err as Error).message}`, title: `Failed to edit ${filePath}`, isError: true };
    }
  },
};
