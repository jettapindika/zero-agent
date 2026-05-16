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
  const [lines, setLines] = React.useState<string[]>([""]);
  const [cursorLine, setCursorLine] = React.useState(0);
  const [cursorCol, setCursorCol] = React.useState(0);
  const [history, setHistory] = React.useState<string[]>([]);
  const [historyIdx, setHistoryIdx] = React.useState(-1);

  const currentText = lines.join("\n");

  useInput((input, key) => {
    if (!focused) return;

    if (key.escape) {
      if (isStreaming) onAbort();
      return;
    }

    if (key.return && key.shift) {
      setLines((prev) => {
        const next = [...prev];
        const line = next[cursorLine] ?? "";
        const before = line.slice(0, cursorCol);
        const after = line.slice(cursorCol);
        next.splice(cursorLine, 1, before, after);
        return next;
      });
      setCursorLine((l) => l + 1);
      setCursorCol(0);
      return;
    }

    if (key.return) {
      const text = currentText.trim();
      if (!text) return;
      setHistory((prev) => [...prev, text]);
      setHistoryIdx(-1);
      setLines([""]);
      setCursorLine(0);
      setCursorCol(0);
      void onSubmit(text);
      return;
    }

    if (key.backspace || key.delete) {
      setLines((prev) => {
        const next = [...prev];
        const line = next[cursorLine] ?? "";
        if (cursorCol > 0) {
          next[cursorLine] = line.slice(0, cursorCol - 1) + line.slice(cursorCol);
          setCursorCol((c) => c - 1);
        } else if (cursorLine > 0) {
          const prevLine = next[cursorLine - 1] ?? "";
          const newCol = prevLine.length;
          next[cursorLine - 1] = prevLine + line;
          next.splice(cursorLine, 1);
          setCursorLine((l) => l - 1);
          setCursorCol(newCol);
        }
        return next;
      });
      return;
    }

    if (key.leftArrow) {
      if (cursorCol > 0) {
        setCursorCol((c) => c - 1);
      } else if (cursorLine > 0) {
        setCursorLine((l) => l - 1);
        setCursorCol((lines[cursorLine - 1] ?? "").length);
      }
      return;
    }

    if (key.rightArrow) {
      const line = lines[cursorLine] ?? "";
      if (cursorCol < line.length) {
        setCursorCol((c) => c + 1);
      } else if (cursorLine < lines.length - 1) {
        setCursorLine((l) => l + 1);
        setCursorCol(0);
      }
      return;
    }

    if (key.upArrow) {
      if (lines.length === 1 && history.length > 0) {
        const idx = historyIdx === -1 ? history.length - 1 : Math.max(0, historyIdx - 1);
        setHistoryIdx(idx);
        const entry = history[idx] ?? "";
        setLines(entry.split("\n"));
        setCursorLine(0);
        setCursorCol(entry.split("\n")[0]?.length ?? 0);
        return;
      }
      if (cursorLine > 0) {
        setCursorLine((l) => l - 1);
        setCursorCol((c) => Math.min(c, (lines[cursorLine - 1] ?? "").length));
      }
      return;
    }

    if (key.downArrow) {
      if (lines.length === 1 && historyIdx >= 0) {
        const idx = historyIdx + 1;
        if (idx >= history.length) {
          setHistoryIdx(-1);
          setLines([""]);
          setCursorCol(0);
        } else {
          setHistoryIdx(idx);
          const entry = history[idx] ?? "";
          setLines(entry.split("\n"));
          setCursorCol(entry.split("\n")[0]?.length ?? 0);
        }
        return;
      }
      if (cursorLine < lines.length - 1) {
        setCursorLine((l) => l + 1);
        setCursorCol((c) => Math.min(c, (lines[cursorLine + 1] ?? "").length));
      }
      return;
    }

    if (key.home) { setCursorCol(0); return; }
    if (key.end) { setCursorCol((lines[cursorLine] ?? "").length); return; }

    if (key.ctrl && input === "a") { setCursorCol(0); return; }
    if (key.ctrl && input === "e") { setCursorCol((lines[cursorLine] ?? "").length); return; }
    if (key.ctrl && input === "k") {
      setLines((prev) => {
        const next = [...prev];
        next[cursorLine] = (next[cursorLine] ?? "").slice(0, cursorCol);
        return next;
      });
      return;
    }
    if (key.ctrl && input === "u") {
      setLines((prev) => {
        const next = [...prev];
        next[cursorLine] = (next[cursorLine] ?? "").slice(cursorCol);
        return next;
      });
      setCursorCol(0);
      return;
    }

    if (key.ctrl || key.meta) return;

    if (input) {
      setLines((prev) => {
        const next = [...prev];
        const line = next[cursorLine] ?? "";
        next[cursorLine] = line.slice(0, cursorCol) + input + line.slice(cursorCol);
        return next;
      });
      setCursorCol((c) => c + input.length);
    }
  });

  const renderLine = (line: string, lineIdx: number) => {
    if (!focused || lineIdx !== cursorLine) {
      return <Text key={lineIdx} color={theme.text}>{line || " "}</Text>;
    }
    const before = line.slice(0, cursorCol);
    const cursor = line[cursorCol] ?? " ";
    const after = line.slice(cursorCol + 1);
    return (
      <Text key={lineIdx}>
        <Text color={theme.text}>{before}</Text>
        <Text backgroundColor={theme.accent} color={theme.bg}>{cursor}</Text>
        <Text color={theme.text}>{after}</Text>
      </Text>
    );
  };

  return (
    <Box flexDirection="column" paddingX={1} borderStyle="single" borderColor={focused ? theme.accent : theme.border}>
      <Box>
        <Text color={theme.accent} bold>{"> "}</Text>
        <Box flexDirection="column">
          {lines.map((line, idx) => renderLine(line, idx))}
        </Box>
      </Box>
      {lines.length > 1 && (
        <Text color={theme.dim}>{lines.length} lines</Text>
      )}
    </Box>
  );
}
