import React from "react";
import { Box, Text, useInput } from "ink";
import TextInput from "ink-text-input";
import { theme } from "../theme.js";

type Props = {
  onSubmit: (value: string) => Promise<void>;
  onAbort: () => void;
  isStreaming: boolean;
};

export function Input({ onSubmit, onAbort, isStreaming }: Props) {
  const [value, setValue] = React.useState("");

  useInput((_input, key) => {
    if (key.escape && isStreaming) {
      onAbort();
    }
  });

  return (
    <Box paddingX={1}>
      <Text color={theme.accent} bold>{"> "}</Text>
      <TextInput
        value={value}
        onChange={setValue}
        onSubmit={async (submitted) => {
          if (!submitted.trim()) return;
          await onSubmit(submitted);
          setValue("");
        }}
      />
      <Text color={theme.dim}> Shift+↵ nl</Text>
    </Box>
  );
}
