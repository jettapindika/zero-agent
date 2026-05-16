import { Box, Text } from "ink";
import type { ToolCallInfo } from "../providers/PiSessionProvider.js";
import { ToolCall } from "./ToolCall.js";
import { theme } from "../theme.js";

type Props = {
  toolCall: ToolCallInfo | null;
  toolCalls?: ToolCallInfo[];
};

export function RightPanel({ toolCall, toolCalls = toolCall ? [toolCall] : [] }: Props) {
  const visibleTools = toolCalls.length > 0 ? toolCalls : toolCall ? [toolCall] : [];

  return (
    <Box width={30} flexDirection="column" borderStyle="single" borderColor={theme.border} paddingX={1} paddingY={1}>
      <Text color={theme.muted} bold>Tool Timeline</Text>
      {visibleTools.length > 0 ? (
        <Box marginTop={1} flexDirection="column">
          {visibleTools.map((item) => (
            <Box key={item.id} marginBottom={1}>
              <ToolCall toolCall={item} />
            </Box>
          ))}
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
