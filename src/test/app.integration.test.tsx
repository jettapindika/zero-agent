import test from "node:test";
import assert from "node:assert/strict";
import { render } from "ink-testing-library";
import { PiSessionProvider } from "../providers/PiSessionProvider.js";
import { UIStateProvider } from "../providers/UIStateProvider.js";
import { App } from "../app.js";

test("Full app renders header, footer, and chat area", async () => {
  const app = render(
    <PiSessionProvider>
      <UIStateProvider>
        <App />
      </UIStateProvider>
    </PiSessionProvider>
  );

  await new Promise((resolve) => setTimeout(resolve, 50));
  const frame = (app.lastFrame() ?? "").replace(/\n/g, " ");

  assert.match(frame, /opencode-pi/);
  assert.match(frame, /build/);
  assert.match(frame, /\? help/);
  assert.match(frame, /No messages yet/);
  assert.match(frame, /claude-opus/);

  app.unmount();
});
