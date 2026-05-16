import React, { createContext, useCallback, useMemo, useState } from "react";

export type AgentMode = "build" | "plan";
export type FocusTarget = "input" | "sidebar" | "chat";
export type OverlayState = "none" | "session-picker" | "model-picker" | "help";

export type UIStateValue = {
  sidebarVisible: boolean;
  rightPanelVisible: boolean;
  focus: FocusTarget;
  overlay: OverlayState;
  mode: AgentMode;
  scrollOffset: number;
  commandMode: boolean;
  toggleSidebar: () => void;
  toggleRightPanel: () => void;
  setFocus: (focus: FocusTarget) => void;
  cycleFocus: () => void;
  setOverlay: (overlay: OverlayState) => void;
  setMode: (mode: AgentMode) => void;
  toggleMode: () => void;
  setScrollOffset: (value: number) => void;
  setCommandMode: (value: boolean) => void;
};

export const UIStateContext = createContext<UIStateValue | null>(null);

type Props = {
  children: React.ReactNode;
};

const focusOrder: FocusTarget[] = ["input", "sidebar", "chat"];

export function UIStateProvider({ children }: Props) {
  const [sidebarVisible, setSidebarVisible] = useState(true);
  const [rightPanelVisible, setRightPanelVisible] = useState(true);
  const [focus, setFocus] = useState<FocusTarget>("input");
  const [overlay, setOverlay] = useState<OverlayState>("none");
  const [mode, setMode] = useState<AgentMode>("build");
  const [scrollOffset, setScrollOffset] = useState(0);
  const [commandMode, setCommandMode] = useState(false);

  const cycleFocus = useCallback(() => {
    setFocus((current) => focusOrder[(focusOrder.indexOf(current) + 1) % focusOrder.length] ?? "input");
  }, []);

  const toggleMode = useCallback(() => {
    setMode((current) => (current === "build" ? "plan" : "build"));
  }, []);

  const value = useMemo<UIStateValue>(
    () => ({
      sidebarVisible,
      rightPanelVisible,
      focus,
      overlay,
      mode,
      scrollOffset,
      commandMode,
      toggleSidebar: () => setSidebarVisible((current) => !current),
      toggleRightPanel: () => setRightPanelVisible((current) => !current),
      setFocus,
      cycleFocus,
      setOverlay,
      setMode,
      toggleMode,
      setScrollOffset,
      setCommandMode
    }),
    [sidebarVisible, rightPanelVisible, focus, overlay, mode, scrollOffset, commandMode, cycleFocus, toggleMode]
  );

  return <UIStateContext value={value}>{children}</UIStateContext>;
}
