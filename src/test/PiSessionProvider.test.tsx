import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { render } from "ink-testing-library";
import { Text } from "ink";
import { PiSessionContext, PiSessionProvider } from "../providers/PiSessionProvider.js";
import type { SavedSession } from "../session/storage.js";

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

test("PiSessionProvider hydrates and switches persisted sessions", async () => {
  const saved: Record<string, SavedSession> = {
    alpha: {
      id: "alpha",
      title: "Alpha project",
      createdAt: 1,
      updatedAt: 10,
      modelMessages: [{ role: "system", content: "system" }, { role: "user", content: "alpha prompt" }],
      uiMessages: [{ id: "u-alpha", role: "user", content: "alpha prompt" }]
    },
    beta: {
      id: "beta",
      title: "Beta project",
      createdAt: 2,
      updatedAt: 20,
      modelMessages: [{ role: "system", content: "system" }, { role: "user", content: "beta prompt" }],
      uiMessages: [{ id: "u-beta", role: "user", content: "beta prompt" }]
    }
  };

  const storage = {
    listSessions: async () => [
      { id: "beta", title: "Beta project", updatedAt: 20 },
      { id: "alpha", title: "Alpha project", updatedAt: 10 }
    ],
    loadSession: async (id: string) => saved[id] ?? null,
    saveSession: async (session: SavedSession) => {
      saved[session.id] = session;
    }
  };

  function Probe() {
    const ctx = React.useContext(PiSessionContext);
    if (!ctx) return <Text>missing</Text>;

    React.useEffect(() => {
      if (ctx.activeSession?.id === "beta") {
        void ctx.switchSession("alpha");
      }
    }, [ctx]);

    return (
      <Text>
        {JSON.stringify({
          active: ctx.activeSession?.id,
          sessions: ctx.sessions.map((session) => session.id),
          messages: ctx.messages.map((message) => message.content)
        })}
      </Text>
    );
  }

  const app = render(
    <PiSessionProvider storage={storage}>
      <Probe />
    </PiSessionProvider>
  );

  await new Promise((resolve) => setTimeout(resolve, 100));

  const frame = (app.lastFrame() ?? "").replace(/\n/g, "");
  assert.match(frame, /"active":"alpha"/);
  assert.match(frame, /"sessions":\["beta","alpha"\]/);
  assert.match(frame, /alpha prompt/);
  assert.doesNotMatch(frame, /beta prompt/);
  app.unmount();
});
