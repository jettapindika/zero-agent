import { Box, Text } from "ink";
import { theme } from "../theme.js";

type Props = {
  tokens: number;
  estimatedCost: number;
  latencyMs: number | null;
  isStreaming: boolean;
  elapsedMs: number;
  cwd: string;
};

export function Footer({ tokens, estimatedCost, latencyMs, isStreaming, elapsedMs, cwd }: Props) {
  const latencyText = isStreaming
    ? `${(elapsedMs / 1000).toFixed(1)}s`
    : latencyMs !== null ? `${(latencyMs / 1000).toFixed(1)}s` : "-";

  const shortCwd = cwd.replace(/^\/Users\/[^/]+/, "~");

  return (
    <Box justifyContent="space-between" paddingX={2}>
      <Box gap={2}>
        <Text color={theme.dim}>{shortCwd}</Text>
        <Text color={theme.muted}>
          {tokens > 0 && <Text>{tokens} tok</Text>}
          {tokens > 0 && <Text color={theme.dim}> {"\u00B7"} </Text>}
          {tokens > 0 && <Text>${estimatedCost.toFixed(4)}</Text>}
          {tokens > 0 && <Text color={theme.dim}> {"\u00B7"} </Text>}
          <Text>{isStreaming ? "elapsed" : "latency"}: {latencyText}</Text>
        </Text>
      </Box>
      <Box gap={2}>
        <Text color={theme.dim}>? help</Text>
        <Text color={theme.dim}>^X cmd</Text>
        <Text color={theme.dim}>^C^C quit</Text>
      </Box>
    </Box>
  );
}
