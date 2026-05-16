import { Box, Text } from "ink";
import type { ToolCallInfo } from "../providers/PiSessionProvider.js";
import { ToolCall } from "./ToolCall.js";
import { theme } from "../theme.js";

type Props = {
  toolCall: ToolCallInfo | null;
};

export function RightPanel({ toolCall }: Props) {
  return (
    <Box width={30} flexDirection="column" borderStyle="single" borderColor={theme.border} paddingX={1} paddingY={1}>
      <Text color={theme.muted} bold>Active Tool</Text>
      {toolCall ? (
        <Box marginTop={1}>
          <ToolCall toolCall={toolCall} />
        </Box>
      ) : (
        <Box marginTop={1} flexDirection="column">
          <Text color={theme.dim}>No active tool</Text>
          <Text color={theme.dim}>Tool details appear here</Text>
          <Text color={theme.dim}>during execution</Text>
        </Box>
      )}
    </Box>
  );
}
