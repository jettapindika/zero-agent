import { Box, Text } from "ink";
import { theme } from "../theme.js";

type Props = {
  model: string;
  mode: "build" | "plan";
  isStreaming: boolean;
  now?: Date;
};

export function Header({ model, mode, isStreaming, now = new Date() }: Props) {
  const time = now.toISOString().slice(11, 16);

  return (
    <Box justifyContent="space-between" paddingX={2} paddingY={0}>
      <Box gap={1}>
        <Text color={theme.accent} bold>opencode-pi</Text>
        <Text color={theme.dim}>{"\u2502"}</Text>
        <Text color={theme.text}>{model}</Text>
        <Text color={theme.dim}>{"\u2502"}</Text>
        <Text color={mode === "build" ? theme.green : theme.yellow} bold>{mode}</Text>
        {isStreaming && (
          <>
            <Text color={theme.dim}>{"\u2502"}</Text>
            <Text color={theme.yellow}>{"\u25CF"} streaming</Text>
          </>
        )}
      </Box>
      <Text color={theme.muted}>{time}</Text>
    </Box>
  );
}
