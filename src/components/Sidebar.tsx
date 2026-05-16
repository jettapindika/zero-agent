import React from "react";
import { Box, Text, useInput } from "ink";
import type { SessionSummary } from "../providers/PiSessionProvider.js";
import { theme, truncateText } from "../theme.js";

type Props = {
  sessions: SessionSummary[];
  activeSessionId: string | null;
  recentFiles: string[];
  cwd: string;
  hasAgentsFile: boolean;
  focused: boolean;
  onSwitchSession: (id: string) => void;
};

export function Sidebar({ sessions, activeSessionId, recentFiles, cwd, hasAgentsFile, focused, onSwitchSession }: Props) {
  const [selectedIdx, setSelectedIdx] = React.useState(0);

  const maxIdx = sessions.length - 1;

  useInput((input, key) => {
    if (!focused) return;

    if (input === "j" || key.downArrow) {
      setSelectedIdx((i) => Math.min(i + 1, maxIdx));
      return;
    }
    if (input === "k" || key.upArrow) {
      setSelectedIdx((i) => Math.max(i - 1, 0));
      return;
    }
    if (key.return) {
      const session = sessions[selectedIdx];
      if (session) onSwitchSession(session.id);
      return;
    }
  });

  React.useEffect(() => {
    if (selectedIdx > maxIdx) setSelectedIdx(Math.max(0, maxIdx));
  }, [sessions.length, maxIdx, selectedIdx]);

  return (
    <Box width={22} flexDirection="column" borderStyle="single" borderColor={focused ? theme.accent : theme.border} paddingX={1}>
      <Text color={theme.muted} bold>{"\u25B8"} Sessions</Text>
      {sessions.length === 0 && <Text color={theme.dim}>  No sessions</Text>}
      {sessions.map((session, idx) => {
        const isActive = session.id === activeSessionId;
        const isSelected = focused && idx === selectedIdx;
        return (
          <Text
            key={session.id}
            backgroundColor={isSelected ? theme.accent : undefined}
            color={isSelected ? theme.bg : isActive ? theme.accent : theme.text}
          >
            {isActive ? "\u25CF " : "  "}{truncateText(session.title, 16)}
          </Text>
        );
      })}

      <Text color={theme.muted} bold>{"\n\u25B8"} Files</Text>
      {recentFiles.length === 0 && <Text color={theme.dim}>  No files yet</Text>}
      {recentFiles.slice(0, 6).map((filePath, idx) => (
        <Text key={`${filePath}-${idx}`} color={theme.text}>  {truncateText(filePath, 18)}</Text>
      ))}

      <Text color={theme.muted} bold>{"\n\u25B8"} Context</Text>
      <Text color={theme.dim}>  {truncateText(cwd, 18)}</Text>
      <Text color={theme.dim}>  AGENTS.md: {hasAgentsFile ? <Text color={theme.green}>yes</Text> : "no"}</Text>

      {focused && (
        <Box marginTop={1}>
          <Text color={theme.dim}>j/k nav  ↵ select</Text>
        </Box>
      )}
    </Box>
  );
}
