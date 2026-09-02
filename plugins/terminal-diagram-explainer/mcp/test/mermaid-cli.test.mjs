import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const fakeMmdc = path.join(root, "test", "fake-mmdc.mjs");
const artifactScript = path.resolve(
  root,
  "../skills/terminal-diagram-explainer/scripts/render-artifacts.sh",
);

function renderArtifacts(source, environment = {}) {
  return new Promise((resolve, reject) => {
    const child = execFile(
      artifactScript,
      {
        cwd: root,
        env: {
          ...process.env,
          TERMINAL_DIAGRAM_MMDC_BIN: process.execPath,
          TERMINAL_DIAGRAM_MMDC_PREFIX_ARGS_JSON: JSON.stringify([fakeMmdc]),
          ...environment,
        },
        maxBuffer: 16 * 1024 * 1024,
      },
      (error, stdout, stderr) => {
        if (error) reject(Object.assign(error, { stderr }));
        else resolve({ artifacts: JSON.parse(stdout), stderr });
      },
    );
    child.stdin.end(source);
  });
}

test("artifact fallback uses pinned Mermaid CLI for semantic SVG and PNG", async (t) => {
  const workDir = await mkdtemp(path.join(os.tmpdir(), "term-diagram-mmdc-test."));
  t.after(() => rm(workDir, { recursive: true, force: true }));
  const log = path.join(workDir, "mmdc.jsonl");
  const { artifacts, stderr } = await renderArtifacts(
    "flowchart TD\nStart --> Choice{Decision}\nChoice -->|yes| Done",
    { FAKE_MMDC_LOG: log },
  );
  assert.equal(stderr, "");
  assert.equal(artifacts.renderer, "mermaid-cli");
  const html = await readFile(artifacts.html, "utf8");
  const svg = await readFile(artifacts.svg, "utf8");
  const png = await readFile(artifacts.png);
  assert.match(html, /__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__/);
  assert.doesNotMatch(html, /feedback:/);
  assert.match(svg, /class="node default"/);
  assert.match(svg, /marker-end="url\(#arrow\)"/);
  assert.deepEqual(
    [...png.subarray(0, 8)],
    [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a],
  );

  const calls = (await readFile(log, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  assert.equal(calls.length, 2);
  assert.deepEqual(
    calls.map((args) => args[args.indexOf("--outputFormat") + 1]),
    ["svg", "png"],
  );
  assert.ok(calls.every((args) => args.includes("--configFile")));
  assert.ok(calls.every((args) => !args.some((arg) => arg.startsWith("--iconPacks"))));
});

test("unsafe source fails before Mermaid CLI is invoked", async (t) => {
  const workDir = await mkdtemp(path.join(os.tmpdir(), "term-diagram-mmdc-unsafe."));
  t.after(() => rm(workDir, { recursive: true, force: true }));
  const log = path.join(workDir, "mmdc.jsonl");
  await assert.rejects(
    renderArtifacts('flowchart TD\nA@{ img: "https://example.test/x" }', { FAKE_MMDC_LOG: log }),
    /unsafe Mermaid source/,
  );
  await assert.rejects(readFile(log), /ENOENT/);
});
