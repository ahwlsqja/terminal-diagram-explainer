import assert from "node:assert/strict";
import test from "node:test";

import {
  MAX_MERMAID_BYTES,
  containsUnsafeMermaidSource,
  validateRenderInput,
} from "../src/source-policy.mjs";

test("accepts ordinary Mermaid diagrams and formatting-only labels", () => {
  for (const source of [
    "flowchart LR\nA[Start] --> B[Done]",
    "sequenceDiagram\nAlice->>Bob: a < b and x > y",
    "sequenceDiagram\nRecipient->>API: click unsubscribe",
    'flowchart TD\nA["line one<br/>line two"] --> B["<i>done</i>"]',
    "stateDiagram-v2\nstate Choice <<choice>>",
  ]) {
    assert.equal(containsUnsafeMermaidSource(source), false, source);
    assert.equal(validateRenderInput({ source }).source, source);
  }
});

test("canonicalizes quoted model-generated flow labels to standard Mermaid syntax", () => {
  const source = `%% A -- "example remains unchanged" --> B
flowchart TD
  REVIEW -- "finding 또는 실패" -.-> WORKTREE
  REVIEW -- "clean" --> STABLE{"120초 안정성?"}
  STABLE -- "아니오" -.-> WORKTREE
  STABLE -- "예" --> MERGE`;
  assert.equal(
    validateRenderInput({ source }).source,
    `%% A -- "example remains unchanged" --> B
flowchart TD
  REVIEW -.->|finding 또는 실패| WORKTREE
  REVIEW -->|clean| STABLE{"120초 안정성?"}
  STABLE -.->|아니오| WORKTREE
  STABLE -->|예| MERGE`,
  );
});

test("does not mutate valid labels or non-flow diagrams", () => {
  for (const source of [
    'flowchart LR\nA["quoted -- node"] --> B',
    "flowchart LR\nA -- yes --> B",
    'sequenceDiagram\nA-->>B: "quoted -- message"',
  ]) {
    assert.equal(validateRenderInput({ source }).source, source);
  }
});

test("rejects resource loading, config injection, active HTML, and disguised escapes", () => {
  for (const source of [
    'flowchart TD\nA@{ img: "https://example.test/x.png" }',
    "flowchart TD\nA[url(https://example.test/x)]",
    "flowchart TD\nA[@import 'x']",
    '%%{init: {"themeCSS": "a { color: red }"}}%%\nflowchart TD\nA',
    'flowchart TD\nA["<img src=x>"]',
    'flowchart TD\nA["&#60;img src=x&#62;"]',
    'flowchart TD\nA@{ "\\u0069mg": "https://example.test/x" }',
    "flowchart TD\nA[ok]\nclick A href \"https://example.test\"",
  ]) {
    assert.equal(containsUnsafeMermaidSource(source), true, source);
    assert.throws(() => validateRenderInput({ source }), /unsafe Mermaid source/);
  }
});

test("rejects malformed arguments, controls, and oversized input", () => {
  assert.throws(() => validateRenderInput(null), /object/);
  assert.throws(() => validateRenderInput({ source: "" }), /non-empty/);
  assert.throws(() => validateRenderInput({ source: "flowchart LR\nA[bad\u001b]" }), /control/);
  assert.throws(() => validateRenderInput({ source: "flowchart LR\nA[abc\u202Etxt]" }), /control/);
  assert.throws(
    () => validateRenderInput({ source: "x".repeat(MAX_MERMAID_BYTES + 1) }),
    /size limit/,
  );
  assert.throws(() => validateRenderInput({ source: "flowchart LR\nA", theme: "forest" }), /theme/);
  assert.throws(() => validateRenderInput({ source: "flowchart LR\nA", title: "x".repeat(81) }), /title/);
  assert.throws(() => validateRenderInput({ source: "flowchart LR\nA", extra: true }), /unexpected/);
});
