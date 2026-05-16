import { Box, Text } from "ink";
import type { Message as MessageType } from "../providers/PiSessionProvider.js";
import { theme } from "../theme.js";

type Props = {
  message: MessageType;
  isStreaming?: boolean;
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

function getToolIcon(toolName: string): string {
  return toolIcons[toolName] ?? "\u2699";
}

export function Message({ message, isStreaming = false }: Props) {
  const isUser = message.role === "user";
  const borderColor = isUser ? theme.accent : theme.borderHi;

  return (
    <Box flexDirection="column" marginTop={1}>
      <Box borderStyle="bold" borderLeft borderRight={false} borderTop={false} borderBottom={false} borderColor={borderColor} paddingLeft={2} paddingY={1} flexDirection="column">
        <Text color={isUser ? theme.accent : theme.muted} bold>
          {isUser ? "\u25A3 you" : "\u25A3 assistant"}
        </Text>
        <Box marginTop={1} flexDirection="column">
          <Text color={theme.text} wrap="wrap">
            {message.content}
            {isStreaming && message.role === "assistant" ? <Text color={theme.accent}>{" \u2588"}</Text> : ""}
          </Text>
        </Box>
        {message.toolCalls && message.toolCalls.length > 0 && (
          <Box marginTop={1} flexDirection="column">
            {message.toolCalls.map((tc) => (
              <Text key={tc.id} color={tc.status === "error" ? theme.red : tc.status === "running" ? theme.yellow : theme.muted}>
                <Text color={tc.status === "error" ? theme.red : theme.dim}>{getToolIcon(tc.toolName)} </Text>
                <Text>{tc.toolName}</Text>
                {tc.args && Object.keys(tc.args).length > 0 && (
                  <Text color={theme.dim}> {Object.values(tc.args)[0] as string}</Text>
                )}
                {tc.status === "done" && tc.durationMs != null && (
                  <Text color={theme.dim}> {tc.durationMs}ms</Text>
                )}
                {tc.status === "running" && <Text color={theme.yellow}> ...</Text>}
                {tc.status === "error" && <Text color={theme.red}> failed</Text>}
              </Text>
            ))}
          </Box>
        )}
      </Box>
    </Box>
  );
}
