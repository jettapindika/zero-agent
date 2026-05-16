import { Box, Text } from "ink";
import { theme } from "../theme.js";

type Props = {
  tokens: number;
  estimatedCost: number;
  latencyMs: number | null;
  isStreaming: boolean;
  elapsedMs: number;
};

export function Footer({ tokens, estimatedCost, latencyMs, isStreaming, elapsedMs }: Props) {
  const latencyText = isStreaming
    ? `elapsed: ${(elapsedMs / 1000).toFixed(1)}s`
    : `latency: ${((latencyMs ?? 0) / 1000).toFixed(1)}s`;

  return (
    <Box justifyContent="space-between" paddingX={1}>
      <Text color={theme.muted}>{`tokens: ${tokens}  $${estimatedCost.toFixed(4)}  ${latencyText}`}</Text>
      <Text color={theme.dim}>?: help  Ctrl+X: cmd</Text>
    </Box>
  );
}
