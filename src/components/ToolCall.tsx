import { Box, Text } from "ink";
import type { ToolCallInfo } from "../providers/PiSessionProvider.js";
import { Spinner } from "./Spinner.js";
import { theme, truncateText } from "../theme.js";

type Props = {
  toolCall: ToolCallInfo;
};

const toolIcons: Record<string, string> = {
  bash: "$",
  read: "\u2192",
  write: "\u2190",
  edit: "\u2190",
  grep: "\u2731",
  glob: "\u2731",
  find: "\u2731",
  ls: "\u2192",
};

function formatArgs(args: Record<string, unknown>): string[] {
  return Object.entries(args).map(([key, val]) => {
    const strVal = typeof val === "string" ? val : JSON.stringify(val);
    return `${key}: ${truncateText(strVal, 40)}`;
  });
}

function formatResult(_toolName: string, result: string): string[] {
  const lines = result.split("\n");
  if (lines.length > 15) {
    return [...lines.slice(0, 12), `... (${lines.length - 12} more lines)`];
  }
  return lines;
}

export function ToolCall({ toolCall }: Props) {
  const icon = toolIcons[toolCall.toolName] ?? "\u2699";

  return (
    <Box flexDirection="column" paddingX={1} gap={1}>
      <Box gap={1}>
        <Text color={theme.accent} bold>{icon}</Text>
        <Text color={theme.accent} bold>{toolCall.toolName}</Text>
        {toolCall.status === "running" && <Spinner />}
        {toolCall.status === "done" && (
          <Text color={theme.green}>{"\u2713"} {toolCall.durationMs ?? 0}ms</Text>
        )}
        {toolCall.status === "error" && (
          <Text color={theme.red}>{"\u2717"} error</Text>
        )}
      </Box>

      <Box flexDirection="column" paddingLeft={2}>
        {formatArgs(toolCall.args).map((line, idx) => (
          <Text key={idx} color={theme.muted}>{line}</Text>
        ))}
      </Box>

      {toolCall.result && (
        <Box flexDirection="column" paddingLeft={2} marginTop={1}>
          {formatResult(toolCall.toolName, toolCall.result).map((line, idx) => (
            <Text key={idx} color={toolCall.isError ? theme.red : theme.text} wrap="truncate">{line}</Text>
          ))}
        </Box>
      )}
    </Box>
  );
}
