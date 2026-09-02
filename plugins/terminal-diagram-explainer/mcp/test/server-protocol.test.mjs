import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function startServer() {
  const child = spawn(process.execPath, ["src/server.mjs"], {
    cwd: root,
    env: {
      ...process.env,
      TERM_DIAGRAM_BIN: process.execPath,
      TERM_DIAGRAM_PREFIX_ARGS_JSON: JSON.stringify(["test/fake-term-diagram.mjs"]),
    },
    stdio: ["pipe", "pipe", "pipe"],
  });
  const pending = new Map();
  let buffer = "";
  child.stdout.setEncoding("utf8");
  child.stdout.on("data", (chunk) => {
    buffer += chunk;
    for (;;) {
      const newline = buffer.indexOf("\n");
      if (newline < 0) break;
      const line = buffer.slice(0, newline).trim();
      buffer = buffer.slice(newline + 1);
      if (!line) continue;
      const message = JSON.parse(line);
      const waiter = pending.get(message.id);
      if (waiter) {
        pending.delete(message.id);
        waiter.resolve(message);
      }
    }
  });
  let nextId = 1;
  return {
    child,
    request(method, params = {}) {
      const id = nextId++;
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
          pending.delete(id);
          reject(new Error(`timeout waiting for ${method}`));
        }, 3000);
        pending.set(id, {
          resolve(message) {
            clearTimeout(timer);
            resolve(message);
          },
        });
      });
    },
    notify(method, params = {}) {
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method, params })}\n`);
    },
    async close() {
      child.stdin.end();
      if (child.exitCode === null) await once(child, "exit");
    },
  };
}

test("serves a render tool and self-contained MCP App resource", async (t) => {
  const server = startServer();
  t.after(() => server.close());

  const initialized = await server.request("initialize", {
    protocolVersion: "2025-11-25",
    capabilities: {},
    clientInfo: { name: "test-client", version: "1.0.0" },
  });
  assert.equal(initialized.result.serverInfo.name, "terminal-diagram-explainer");
  assert.equal(initialized.result.capabilities.tools.listChanged, false);
  assert.deepEqual(initialized.result.capabilities.resources, {});
  server.notify("notifications/initialized");

  const tools = await server.request("tools/list");
  assert.equal(tools.result.tools.length, 1);
  const tool = tools.result.tools[0];
  assert.equal(tool.name, "render_diagram");
  assert.equal(tool._meta.ui.resourceUri, "ui://terminal-diagram-explainer/viewer-v1.html");
  assert.equal(tool.annotations.readOnlyHint, true);

  const resources = await server.request("resources/list");
  assert.equal(resources.result.resources[0].mimeType, "text/html;profile=mcp-app");
  const resource = await server.request("resources/read", {
    uri: "ui://terminal-diagram-explainer/viewer-v1.html",
  });
  const html = resource.result.contents[0].text;
  assert.match(html, /Content-Security-Policy/);
  assert.match(html, /connect-src 'none'/);
  assert.match(html, /data-action="fit"/);
  assert.match(html, /data-action="source"/);
  assert.match(html, /user-select: none/);
  assert.doesNotMatch(html, /<script[^>]+src=/i);
  assert.ok(Buffer.byteLength(html) < 8 * 1024 * 1024);

  const rendered = await server.request("tools/call", {
    name: "render_diagram",
    arguments: {
      source: "flowchart LR\nA[Request] --> B[Response]",
      title: "Request path",
      theme: "auto",
    },
  });
  assert.equal(rendered.result.isError, undefined);
  assert.deepEqual(rendered.result.structuredContent, {
    source: "flowchart LR\nA[Request] --> B[Response]",
    title: "Request path",
    theme: "auto",
    terminalFallback: "[Request] ---> [Response]",
    uiHint: "Codex TUI에서는 /app으로 같은 세션을 Desktop App에서 열어 inline UI를 확인하세요.",
  });
  assert.match(rendered.result.content[0].text, /Request path/);
  assert.match(rendered.result.content[0].text, /```text\n\[Request\] ---> \[Response\]\n```/);
  assert.match(rendered.result.content[0].text, /\/app/);
  assert.equal(rendered.result.content[1].type, "resource_link");
  assert.equal(rendered.result.content[1].uri, "ui://terminal-diagram-explainer/viewer-v1.html");

  const rejected = await server.request("tools/call", {
    name: "render_diagram",
    arguments: { source: 'flowchart TD\nA@{ img: "https://example.test/x" }' },
  });
  assert.equal(rejected.result.isError, true);
  assert.equal(rejected.result.structuredContent, undefined);
});

test("built widget is deterministic", async () => {
  const first = await readFile(path.join(root, "dist/widget.html"));
  const firstNotices = await readFile(path.join(root, "dist/THIRD_PARTY_NOTICES.md"));
  const { execFile } = await import("node:child_process");
  const run = () =>
    new Promise((resolve, reject) => {
      execFile(process.execPath, ["build.mjs"], { cwd: root }, (error) =>
        error ? reject(error) : resolve(),
      );
    });
  await run();
  const second = await readFile(path.join(root, "dist/widget.html"));
  const secondNotices = await readFile(path.join(root, "dist/THIRD_PARTY_NOTICES.md"));
  assert.deepEqual(second, first);
  assert.deepEqual(secondNotices, firstNotices);
  assert.match(firstNotices.toString(), /mermaid@11\.17\.2/);
  assert.match(firstNotices.toString(), /@modelcontextprotocol\/ext-apps@1\.7\.5/);
  assert.doesNotMatch(firstNotices.toString(), /esbuild@/);
});
