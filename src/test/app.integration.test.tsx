import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { render } from "ink-testing-library";
import { PiSessionProvider } from "../providers/PiSessionProvider.js";
import { UIStateProvider } from "../providers/UIStateProvider.js";
import { App } from "../app.js";
import { createMockSession, createMockSessionManager } from "./mocks/piSdk.js";

test("Full app renders header, footer, and chat area", async () => {
  const mockSession = createMockSession();

  const app = render(
    <PiSessionProvider
      createSession={async () => ({ session: mockSession, extensionsResult: {} })}
      createSessionManager={async () => createMockSessionManager()}
    >
      <UIStateProvider>
        <App />
      </UIStateProvider>
    </PiSessionProvider>
  );

  await new Promise((resolve) => setTimeout(resolve, 50));

  const frame = (app.lastFrame() ?? "").replace(/\n/g, " ");

  assert.match(frame, /opencode-pi/);
  assert.match(frame, /build/);
  assert.match(frame, /tokens:/);
  assert.match(frame, /\?: help/);

  app.unmount();
});

test("Full app shows messages after prompt event", async () => {
  const mockSession = createMockSession();

  const app = render(
    <PiSessionProvider
      createSession={async () => ({ session: mockSession, extensionsResult: {} })}
      createSessionManager={async () => createMockSessionManager()}
    >
      <UIStateProvider>
        <App />
      </UIStateProvider>
    </PiSessionProvider>
  );

  await new Promise((resolve) => setTimeout(resolve, 50));

  mockSession.emit({ type: "agent_start" });
  mockSession.emit({ type: "message_start", role: "assistant", messageId: "a1" });
  mockSession.emit({
    type: "message_update",
    assistantMessageEvent: { type: "text_delta", delta: "Hello world" }
  });
  mockSession.emit({ type: "agent_end" });

  await new Promise((resolve) => setTimeout(resolve, 50));

  const frame = (app.lastFrame() ?? "").replace(/\n/g, " ");
  assert.match(frame, /Hello world/);

  app.unmount();
});
