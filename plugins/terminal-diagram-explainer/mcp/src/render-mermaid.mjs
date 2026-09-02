#!/usr/bin/env node

import { readFile, writeFile } from "node:fs/promises";

import { MMDC_VERSION, renderMermaidArtifact } from "./mermaid-cli.mjs";
import { MAX_MERMAID_BYTES, validateRenderInput } from "./source-policy.mjs";

function parseArgs(args) {
  const options = { theme: "light" };
  for (let index = 0; index < args.length; index += 1) {
    const current = args[index];
    if (current === "--input" && args[index + 1]) options.input = args[++index];
    else if (current === "--svg" && args[index + 1]) options.svg = args[++index];
    else if (current === "--png" && args[index + 1]) options.png = args[++index];
    else if (current === "--theme" && args[index + 1]) options.theme = args[++index];
    else throw new TypeError(`unsupported argument: ${current}`);
  }
  if (!options.input || (!options.svg && !options.png)) {
    throw new TypeError("--input and at least one of --svg or --png are required");
  }
  return options;
}

const options = parseArgs(process.argv.slice(2));
const sourceBuffer = await readFile(options.input);
if (sourceBuffer.length > MAX_MERMAID_BYTES) {
  throw new RangeError(`source size limit exceeded: ${MAX_MERMAID_BYTES} bytes`);
}
const source = sourceBuffer.toString("utf8");
validateRenderInput({ source, theme: options.theme });
if (options.svg) {
  await writeFile(options.svg, renderMermaidArtifact(source, { format: "svg", theme: options.theme }), {
    mode: 0o600,
  });
}
if (options.png) {
  await writeFile(options.png, renderMermaidArtifact(source, { format: "png", theme: options.theme }), {
    mode: 0o600,
  });
}
process.stdout.write(`mermaid-cli ${MMDC_VERSION}\n`);
