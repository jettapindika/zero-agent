import { Box, Text } from "ink";
import type { Message as MessageType } from "../providers/PiSessionProvider.js";
import { theme } from "../theme.js";

type Props = {
  message: MessageType;
  isStreaming?: boolean;
};

export function Message({ message, isStreaming = false }: Props) {
  const isUser = message.role === "user";
  const borderColor = isUser ? theme.userBg : theme.borderHi;
  const label = isUser ? "you" : message.role;

  return (
    <Box flexDirection="column" borderStyle="round" borderColor={borderColor} paddingX={1} marginBottom={1}>
      <Text color={isUser ? theme.accent : theme.muted} bold>{label}</Text>
      <Text color={theme.text}>
        {message.content}
        {isStreaming && message.role === "assistant" ? " \u2588" : ""}
      </Text>
      {message.toolCalls?.map((tc) => (
        <Text key={tc.id} color={tc.status === "error" ? theme.red : theme.muted}>
          {tc.status === "running" ? "\u2699" : "\u2713"} {tc.toolName}
          {tc.durationMs != null ? `  ${tc.durationMs}ms` : ""}
        </Text>
      ))}
    </Box>
  );
}
