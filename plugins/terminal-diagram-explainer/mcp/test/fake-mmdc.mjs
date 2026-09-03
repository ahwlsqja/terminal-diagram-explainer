#!/usr/bin/env node

import { appendFileSync, writeFileSync } from "node:fs";

const args = process.argv.slice(2);
if (args.length === 1 && args[0] === "--version") {
  process.stdout.write("11.16.0\n");
  process.exit(0);
}
if (process.env.FAKE_MMDC_LOG) {
  appendFileSync(process.env.FAKE_MMDC_LOG, `${JSON.stringify(args)}\n`);
}
if (process.env.FAKE_MMDC_FAIL === "parse") {
  process.stderr.write("Parse error on line 2\n");
  process.exit(1);
}
const outputIndex = args.indexOf("--output");
const formatIndex = args.indexOf("--outputFormat");
if (outputIndex < 0 || formatIndex < 0) process.exit(2);
const output = args[outputIndex + 1];
const format = args[formatIndex + 1];
if (format === "svg") {
  writeFileSync(
    output,
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 120"><defs><marker id="arrow"><path d="M0 0L10 5L0 10z"/></marker></defs><g class="nodes"><g class="node default"><rect width="100" height="40"/><text>Semantic Mermaid</text></g><path class="flowchart-link" d="M100 20L220 20" marker-end="url(#arrow)"/></g></svg>',
  );
} else if (format === "png") {
  writeFileSync(
    output,
    Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
      "base64",
    ),
  );
} else {
  process.exit(2);
}
