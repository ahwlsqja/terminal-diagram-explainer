#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import readline from "node:readline";

import { validateRenderInput } from "./source-policy.mjs";

const SERVER_NAME = "terminal-diagram-explainer";
const SERVER_VERSION = "0.19.0";
const UI_URI = "ui://terminal-diagram-explainer/viewer-v1.html";
const UI_MIME_TYPE = "text/html;profile=mcp-app";
const widgetUrl = new URL("../dist/widget.html", import.meta.url);
const widgetHtml = await readFile(widgetUrl, "utf8");

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
    },
    required: ["source", "title", "theme"],
  },
  annotations: {
    readOnlyHint: true,
    destructiveHint: false,
    idempotentHint: true,
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
        return {
          content: [{ type: "text", text: `Rendered interactive diagram: ${output.title}` }],
          structuredContent: output,
        };
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
