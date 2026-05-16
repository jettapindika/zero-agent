import { Box, Text } from "ink";
import type { ToolCallInfo } from "../providers/PiSessionProvider.js";
import { ToolCall } from "./ToolCall.js";
import { theme } from "../theme.js";

type Props = {
  toolCall: ToolCallInfo | null;
};

export function RightPanel({ toolCall }: Props) {
  return (
    <Box width={30} flexDirection="column" borderStyle="single" borderColor={theme.border} paddingX={1}>
      <Text color={theme.muted} bold>Active Tool</Text>
      {toolCall ? <ToolCall toolCall={toolCall} /> : <Text color={theme.dim}>No active tool call</Text>}
    </Box>
  );
}
