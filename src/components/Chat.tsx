import { Box, Text } from "ink";
import type { Message as MessageType } from "../providers/PiSessionProvider.js";
import { Message } from "./Message.js";
import { Spinner } from "./Spinner.js";
import { theme } from "../theme.js";

type Props = {
  messages: MessageType[];
  scrollOffset: number;
  isStreaming: boolean;
  height: number;
  focused: boolean;
};

export function Chat({ messages, scrollOffset, isStreaming, height, focused }: Props) {
  const lastMsg = messages[messages.length - 1];
  const showThinking = isStreaming && lastMsg?.role === "assistant" && lastMsg.content === "";

  if (messages.length === 0) {
    return (
      <Box
        flexDirection="column"
        flexGrow={1}
        height={height}
        justifyContent="center"
        alignItems="center"
        borderStyle="single"
        borderColor={focused ? theme.accent : theme.border}
      >
        <Text color={theme.muted}>No messages yet</Text>
        <Text color={theme.dim}>Type below and press Enter to start</Text>
      </Box>
    );
  }

  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      height={height}
      overflow="hidden"
      borderStyle="single"
      borderColor={focused ? theme.accent : theme.border}
      paddingX={1}
    >
      <Box flexDirection="column" marginTop={-Math.max(0, scrollOffset)}>
        {messages.map((message, index) => (
          <Message
            key={message.id}
            message={message}
            isStreaming={isStreaming && index === messages.length - 1 && message.role === "assistant"}
          />
        ))}
        {showThinking && (
          <Box marginTop={1} paddingLeft={3}>
            <Spinner label="thinking..." />
          </Box>
        )}
      </Box>
      {scrollOffset > 0 && (
        <Box position="absolute" marginTop={0}>
          <Text color={theme.dim}> \u2191 scroll up ({scrollOffset} lines) </Text>
        </Box>
      )}
    </Box>
  );
}
