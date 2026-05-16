import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { render } from "ink-testing-library";
import { Text } from "ink";
import { PiSessionContext, PiSessionProvider } from "../providers/PiSessionProvider.js";
import { createMockSession, createMockSessionManager } from "./mocks/piSdk.js";

test("PiSessionProvider normalizes sdk events into view state", async () => {
  const mockSession = createMockSession();

  function Probe() {
    const ctx = React.useContext(PiSessionContext);
    if (!ctx) return <Text>missing</Text>;

    React.useEffect(() => {
      void ctx.prompt("hello").then(() => {
        mockSession.emit({ type: "agent_start" });
        mockSession.emit({ type: "message_start", role: "assistant", messageId: "a1" });
        mockSession.emit({
          type: "message_update",
          assistantMessageEvent: { type: "text_delta", delta: "Hi" }
        });
        mockSession.emit({
          type: "tool_execution_start",
          toolCallId: "t1",
          toolName: "read",
          args: { path: "src/index.tsx" }
        });
        mockSession.emit({
          type: "tool_execution_end",
          toolCallId: "t1",
          toolName: "read",
          result: "file content",
          isError: false
        });
        mockSession.emit({ type: "agent_end" });
      });
    }, [ctx]);

    return (
      <Text>
        {JSON.stringify({
          messageCount: ctx.messages.length,
          isStreaming: ctx.isStreaming,
          hasToolCall: ctx.activeToolCall !== null,
          sessionCount: ctx.sessions.length
        })}
      </Text>
    );
  }

  const app = render(
    <PiSessionProvider
      createSession={async () => ({ session: mockSession, extensionsResult: {} })}
      createSessionManager={async () => createMockSessionManager()}
    >
      <Probe />
    </PiSessionProvider>
  );

  // Wait for async bootstrap + events
  await new Promise((resolve) => setTimeout(resolve, 50));

  const frame = (app.lastFrame() ?? "").replace(/\n/g, "");
  assert.match(frame, /"sessionCount":2/);
  assert.match(frame, /"isStreaming":false/);
  app.unmount();
});
