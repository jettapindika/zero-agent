import { Box, Text } from "ink";
import type { ToolCallInfo } from "../providers/PiSessionProvider.js";
import { Spinner } from "./Spinner.js";
import { theme } from "../theme.js";

type Props = {
  toolCall: ToolCallInfo;
};

export function ToolCall({ toolCall }: Props) {
  return (
    <Box flexDirection="column" paddingX={1}>
      <Text color={theme.accent} bold>{"\u2699"} {toolCall.toolName}</Text>
      <Text color={theme.muted}>{JSON.stringify(toolCall.args)}</Text>
      {toolCall.status === "running" ? (
        <Spinner label="running" />
      ) : (
        <Text color={toolCall.status === "error" ? theme.red : theme.green}>
          {toolCall.status === "error" ? "\u2717" : "\u2713"} {toolCall.durationMs ?? 0}ms
        </Text>
      )}
      {toolCall.result ? <Text color={theme.text}>{toolCall.result}</Text> : null}
    </Box>
  );
}
