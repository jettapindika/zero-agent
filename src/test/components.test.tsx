import test from "node:test";
import assert from "node:assert/strict";
import { render } from "ink-testing-library";
import { Spinner } from "../components/Spinner.js";

test("Spinner renders a frame and optional label", () => {
  const app = render(<Spinner label="loading" />);
  const frame = app.lastFrame() ?? "";

  assert.match(frame, /loading/);
  app.unmount();
});
