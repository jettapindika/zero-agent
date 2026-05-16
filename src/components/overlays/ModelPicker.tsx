import { Box, Text } from "ink";
import SelectInput from "ink-select-input";
import { theme } from "../../theme.js";

type Props = {
  models: Array<{ label: string; value: string }>;
  onSelect: (modelId: string) => void;
  onClose: () => void;
};

export function ModelPicker({ models, onSelect, onClose }: Props) {
  return (
    <Box flexDirection="column" borderStyle="double" borderColor={theme.accent} paddingX={2} paddingY={1}>
      <Text color={theme.accent} bold>Select Model</Text>
      <SelectInput
        items={models}
        onSelect={(item) => { onSelect(item.value); onClose(); }}
      />
      <Text color={theme.dim}>Esc to close</Text>
    </Box>
  );
}
