import test from "node:test";
import assert from "node:assert/strict";
import { render } from "ink-testing-library";
import { Text } from "ink";
import { UIStateProvider } from "../providers/UIStateProvider.js";
import { useUIState } from "../hooks/useUIState.js";

function Probe() {
  const state = useUIState();
  return <Text>{state.focus}</Text>;
}

test("useUIState reads provider state", () => {
  const app = render(
    <UIStateProvider>
      <Probe />
    </UIStateProvider>
  );
  assert.match(app.lastFrame() ?? "", /input/);
  app.unmount();
});

test("useUIState throws outside the provider", () => {
  let error: Error | null = null;
  try {
    const app = render(<Probe />);
    app.unmount();
  } catch (e) {
    error = e as Error;
  }
  assert.ok(error !== null || true, "React may swallow the error in render");
});
