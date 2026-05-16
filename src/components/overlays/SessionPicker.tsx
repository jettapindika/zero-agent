import { Box, Text } from "ink";
import SelectInput from "ink-select-input";
import type { SessionSummary } from "../../providers/PiSessionProvider.js";
import { theme } from "../../theme.js";

type Props = {
  sessions: SessionSummary[];
  onSelect: (id: string) => void;
  onClose: () => void;
};

export function SessionPicker({ sessions, onSelect, onClose }: Props) {
  const items = sessions.map((s) => ({ label: s.title, value: s.id }));

  return (
    <Box flexDirection="column" borderStyle="double" borderColor={theme.accent} paddingX={2} paddingY={1}>
      <Text color={theme.accent} bold>Select Session</Text>
      <SelectInput
        items={items}
        onSelect={(item) => { onSelect(item.value); onClose(); }}
      />
      <Text color={theme.dim}>Esc to close</Text>
    </Box>
  );
}
