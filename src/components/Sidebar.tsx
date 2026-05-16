import { Box, Text } from "ink";
import type { SessionSummary } from "../providers/PiSessionProvider.js";
import { theme, truncateText } from "../theme.js";

type Props = {
  sessions: SessionSummary[];
  activeSessionId: string | null;
  recentFiles: string[];
  cwd: string;
  hasAgentsFile: boolean;
  focused: boolean;
};

export function Sidebar({ sessions, activeSessionId, recentFiles, cwd, hasAgentsFile, focused }: Props) {
  return (
    <Box width={22} flexDirection="column" borderStyle="single" borderColor={focused ? theme.accent : theme.border} paddingX={1}>
      <Text color={theme.muted} bold>{"\u25B8"} Sessions</Text>
      {sessions.map((session) => (
        <Text key={session.id} color={session.id === activeSessionId ? theme.accent : theme.text}>
          {session.id === activeSessionId ? "\u25CF " : "  "}{truncateText(session.title, 16)}
        </Text>
      ))}
      <Text color={theme.muted} bold>{"\u25B8"} Files</Text>
      {recentFiles.slice(0, 6).map((filePath, idx) => (
        <Text key={`${filePath}-${idx}`} color={theme.text}>  {truncateText(filePath, 18)}</Text>
      ))}
      <Text color={theme.muted} bold>{"\u25B8"} Context</Text>
      <Text color={theme.dim}>  cwd: {truncateText(cwd, 16)}</Text>
      <Text color={theme.dim}>  AGENTS.md: {hasAgentsFile ? "yes" : "no"}</Text>
    </Box>
  );
}
