import React from "react";
import { Box, Text, useInput } from "ink";
import { theme } from "../theme.js";

type Props = {
  onSubmit: (value: string) => Promise<void>;
  onAbort: () => void;
  isStreaming: boolean;
  focused: boolean;
};

export function Input({ onSubmit, onAbort, isStreaming, focused }: Props) {
  const [value, setValue] = React.useState("");

  useInput((input, key) => {
    if (!focused) return;

    if (key.escape) {
      if (isStreaming) onAbort();
      return;
    }

    if (key.return) {
      if (!value.trim()) return;
      const text = value;
      setValue("");
      void onSubmit(text);
      return;
    }

    if (key.backspace || key.delete) {
      setValue((prev) => prev.slice(0, -1));
      return;
    }

    if (key.ctrl || key.meta) return;

    if (input) {
      setValue((prev) => prev + input);
    }
  });

  return (
    <Box paddingX={1}>
      <Text color={theme.accent} bold>{"> "}</Text>
      <Text color={theme.text}>{value}</Text>
      {focused && <Text color={theme.accent}>{"\u2588"}</Text>}
      {!focused && <Text color={theme.dim}> (Tab to focus)</Text>}
    </Box>
  );
}
