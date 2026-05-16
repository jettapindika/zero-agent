import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { render } from "ink-testing-library";
import { Text } from "ink";
import { PiSessionContext, PiSessionProvider } from "../providers/PiSessionProvider.js";

test("PiSessionProvider exposes default state", async () => {
  function Probe() {
    const ctx = React.useContext(PiSessionContext);
    if (!ctx) return <Text>missing</Text>;
    return (
      <Text>
        {JSON.stringify({
          messageCount: ctx.messages.length,
          isStreaming: ctx.isStreaming,
          model: ctx.model,
          sessionCount: ctx.sessions.length
        })}
      </Text>
    );
  }

  const app = render(
    <PiSessionProvider>
      <Probe />
    </PiSessionProvider>
  );

  await new Promise((resolve) => setTimeout(resolve, 50));
  const frame = (app.lastFrame() ?? "").replace(/\n/g, "");
  assert.match(frame, /"messageCount":0/);
  assert.match(frame, /"isStreaming":false/);
  assert.match(frame, /"sessionCount":1/);
  assert.match(frame, /claude-opus/);
  app.unmount();
});
