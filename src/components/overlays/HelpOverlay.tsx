import { Box, Text } from "ink";
import { theme } from "../../theme.js";

type Props = {
  onClose: () => void;
};

const shortcuts = [
  ["Ctrl+B", "Toggle sidebar"],
  ["Ctrl+\\", "Toggle right panel"],
  ["Ctrl+T", "Switch build/plan mode"],
  ["Tab", "Cycle focus"],
  ["Ctrl+N", "New session"],
  ["Ctrl+P", "Session picker"],
  ["Ctrl+L", "Model picker"],
  ["Escape", "Abort generation"],
  ["Ctrl+C x2", "Quit"],
  ["PageUp/Down", "Scroll chat"],
  ["Shift+G", "Jump to bottom"],
  ["j/k", "Navigate sidebar"],
  ["Shift+Enter", "Newline in input"],
] as const;

export function HelpOverlay({ onClose }: Props) {
  void onClose;
  return (
    <Box flexDirection="column" borderStyle="double" borderColor={theme.accent} paddingX={2} paddingY={1}>
      <Text color={theme.accent} bold>Keyboard Shortcuts</Text>
      <Text> </Text>
      {shortcuts.map(([key, desc]) => (
        <Box key={key}>
          <Text color={theme.text}>{key.padEnd(16)}</Text>
          <Text color={theme.muted}>{desc}</Text>
        </Box>
      ))}
      <Text> </Text>
      <Text color={theme.dim}>Press Esc to close</Text>
    </Box>
  );
}
