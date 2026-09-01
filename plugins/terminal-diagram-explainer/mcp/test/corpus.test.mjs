import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { containsUnsafeMermaidSource } from "../src/source-policy.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("visual corpus keeps at least 200 unique safe Mermaid use cases", async () => {
  const casesRoot = path.join(root, "testdata/cases");
  const files = (await readdir(casesRoot)).filter((name) => name.endsWith(".json")).sort();
  const cases = [];
  for (const file of files) {
    const parsed = JSON.parse(await readFile(path.join(casesRoot, file), "utf8"));
    assert.ok(Array.isArray(parsed), file);
    cases.push(...parsed);
  }
  assert.ok(cases.length >= 200, `cases=${cases.length}`);
  assert.equal(new Set(cases.map((item) => item.id)).size, cases.length);
  for (const item of cases) {
    assert.match(item.id, /^[A-Z]+-[0-9]{3}$/u);
    assert.ok(typeof item.kind === "string" && item.kind.length > 0, item.id);
    assert.ok(typeof item.category === "string" && item.category.length > 0, item.id);
    assert.ok(typeof item.source === "string" && item.source.trim().length > 0, item.id);
    assert.equal(containsUnsafeMermaidSource(item.source), false, item.id);
    assert.ok(Array.isArray(item.risk_tags) && item.risk_tags.length > 0, item.id);
  }
});
