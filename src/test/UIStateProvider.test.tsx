import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { render } from "ink-testing-library";
import { Text } from "ink";
import { UIStateProvider, UIStateContext } from "../providers/UIStateProvider.js";

function Probe() {
  const ctx = React.useContext(UIStateContext);
  if (!ctx) return <Text>missing</Text>;

  return (
    <Text>
      {JSON.stringify({
        sidebarVisible: ctx.sidebarVisible,
        rightPanelVisible: ctx.rightPanelVisible,
        focus: ctx.focus,
        overlay: ctx.overlay,
        mode: ctx.mode,
        scrollOffset: ctx.scrollOffset
      })}
    </Text>
  );
}

test("UIStateProvider exposes default state", () => {
  const app = render(
    <UIStateProvider>
      <Probe />
    </UIStateProvider>
  );

  const frame = (app.lastFrame() ?? "").replace(/\n/g, "");
  assert.match(frame, /"sidebarVisible":true/);
  assert.match(frame, /"rightPanelVisible":true/);
  assert.match(frame, /"focus":"input"/);
  assert.match(frame, /"overlay":"none"/);
  assert.match(frame, /"mode":"build"/);
  assert.match(frame, /"scrollOffset":0/);
});
