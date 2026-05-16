import { Box } from "ink";
import type { Message as MessageType } from "../providers/PiSessionProvider.js";
import { Message } from "./Message.js";
import { theme } from "../theme.js";

type Props = {
  messages: MessageType[];
  scrollOffset: number;
  isStreaming: boolean;
  height: number;
};

export function Chat({ messages, scrollOffset, isStreaming, height }: Props) {
  return (
    <Box
      flexDirection="column"
      flexGrow={1}
      height={height}
      overflow="hidden"
      borderStyle="single"
      borderColor={theme.border}
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
      </Box>
    </Box>
  );
}
