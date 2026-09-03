import {
  App,
  applyDocumentTheme,
  applyHostFonts,
  applyHostStyleVariables,
} from "@modelcontextprotocol/ext-apps/app-with-deps";
import mermaid from "mermaid";
import { containsUnsafeMermaidSource } from "./source-policy.mjs";

const root = document.getElementById("diagram-widget");
const viewport = document.getElementById("diagram-viewport");
const surface = document.getElementById("diagram-surface");
const sourcePanel = document.getElementById("source-panel");
const sourceCode = document.getElementById("source-code");
const title = document.getElementById("diagram-title");
const status = document.getElementById("diagram-status");
const zoomValue = document.getElementById("zoom-value");
const sourceButton = document.querySelector('[data-action="source"]');

let payload = null;
let revision = 0;
let scale = 1;
let offsetX = 0;
let offsetY = 0;
let naturalWidth = 1;
let naturalHeight = 1;
let fitted = true;
let dragging = false;
let lastX = 0;
let lastY = 0;
let showingSource = false;
let pendingPayload = null;
let rendering = false;
let drainScheduled = false;
let lastRenderedSignature = "";

function applyTransform() {
  surface.style.transform = `translate(${offsetX}px, ${offsetY}px) scale(${scale})`;
  zoomValue.textContent = `${Math.round(scale * 100)}%`;
}

function fit() {
  const padding = 24;
  scale = Math.min(
    Math.max((viewport.clientWidth - padding * 2) / naturalWidth, 0.1),
    Math.max((viewport.clientHeight - padding * 2) / naturalHeight, 0.1),
    1.6,
  );
  offsetX = (viewport.clientWidth - naturalWidth * scale) / 2;
  offsetY = (viewport.clientHeight - naturalHeight * scale) / 2;
  fitted = true;
  applyTransform();
}

function zoomAt(factor, clientX, clientY) {
  const bounds = viewport.getBoundingClientRect();
  const pointX = clientX - bounds.left;
  const pointY = clientY - bounds.top;
  const contentX = (pointX - offsetX) / scale;
  const contentY = (pointY - offsetY) / scale;
  scale = Math.min(8, Math.max(0.1, scale * factor));
  offsetX = pointX - contentX * scale;
  offsetY = pointY - contentY * scale;
  fitted = false;
  applyTransform();
}

function resolvedTheme(requested) {
  if (requested === "light" || requested === "dark") return requested;
  const hostTheme = app.getHostContext()?.theme;
  if (hostTheme === "light" || hostTheme === "dark") return hostTheme;
  return matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function sanitizeSvg(svg) {
  svg.querySelectorAll("script,image").forEach((node) => node.remove());
  svg.querySelectorAll("a").forEach((node) => {
    node.removeAttribute("href");
    node.removeAttribute("xlink:href");
  });
  for (const node of svg.querySelectorAll("*")) {
    for (const attribute of [...node.attributes]) {
      if (/^on/iu.test(attribute.name) || /url\s*\(/iu.test(attribute.value)) {
        node.removeAttribute(attribute.name);
      }
    }
  }
}

async function render(nextPayload) {
  if (!nextPayload || typeof nextPayload.source !== "string") return;
  if (containsUnsafeMermaidSource(nextPayload.source)) {
    status.textContent = "Unsafe Mermaid source was rejected.";
    status.dataset.state = "error";
    return;
  }
  payload = nextPayload;
  const currentRevision = ++revision;
  const displayTitle = nextPayload.title || "Software diagram";
  title.textContent = displayTitle;
  title.title = displayTitle;
  title.setAttribute("aria-label", displayTitle);
  sourceCode.textContent = nextPayload.source;
  status.textContent = "Rendering...";
  status.dataset.state = "loading";
  const theme = resolvedTheme(nextPayload.theme);
  root.dataset.theme = theme;
  applyDocumentTheme(theme);
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    suppressErrorRendering: true,
    theme: theme === "dark" ? "dark" : "neutral",
    maxTextSize: 250000,
    maxEdges: 200,
    flowchart: {
      htmlLabels: false,
      nodeSpacing: 50,
      rankSpacing: 60,
      padding: 16,
      wrappingWidth: 240,
    },
    class: { htmlLabels: false },
  });
  try {
    const result = await mermaid.render(`terminal-diagram-${currentRevision}`, nextPayload.source);
    if (currentRevision !== revision) return;
    surface.innerHTML = result.svg;
    const svg = surface.querySelector("svg");
    if (!svg) throw new Error("Mermaid returned no SVG");
    sanitizeSvg(svg);
    const viewBox = svg.viewBox.baseVal;
    naturalWidth = Math.max(viewBox.width || svg.width.baseVal.value || 1, 1);
    naturalHeight = Math.max(viewBox.height || svg.height.baseVal.value || 1, 1);
    surface.style.width = `${naturalWidth}px`;
    surface.style.height = `${naturalHeight}px`;
    svg.style.width = "100%";
    svg.style.height = "100%";
    status.textContent = "";
    status.dataset.state = "ready";
    fit();
  } catch (error) {
    if (currentRevision !== revision) return;
    surface.replaceChildren();
    status.textContent = error instanceof Error ? error.message : "Diagram render failed.";
    status.dataset.state = "error";
  }
}

function payloadSignature(nextPayload) {
  if (!nextPayload || typeof nextPayload.source !== "string") return "";
  return `${nextPayload.theme ?? "auto"}\u0000${nextPayload.title ?? ""}\u0000${nextPayload.source}`;
}

async function drainRenderQueue() {
  if (rendering) return;
  rendering = true;
  drainScheduled = false;
  try {
    while (pendingPayload) {
      const next = pendingPayload;
      pendingPayload = null;
      const signature = payloadSignature(next);
      if (signature && signature === lastRenderedSignature) continue;
      await render(next);
      if (status.dataset.state === "ready") lastRenderedSignature = signature;
    }
  } finally {
    rendering = false;
    if (pendingPayload && !drainScheduled) scheduleRender(pendingPayload);
  }
}

function scheduleRender(nextPayload) {
  if (!nextPayload || typeof nextPayload.source !== "string") return;
  pendingPayload = nextPayload;
  if (rendering || drainScheduled) return;
  drainScheduled = true;
  setTimeout(() => void drainRenderQueue(), 0);
}

function applyHostContext(context) {
  if (!context) return;
  if (context.theme) applyDocumentTheme(context.theme);
  if (context.styles?.variables) applyHostStyleVariables(context.styles.variables);
  if (context.styles?.css?.fonts) applyHostFonts(context.styles.css.fonts);
  if (payload?.theme === "auto") scheduleRender(payload);
}

function toggleSource() {
  showingSource = !showingSource;
  sourcePanel.hidden = !showingSource;
  viewport.hidden = showingSource;
  sourceButton.setAttribute("aria-pressed", String(showingSource));
  sourceButton.setAttribute("aria-label", showingSource ? "Show diagram" : "Show Mermaid source");
}

document.querySelector('[data-action="zoom-in"]').addEventListener("click", () =>
  zoomAt(1.25, viewport.clientWidth / 2, viewport.clientHeight / 2),
);
document.querySelector('[data-action="zoom-out"]').addEventListener("click", () =>
  zoomAt(0.8, viewport.clientWidth / 2, viewport.clientHeight / 2),
);
document.querySelector('[data-action="fit"]').addEventListener("click", fit);
document.querySelector('[data-action="reset"]').addEventListener("click", () => {
  scale = 1;
  offsetX = 16;
  offsetY = 16;
  fitted = false;
  applyTransform();
});
sourceButton.addEventListener("click", toggleSource);

viewport.addEventListener(
  "wheel",
  (event) => {
    event.preventDefault();
    zoomAt(event.deltaY < 0 ? 1.12 : 1 / 1.12, event.clientX, event.clientY);
  },
  { passive: false },
);
viewport.addEventListener("pointerdown", (event) => {
  if (event.pointerType === "mouse" && event.button !== 0) return;
  event.preventDefault();
  dragging = true;
  lastX = event.clientX;
  lastY = event.clientY;
  viewport.dataset.dragging = "true";
  viewport.setPointerCapture(event.pointerId);
});
viewport.addEventListener("pointermove", (event) => {
  if (!dragging) return;
  offsetX += event.clientX - lastX;
  offsetY += event.clientY - lastY;
  lastX = event.clientX;
  lastY = event.clientY;
  fitted = false;
  applyTransform();
});
function endDrag(event) {
  if (!dragging) return;
  dragging = false;
  viewport.dataset.dragging = "false";
  if (viewport.hasPointerCapture(event.pointerId)) viewport.releasePointerCapture(event.pointerId);
}
viewport.addEventListener("pointerup", endDrag);
viewport.addEventListener("pointercancel", endDrag);
viewport.addEventListener("dblclick", fit);
new ResizeObserver(() => {
  if (fitted) fit();
}).observe(viewport);

const app = new App(
  { name: "terminal-diagram-explainer", version: "0.20.2" },
  {},
  { autoResize: true, strict: true },
);
app.addEventListener("toolinput", (params) => scheduleRender(params.arguments));
app.addEventListener("toolresult", (params) => scheduleRender(params.structuredContent));
app.addEventListener("hostcontextchanged", applyHostContext);

const standalonePayload = window.__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__;
if (standalonePayload) {
  scheduleRender(standalonePayload);
} else {
  const compatibilityPayload = window.openai?.toolOutput ?? window.openai?.toolInput;
  if (compatibilityPayload) scheduleRender(compatibilityPayload);
  app.connect().then(() => applyHostContext(app.getHostContext())).catch((error) => {
    if (!payload) {
      status.textContent = error instanceof Error ? error.message : "MCP Apps bridge unavailable.";
      status.dataset.state = "error";
    }
  });
}
