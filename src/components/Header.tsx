import { Box, Text } from "ink";
import { theme, truncateText } from "../theme.js";

type Props = {
  model: string;
  mode: "build" | "plan";
  now?: Date;
};

export function Header({ model, mode, now = new Date() }: Props) {
  const time = now.toISOString().slice(11, 16);

  return (
    <Box justifyContent="space-between" paddingX={1}>
      <Box>
        <Text color={theme.accent} bold>opencode-pi</Text>
        <Text color={theme.dim}> │ </Text>
        <Text color={theme.text}>{truncateText(model, 28)}</Text>
        <Text color={theme.dim}> │ </Text>
        <Text color={mode === "build" ? theme.green : theme.yellow}>{mode}</Text>
      </Box>
      <Text color={theme.muted}>{time}</Text>
    </Box>
  );
}
