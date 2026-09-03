#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { accessSync, constants as fsConstants } from "node:fs";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import os from "node:os";
import path from "node:path";
import readline from "node:readline";

import { renderMermaidArtifact } from "./mermaid-cli.mjs";
import { validateRenderInput } from "./source-policy.mjs";
import { buildStandaloneHtml } from "./standalone-html.mjs";

const SERVER_NAME = "terminal-diagram-explainer";
const SERVER_VERSION = "0.20.2";
const UI_URI = "ui://terminal-diagram-explainer/viewer-v1.html";
const UI_MIME_TYPE = "text/html;profile=mcp-app";
const widgetUrl = new URL("../dist/widget.html", import.meta.url);
const widgetHtml = await readFile(widgetUrl, "utf8");
const localViewers = new Map();
const localViewerServer = createServer(serveLocalViewer);
const localViewerOrigin = await listenOnLoopback(localViewerServer);
localViewerServer.unref();

const tool = {
  name: "render_diagram",
  title: "Render software diagram",
  description:
    "Render validated Mermaid source as an interactive pan, zoom, fit, and source-inspection view. Use for software architecture, flowchart, sequence, ER, and state visualization requests.",
  inputSchema: {
    type: "object",
    additionalProperties: false,
    properties: {
      source: {
        type: "string",
        minLength: 1,
        maxLength: 262144,
        description: "Mermaid diagram source.",
      },
      title: {
        type: "string",
        minLength: 1,
        maxLength: 80,
        description: "Concise diagram title.",
      },
      theme: {
        type: "string",
        enum: ["auto", "light", "dark"],
        default: "auto",
      },
    },
    required: ["source"],
  },
  outputSchema: {
    type: "object",
    additionalProperties: false,
    properties: {
      source: { type: "string" },
      title: { type: "string" },
      theme: { type: "string", enum: ["auto", "light", "dark"] },
      terminalFallback: { type: "string" },
      uiHint: { type: "string" },
      localViewerUrl: { type: "string" },
    },
    required: ["source", "title", "theme", "terminalFallback", "uiHint", "localViewerUrl"],
  },
  annotations: {
    readOnlyHint: true,
    destructiveHint: false,
    idempotentHint: false,
    openWorldHint: false,
  },
  _meta: {
    ui: { resourceUri: UI_URI },
    "openai/outputTemplate": UI_URI,
    "openai/toolInvocation/invoking": "Rendering diagram...",
    "openai/toolInvocation/invoked": "Diagram ready",
  },
};

const resource = {
  uri: UI_URI,
  name: "Interactive software diagram viewer",
  title: "Software diagram",
  description: "A sandboxed Mermaid viewer with pan, zoom, fit, and source inspection.",
  mimeType: UI_MIME_TYPE,
};

const UI_HINT = "Codex TUI가 image를 placeholder로 표시하면 Local interactive HTML 링크를 여세요. /app으로 Desktop App inline UI도 열 수 있습니다.";

function listenOnLoopback(server) {
  return new Promise((resolve) => {
    const failed = () => resolve("");
    server.once("error", failed);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", failed);
      const address = server.address();
      resolve(typeof address === "object" && address ? `http://127.0.0.1:${address.port}` : "");
    });
  });
}

function serveLocalViewer(request, response) {
  const expectedHost = `127.0.0.1:${request.socket.localPort}`;
  if (request.headers.host?.toLowerCase() !== expectedHost) {
    response.writeHead(421, { connection: "close" });
    response.end();
    return;
  }
  const expectedOrigin = `http://${expectedHost}`;
  if (request.headers.origin && request.headers.origin !== expectedOrigin) {
    response.writeHead(403, { connection: "close" });
    response.end();
    return;
  }
  if (request.method !== "GET" && request.method !== "HEAD") {
    response.writeHead(405, { allow: "GET, HEAD", connection: "close" });
    response.end();
    return;
  }
  const match = request.url?.match(/^\/([A-Za-z0-9_-]{24})\/diagram\.html$/u);
  const payload = match ? localViewers.get(match[1]) : null;
  if (!payload) {
    response.writeHead(404, {
      "content-type": "text/plain; charset=utf-8",
      connection: "close",
    });
    response.end("Not found\n");
    return;
  }
  const html = buildStandaloneHtml(widgetHtml, payload);
  response.writeHead(200, {
    "content-type": "text/html; charset=utf-8",
    "content-length": Buffer.byteLength(html),
    "cache-control": "no-store",
    "content-security-policy": "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'none'; img-src data: blob:; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; frame-ancestors 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'",
    connection: "close",
    "cross-origin-resource-policy": "same-origin",
    "referrer-policy": "no-referrer",
    "x-content-type-options": "nosniff",
  });
  response.end(request.method === "HEAD" ? undefined : html);
}

function registerLocalViewer(payload) {
  if (!localViewerOrigin) return "";
  const token = randomBytes(18).toString("base64url");
  localViewers.set(token, payload);
  while (localViewers.size > 16) {
    localViewers.delete(localViewers.keys().next().value);
  }
  return `${localViewerOrigin}/${token}/diagram.html`;
}

function rendererCommand() {
  const configured = process.env.TERM_DIAGRAM_BIN;
  const candidates = [
    configured,
    process.env.CODEX_HOME ? path.join(process.env.CODEX_HOME, "bin", "term-diagram") : null,
    path.join(os.homedir(), ".codex", "bin", "term-diagram"),
  ].filter(Boolean);
  for (const binary of candidates) {
    try {
      accessSync(binary, fsConstants.X_OK);
      let prefixArgs = [];
      if (binary === configured && process.env.TERM_DIAGRAM_PREFIX_ARGS_JSON) {
        const parsed = JSON.parse(process.env.TERM_DIAGRAM_PREFIX_ARGS_JSON);
        if (Array.isArray(parsed) && parsed.every((item) => typeof item === "string")) {
          prefixArgs = parsed;
        }
      }
      return { binary, prefixArgs };
    } catch {
      // Try the next deterministic local candidate.
    }
  }
  return null;
}

function renderTerminalFallback(source) {
  const command = rendererCommand();
  if (!command) return "";
  const result = spawnSync(
    command.binary,
    [...command.prefixArgs, "-width", "120", "-height", "200", "-fit"],
    {
      input: source,
      encoding: "utf8",
      timeout: 5000,
      maxBuffer: 2 * 1024 * 1024,
      windowsHide: true,
    },
  );
  if (result.status !== 0 || result.error || typeof result.stdout !== "string") return "";
  return result.stdout.trimEnd();
}

function toolResult(output) {
  const localViewerUrl = registerLocalViewer(output);
  let png = null;
  let graphicFailure = "";
  try {
    png = renderMermaidArtifact(output.source, {
      format: "png",
      theme: output.theme === "dark" ? "dark" : "light",
    });
  } catch (error) {
    graphicFailure = summarizeGraphicFailure(error);
    // The interactive resource and terminal renderer remain available without the optional CLI.
  }
  const terminalFallback = png ? "" : renderTerminalFallback(output.source);
  const safeFallback = terminalFallback.replaceAll("```", "` ` `");
  const preview = png
    ? "\n\nOfficial Mermaid CLI로 PNG 미리보기를 생성했습니다."
    : terminalFallback
      ? `\n\n${graphicFailure} terminal fallback을 표시합니다:\n\`\`\`text\n${safeFallback}\n\`\`\``
      : `\n\n${graphicFailure} 표준 Mermaid edge label 문법 \`A -->|label| B\` 또는 \`A -.->|label| B\`로 source를 고쳐 한 번 재시도하세요.`;
  const content = [
    {
      type: "text",
      text: `Rendered interactive diagram: ${output.title}\n${UI_HINT}${preview}${localViewerUrl ? `\n\nLocal interactive HTML: ${localViewerUrl}` : ""}`,
    },
  ];
  if (png) {
    content.push({ type: "image", data: png.toString("base64"), mimeType: "image/png" });
  }
  if (localViewerUrl) {
    content.push({
      type: "resource_link",
      uri: localViewerUrl,
      name: "Local interactive HTML",
      title: output.title,
      description: "Open the self-contained Mermaid viewer while this Codex session is running.",
      mimeType: "text/html",
    });
  }
  return {
    content,
    structuredContent: { ...output, terminalFallback, uiHint: UI_HINT, localViewerUrl },
  };
}

function summarizeGraphicFailure(error) {
  if (error?.code === "MMDC_UNAVAILABLE") return "Mermaid CLI runtime을 찾지 못했습니다.";
  const message = error instanceof Error ? error.message : "";
  if (/lexical error|parse error|syntax error/iu.test(message)) {
    return "Mermaid source 문법 오류로 PNG를 생성하지 못했습니다.";
  }
  if (/artifact must be between/iu.test(message)) {
    return "Mermaid PNG가 artifact 크기 제한을 벗어났습니다.";
  }
  return "Mermaid CLI가 PNG를 생성하지 못했습니다.";
}

function resultFor(method, params) {
  switch (method) {
    case "initialize":
      return {
        protocolVersion: params?.protocolVersion ?? "2025-11-25",
        capabilities: { tools: { listChanged: false }, resources: {} },
        serverInfo: { name: SERVER_NAME, version: SERVER_VERSION },
        instructions:
          "Call render_diagram for visualization requests. The tool remains useful without UI through its text and structured result.",
      };
    case "ping":
      return {};
    case "tools/list":
      return { tools: [tool] };
    case "resources/list":
      return { resources: [resource] };
    case "resources/templates/list":
      return { resourceTemplates: [] };
    case "resources/read":
      if (params?.uri !== UI_URI) throw rpcError(-32602, "unknown resource URI");
      return {
        contents: [
          {
            uri: UI_URI,
            mimeType: UI_MIME_TYPE,
            text: widgetHtml,
            _meta: {
              ui: {
                prefersBorder: false,
                csp: { connectDomains: [], resourceDomains: [] },
              },
              "openai/widgetDescription":
                "Interactive Mermaid software diagram with pan, zoom, fit, and source inspection.",
            },
          },
        ],
      };
    case "tools/call":
      if (params?.name !== tool.name) throw rpcError(-32602, "unknown tool");
      try {
        const output = validateRenderInput(params.arguments);
        return toolResult(output);
      } catch (error) {
        return {
          isError: true,
          content: [
            {
              type: "text",
              text: `Diagram source rejected: ${error instanceof Error ? error.message : "invalid input"}`,
            },
          ],
        };
      }
    case "prompts/list":
      return { prompts: [] };
    default:
      throw rpcError(-32601, `method not found: ${method}`);
  }
}

function rpcError(code, message) {
  return Object.assign(new Error(message), { code });
}

function write(message) {
  process.stdout.write(`${JSON.stringify(message)}\n`);
}

function handle(message) {
  if (!message || message.jsonrpc !== "2.0" || typeof message.method !== "string") {
    if (message?.id !== undefined) {
      write({ jsonrpc: "2.0", id: message.id ?? null, error: { code: -32600, message: "invalid request" } });
    }
    return;
  }
  if (message.id === undefined) return;
  try {
    write({ jsonrpc: "2.0", id: message.id, result: resultFor(message.method, message.params ?? {}) });
  } catch (error) {
    write({
      jsonrpc: "2.0",
      id: message.id,
      error: {
        code: Number.isInteger(error?.code) ? error.code : -32603,
        message: error instanceof Error ? error.message : "internal error",
      },
    });
  }
}

const input = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
input.on("line", (line) => {
  if (Buffer.byteLength(line) > 1024 * 1024) {
    write({ jsonrpc: "2.0", id: null, error: { code: -32700, message: "message too large" } });
    return;
  }
  try {
    handle(JSON.parse(line));
  } catch {
    write({ jsonrpc: "2.0", id: null, error: { code: -32700, message: "parse error" } });
  }
});
