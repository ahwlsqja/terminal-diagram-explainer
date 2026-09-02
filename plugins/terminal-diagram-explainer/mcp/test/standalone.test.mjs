import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function renderStandalone(source) {
  return new Promise((resolve, reject) => {
    const child = execFile(
      process.execPath,
      ["src/render-standalone.mjs", "--title", "Clean fallback"],
      { cwd: root, maxBuffer: 8 * 1024 * 1024 },
      (error, stdout, stderr) => {
        if (error) reject(Object.assign(error, { stderr }));
        else resolve(stdout);
      },
    );
    child.stdin.end(source);
  });
}

test("builds a self-contained Mermaid HTML fallback without an MCP host", async () => {
  const source = "flowchart TD\nStart --> Choice{Decision}\nChoice -->|yes| Done";
  const html = await renderStandalone(source);
  assert.match(html, /Content-Security-Policy/);
  assert.match(html, /connect-src 'none'/);
  assert.match(html, /__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__/);
  assert.match(html, /Clean fallback/);
  assert.match(html, /flowchart TD\\nStart/);
  assert.doesNotMatch(html, /<script[^>]+src=/i);
  assert.ok(html.indexOf("__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__") < html.lastIndexOf("<script>"));
});

test("rejects unsafe source before creating fallback HTML", async () => {
  await assert.rejects(
    renderStandalone('flowchart TD\nA@{ img: "https://example.test/x" }'),
    /unsafe Mermaid source/,
  );
});
