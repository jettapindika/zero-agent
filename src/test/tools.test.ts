import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { globTool } from "../tools/glob.js";
import { grepTool } from "../tools/grep.js";

test("globTool supports nested ** patterns", async () => {
  const root = await mkdtemp(join(tmpdir(), "pi-opencode-glob-"));
  try {
    await mkdir(join(root, "src", "components"), { recursive: true });
    await writeFile(join(root, "src", "components", "App.tsx"), "export function App() { return null; }\n");

    const result = await globTool.execute({ pattern: "src/**/*.tsx", path: root }, root);

    assert.equal(result.isError, undefined);
    assert.match(result.output, /src\/components\/App\.tsx|App\.tsx/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("grepTool honors configurable result limits", async () => {
  const root = await mkdtemp(join(tmpdir(), "pi-opencode-grep-"));
  try {
    await writeFile(join(root, "a.txt"), "needle one\nneedle two\nneedle three\n");

    const result = await grepTool.execute({ pattern: "needle", path: root, limit: 2 }, root);

    assert.equal(result.output.split("\n").length, 2);
    assert.match(result.title, /2 matches/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
