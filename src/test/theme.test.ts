import test from "node:test";
import assert from "node:assert/strict";
import { theme, truncateText } from "../theme.js";

test("theme exposes the expected core colors", () => {
  assert.equal(theme.accent, "#7c6af7");
  assert.equal(theme.bg, "#0d0d0f");
  assert.equal(typeof theme.text, "string");
});

test("truncateText adds an ellipsis when width is exceeded", () => {
  assert.equal(truncateText("abcdefghijklmnopqrstuvwxyz", 10), "abcdefghi\u2026");
  assert.equal(truncateText("short", 10), "short");
  assert.equal(truncateText("x", 0), "");
});
