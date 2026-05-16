import React from "react";
import { Box, useInput } from "ink";
import { useWindowSize } from "./hooks/useWindowSize.js";
import { usePiSession } from "./hooks/usePiSession.js";
import { useUIState } from "./hooks/useUIState.js";
import { AVAILABLE_MODELS } from "./router.js";
import { Header } from "./components/Header.js";
import { Footer } from "./components/Footer.js";
import { Sidebar } from "./components/Sidebar.js";
import { Chat } from "./components/Chat.js";
import { RightPanel } from "./components/RightPanel.js";
import { Input } from "./components/Input.js";
import { HelpOverlay } from "./components/overlays/HelpOverlay.js";
import { SessionPicker } from "./components/overlays/SessionPicker.js";
import { ModelPicker } from "./components/overlays/ModelPicker.js";

export function App() {
  const { columns, rows } = useWindowSize();
  const session = usePiSession();
  const ui = useUIState();
  const [now, setNow] = React.useState(new Date());
  const [elapsedMs, setElapsedMs] = React.useState(0);
  const streamStartRef = React.useRef<number | null>(null);
  const [ctrlCPressed, setCtrlCPressed] = React.useState(false);

  React.useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(timer);
  }, []);

  React.useEffect(() => {
    if (session.isStreaming) {
      streamStartRef.current = Date.now();
      const timer = setInterval(() => {
        setElapsedMs(Date.now() - (streamStartRef.current ?? Date.now()));
      }, 100);
      return () => clearInterval(timer);
    } else {
      streamStartRef.current = null;
      setElapsedMs(0);
    }
  }, [session.isStreaming]);

  const showSidebar = columns >= 80 && ui.sidebarVisible;
  const showRightPanel = columns > 120 && ui.rightPanelVisible;
  const bodyHeight = rows - 4;

  const recentFiles = React.useMemo(() => {
    const paths: string[] = [];
    for (const msg of session.messages) {
      for (const tc of msg.toolCalls ?? []) {
        const p = (tc.args as Record<string, unknown>).path ?? (tc.args as Record<string, unknown>).filePath;
        if (typeof p === "string" && !paths.includes(p)) paths.push(p);
      }
    }
    return paths.slice(-6);
  }, [session.messages]);

  useInput((input, key) => {
    if (key.escape) {
      if (ui.overlay !== "none") {
        ui.setOverlay("none");
        return;
      }
      if (session.isStreaming) {
        session.abort();
        return;
      }
    }

    if (key.ctrl && input === "b") { ui.toggleSidebar(); return; }
    if (key.ctrl && input === "\\") { ui.toggleRightPanel(); return; }
    if (key.ctrl && input === "t") { ui.toggleMode(); return; }
    if (key.ctrl && input === "n") { void session.newSession(); return; }
    if (key.ctrl && input === "p") { ui.setOverlay(ui.overlay === "session-picker" ? "none" : "session-picker"); return; }
    if (key.ctrl && input === "l") { ui.setOverlay(ui.overlay === "model-picker" ? "none" : "model-picker"); return; }

    if (key.ctrl && input === "c") {
      if (ctrlCPressed) { process.exit(0); }
      setCtrlCPressed(true);
      setTimeout(() => setCtrlCPressed(false), 500);
      return;
    }

    if (key.tab) { ui.cycleFocus(); return; }

    if (ui.focus === "chat") {
      if (key.pageDown) { ui.setScrollOffset(ui.scrollOffset + Math.floor(bodyHeight / 2)); return; }
      if (key.pageUp) { ui.setScrollOffset(Math.max(0, ui.scrollOffset - Math.floor(bodyHeight / 2))); return; }
      if (input === "G" && key.shift) { ui.setScrollOffset(999999); return; }
    }
  });

  const tokenCount = React.useMemo(() => {
    let chars = 0;
    for (const msg of session.messages) { chars += msg.content.length; }
    return Math.round(chars / 4);
  }, [session.messages]);

  const estimatedCost = (tokenCount / 1_000_000) * 9;

  if (ui.overlay === "help") {
    return <HelpOverlay onClose={() => ui.setOverlay("none")} />;
  }

  if (ui.overlay === "session-picker") {
    return (
      <SessionPicker
        sessions={session.sessions}
        onSelect={(id) => void session.switchSession(id)}
        onClose={() => ui.setOverlay("none")}
      />
    );
  }

  if (ui.overlay === "model-picker") {
    return (
      <ModelPicker
        models={AVAILABLE_MODELS.map((m) => ({ label: m.label, value: m.value }))}
        onSelect={(modelId) => session.setModel(modelId)}
        onClose={() => ui.setOverlay("none")}
      />
    );
  }

  return (
    <Box flexDirection="column" height={rows}>
      <Header model={session.model} mode={ui.mode} now={now} />
      <Box flexGrow={1} flexDirection="row" height={bodyHeight}>
        {showSidebar && (
          <Sidebar
            sessions={session.sessions}
            activeSessionId={session.activeSession?.id ?? null}
            recentFiles={recentFiles}
            cwd={process.cwd()}
            hasAgentsFile={true}
            focused={ui.focus === "sidebar"}
          />
        )}
        <Chat
          messages={session.messages}
          scrollOffset={ui.scrollOffset}
          isStreaming={session.isStreaming}
          height={bodyHeight}
        />
        {showRightPanel && <RightPanel toolCall={session.activeToolCall} />}
      </Box>
      <Input
        onSubmit={async (text) => { ui.setScrollOffset(999999); await session.prompt(text); }}
        onAbort={() => session.abort()}
        isStreaming={session.isStreaming}
      />
      <Footer
        tokens={tokenCount}
        estimatedCost={estimatedCost}
        latencyMs={session.latencyMs}
        isStreaming={session.isStreaming}
        elapsedMs={elapsedMs}
      />
    </Box>
  );
}
