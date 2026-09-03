#!/usr/bin/env node

import { readFile, writeFile } from "node:fs/promises";

import { MAX_MERMAID_BYTES, validateRenderInput } from "./source-policy.mjs";

function parseArgs(args) {
  let title = "Software diagram";
  let theme = "auto";
  let sourceOutput = null;
  for (let index = 0; index < args.length; index += 1) {
    const current = args[index];
    if (current === "--title" && args[index + 1]) {
      title = args[++index];
      continue;
    }
    if (current === "--theme" && args[index + 1]) {
      theme = args[++index];
      continue;
    }
    if (current === "--source-output" && args[index + 1]) {
      sourceOutput = args[++index];
      continue;
    }
    throw new TypeError(`unsupported argument: ${current}`);
  }
  return { title, theme, sourceOutput };
}

async function readBoundedStdin() {
  const chunks = [];
  let total = 0;
  for await (const chunk of process.stdin) {
    total += chunk.length;
    if (total > MAX_MERMAID_BYTES) {
      throw new RangeError(`source size limit exceeded: ${MAX_MERMAID_BYTES} bytes`);
    }
    chunks.push(chunk);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function scriptSafeJson(value) {
  return JSON.stringify(value)
    .replaceAll("<", "\\u003c")
    .replaceAll(">", "\\u003e")
    .replaceAll("&", "\\u0026")
    .replaceAll("\u2028", "\\u2028")
    .replaceAll("\u2029", "\\u2029");
}

const options = parseArgs(process.argv.slice(2));
const source = await readBoundedStdin();
const payload = validateRenderInput({ source, title: options.title, theme: options.theme });
if (options.sourceOutput) {
  await writeFile(options.sourceOutput, payload.source, {
    encoding: "utf8",
    mode: 0o600,
    flag: "wx",
  });
}
const widgetUrl = new URL("../dist/widget.html", import.meta.url);
const widgetHtml = await readFile(widgetUrl, "utf8");
const scriptIndex = widgetHtml.lastIndexOf("<script>");
if (scriptIndex < 0) throw new Error("widget bundle has no inline script marker");
const payloadScript = `<script>window.__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__=${scriptSafeJson(payload)};</script>\n`;
process.stdout.write(`${widgetHtml.slice(0, scriptIndex)}${payloadScript}${widgetHtml.slice(scriptIndex)}`);
