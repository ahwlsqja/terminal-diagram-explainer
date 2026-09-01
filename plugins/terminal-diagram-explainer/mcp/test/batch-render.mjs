import { createReadStream } from "node:fs";
import { mkdir, readFile, readdir, stat, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const mcpRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const qaRoot = process.argv[2] ?? "/tmp/terminal-diagram-mcp-qa";
const casesRoot = process.argv[3] ?? path.join(mcpRoot, "testdata/cases");
const artifactsRoot = path.join(qaRoot, "artifacts");
const reportsRoot = path.join(qaRoot, "reports");
const caseFiles = (await readdir(casesRoot))
  .filter((name) => name.endsWith(".json"))
  .sort();
const cases = [];
const seen = new Set();
for (const name of caseFiles) {
  const parsed = JSON.parse(await readFile(path.join(casesRoot, name), "utf8"));
  if (!Array.isArray(parsed)) throw new Error(`${name} must contain an array`);
  for (const item of parsed) {
    if (!item?.id || !item?.kind || typeof item.source !== "string" || !item.source.trim()) {
      throw new Error(`${name} contains an incomplete case`);
    }
    if (!/^[A-Z]+-[0-9]{3}$/u.test(item.id) || seen.has(item.id)) {
      throw new Error(`invalid or duplicate case id: ${item.id}`);
    }
    seen.add(item.id);
    cases.push(item);
  }
}
if (cases.length < 200) throw new Error(`case count ${cases.length} is below 200`);

await mkdir(artifactsRoot, { recursive: true });
await mkdir(reportsRoot, { recursive: true });

const routes = new Map([
  ["/", path.join(mcpRoot, "test/host-harness.html")],
  ["/dist/widget.html", path.join(mcpRoot, "dist/widget.html")],
]);
const server = createServer(async (request, response) => {
  const pathname = new URL(request.url ?? "/", `http://${request.headers.host}`).pathname;
  const file = routes.get(pathname);
  if (!file) {
    response.writeHead(404);
    response.end();
    return;
  }
  const info = await stat(file);
  response.writeHead(200, {
    "content-type": "text/html; charset=utf-8",
    "content-length": info.size,
    "cache-control": "no-store",
  });
  createReadStream(file).pipe(response);
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const address = server.address();
const hostUrl = `http://127.0.0.1:${address.port}/`;

const viewports = [
  { name: "1024-dark", width: 1024, height: 760, colorScheme: "dark" },
  { name: "736-light", width: 736, height: 720, colorScheme: "light" },
  { name: "360-dark", width: 360, height: 720, colorScheme: "dark" },
];
const browser = await chromium.launch({ channel: "chrome", headless: true });
const lanes = [];
for (const viewport of viewports) {
  const context = await browser.newContext({
    viewport: { width: viewport.width, height: viewport.height },
    colorScheme: viewport.colorScheme,
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  const runtimeErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error" || message.type() === "warning") runtimeErrors.push(message.text());
  });
  page.on("pageerror", (error) => runtimeErrors.push(error.message));
  await page.goto(hostUrl, { waitUntil: "domcontentloaded" });
  await page.frameLocator("#widget").locator('#diagram-status[data-state="ready"]').waitFor({
    state: "attached",
    timeout: 10000,
  });
  lanes.push({ viewport, context, page, runtimeErrors });
}

const results = [];
for (let index = 0; index < cases.length; index += 1) {
  if (index > 0 && index % 40 === 0) {
    await Promise.all(lanes.map((lane) => resetLane(lane)));
  }
  const current = cases[index];
  const caseDir = path.join(artifactsRoot, current.id);
  await mkdir(caseDir, { recursive: true });
  const laneResults = await Promise.all(
    lanes.map(async (lane) => renderCase(lane, current, caseDir, index === 0)),
  );
  const passed = laneResults.every((item) => item.passed);
  results.push({
    id: current.id,
    kind: current.kind,
    category: current.category,
    intent: current.intent,
    risk_tags: current.risk_tags,
    passed,
    viewports: laneResults,
  });
  console.log(`[${passed ? "PASS" : "FAIL"}] ${current.id} ${current.category ?? ""}`);
}

const report = {
  generated_at: new Date().toISOString(),
  total: results.length,
  passed: results.filter((item) => item.passed).length,
  failed: results.filter((item) => !item.passed).length,
  viewports,
  case_files: caseFiles,
  results,
};
await writeFile(path.join(reportsRoot, "render-report.json"), `${JSON.stringify(report, null, 2)}\n`);
await writeIndex(cases, results);
await writeContactSheets(cases);

for (const lane of lanes) await lane.context.close();
await browser.close();
await new Promise((resolve) => server.close(resolve));
console.log(`SUMMARY total=${report.total} passed=${report.passed} failed=${report.failed}`);
if (report.failed > 0) process.exitCode = 1;

async function renderCase(lane, current, caseDir, verifyControls) {
  const { page, viewport, runtimeErrors } = lane;
  const errorStart = runtimeErrors.length;
  const payload = { source: current.source, title: current.id, theme: "auto" };
  await page.evaluate((next) => window.renderCase(next), payload);
  const frame = page.frameLocator("#widget");
  const titleLocator = frame.locator("#diagram-title");
  await titleLocator.filter({ hasText: current.id }).waitFor({ state: "visible", timeout: 8000 });
  await frame.locator('#diagram-status[data-state="ready"]').waitFor({ state: "attached", timeout: 8000 });
  await page.waitForTimeout(30);

  let controls = null;
  if (verifyControls) {
    const sourceButton = frame.getByLabel("Show Mermaid source", { exact: true });
    await sourceButton.click();
    const sourceVisible = await frame.locator("#source-panel").isVisible();
    const sourceMatches = (await frame.locator("#source-code").innerText()) === current.source;
    await frame.getByLabel("Show diagram", { exact: true }).click();
    const beforeZoom = await frame.locator("#zoom-value").innerText();
    await frame.getByLabel("Zoom in", { exact: true }).click();
    const afterZoom = await frame.locator("#zoom-value").innerText();
    await frame.getByLabel("Fit diagram", { exact: true }).click();
    controls = { sourceVisible, sourceMatches, beforeZoom, afterZoom };
  }

  const metrics = await frame.locator("#diagram-widget").evaluate((root) => {
    const svg = root.querySelector("svg");
    const nodes = [...root.querySelectorAll("svg .node")].map((node) => {
      const rect = node.getBoundingClientRect();
      return { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom };
    });
    let nodeOverlapCount = 0;
    for (let left = 0; left < nodes.length; left += 1) {
      for (let right = left + 1; right < nodes.length; right += 1) {
        const a = nodes[left];
        const b = nodes[right];
        const overlapWidth = Math.min(a.right, b.right) - Math.max(a.left, b.left);
        const overlapHeight = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (overlapWidth > 1 && overlapHeight > 1) nodeOverlapCount += 1;
      }
    }
    const rootRect = root.getBoundingClientRect();
    const controlsInside = [...root.querySelectorAll("button")].every((button) => {
      const rect = button.getBoundingClientRect();
      return rect.left >= rootRect.left - 1 && rect.right <= rootRect.right + 1;
    });
    return {
      hasSvg: Boolean(svg),
      svgText: svg?.textContent?.trim() ?? "",
      shapeCount: svg?.querySelectorAll("path,rect,circle,ellipse,polygon,line").length ?? 0,
      sourceMatches: root.querySelector("#source-code")?.textContent?.length > 0,
      horizontalOverflow: root.scrollWidth > root.clientWidth + 1,
      controlsInside,
      fullTitleAffordance:
        root.querySelector("#diagram-title")?.getAttribute("title") ===
        root.querySelector("#diagram-title")?.textContent,
      nodeCount: nodes.length,
      emptyNodeLabelCount: [...root.querySelectorAll("svg .node")].filter(
        (node) =>
          node.querySelector(".label,.nodeLabel") &&
          !(node.textContent ?? "").trim(),
      ).length,
      nodeOverlapCount,
      status: root.querySelector("#diagram-status")?.getAttribute("data-state"),
    };
  });
  const errors = runtimeErrors.slice(errorStart);
  const screenshotPath = path.join(caseDir, `${viewport.name}.png`);
  await page.locator("#widget").screenshot({ path: screenshotPath });
  let zoomEvidence = null;
  const fittedPercent = Number.parseInt(await frame.locator("#zoom-value").innerText(), 10);
  if (viewport.name === "360-dark" && fittedPercent < 50) {
    await frame.getByLabel("Reset to 100%", { exact: true }).click();
    const resetPercent = Number.parseInt(await frame.locator("#zoom-value").innerText(), 10);
    const resetScreenshot = path.join(caseDir, "360-dark-100.png");
    await page.locator("#widget").screenshot({ path: resetScreenshot });
    const viewportBox = await frame.locator("#diagram-viewport").boundingBox();
    const beforePanTransform = await frame.locator("#diagram-surface").getAttribute("style");
    if (viewportBox) {
      const centerX = viewportBox.x + viewportBox.width / 2;
      const centerY = viewportBox.y + viewportBox.height / 2;
      await page.mouse.move(centerX, centerY);
      await page.mouse.down();
      await page.mouse.move(centerX + 80, centerY + 60, { steps: 5 });
      await page.mouse.up();
    }
    const afterPanTransform = await frame.locator("#diagram-surface").getAttribute("style");
    const selectionCollapsed = await frame
      .locator("#diagram-viewport")
      .evaluate(() => window.getSelection()?.isCollapsed ?? true);
    const panScreenshot = path.join(caseDir, "360-dark-pan.png");
    await page.locator("#widget").screenshot({ path: panScreenshot });
    zoomEvidence = {
      fittedPercent,
      resetPercent,
      panChanged: beforePanTransform !== afterPanTransform,
      selectionCollapsed,
      resetScreenshot,
      panScreenshot,
    };
    await frame.getByLabel("Fit diagram", { exact: true }).click();
  }
  if (viewport.name === "1024-dark") {
    const svg = await frame.locator("#diagram-surface svg").evaluate((node) => node.outerHTML);
    await writeFile(path.join(caseDir, "diagram.svg"), `${svg}\n`);
  }
  const passed =
    metrics.hasSvg &&
    metrics.shapeCount > 0 &&
    metrics.svgText.length > 0 &&
    metrics.sourceMatches &&
    !metrics.horizontalOverflow &&
    metrics.controlsInside &&
    metrics.fullTitleAffordance &&
    metrics.emptyNodeLabelCount === 0 &&
    metrics.nodeOverlapCount === 0 &&
    metrics.status === "ready" &&
    errors.length === 0 &&
    (!zoomEvidence ||
      (zoomEvidence.resetPercent === 100 &&
        zoomEvidence.panChanged &&
        zoomEvidence.selectionCollapsed)) &&
    (!controls || (controls.sourceVisible && controls.sourceMatches && controls.beforeZoom !== controls.afterZoom));
  return { ...viewport, passed, metrics, controls, zoomEvidence, errors, screenshot: screenshotPath };
}

async function resetLane(lane) {
  lane.runtimeErrors.length = 0;
  await lane.page.goto(hostUrl, { waitUntil: "domcontentloaded" });
  await lane.page.frameLocator("#widget").locator('#diagram-status[data-state="ready"]').waitFor({
    state: "attached",
    timeout: 10000,
  });
}

async function writeIndex(caseItems, resultItems) {
  const byId = new Map(resultItems.map((item) => [item.id, item]));
  const cards = caseItems
    .map((item) => {
      const result = byId.get(item.id);
      const source = escapeHtml(item.source);
      return `<article class="case ${result.passed ? "pass" : "fail"}">
  <header><strong>${escapeHtml(item.id)}</strong><span>${escapeHtml(item.kind)}</span><span>${result.passed ? "PASS" : "FAIL"}</span></header>
  <img src="artifacts/${encodeURIComponent(item.id)}/1024-dark.png" alt="${escapeHtml(item.id)} rendered diagram">
  <nav><a href="artifacts/${encodeURIComponent(item.id)}/736-light.png">736 light</a><a href="artifacts/${encodeURIComponent(item.id)}/360-dark.png">360 dark</a><a href="artifacts/${encodeURIComponent(item.id)}/diagram.svg">SVG</a></nav>
  <details><summary>${escapeHtml(item.category ?? "source")}</summary><pre>${source}</pre></details>
</article>`;
    })
    .join("\n");
  const html = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Mermaid MCP QA</title><style>
  :root{color-scheme:light dark;font-family:system-ui,sans-serif}body{margin:0;padding:16px;background:#111;color:#eee}h1{font-size:20px}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:12px}.case{min-width:0;border:1px solid #444;padding:8px;background:#18181b}.case.fail{border-color:#ef4444}header,nav{display:flex;gap:10px;align-items:center;flex-wrap:wrap}header span:last-child{margin-left:auto}img{display:block;width:100%;height:260px;object-fit:contain;background:#111;margin:8px 0}a{color:#8ab4f8}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:11px}</style></head><body><h1>Mermaid MCP QA — ${resultItems.filter((item) => item.passed).length}/${resultItems.length} passed</h1><main class="grid">${cards}</main></body></html>`;
  await writeFile(path.join(qaRoot, "index.html"), html);
}

async function writeContactSheets(caseItems) {
  const groups = new Map();
  for (const item of caseItems) {
    const group = groups.get(item.kind) ?? [];
    group.push(item);
    groups.set(item.kind, group);
  }
  const context = await browser.newContext({ viewport: { width: 1200, height: 800 }, colorScheme: "dark" });
  const page = await context.newPage();
  for (const [kind, items] of groups) {
    const cells = [];
    for (const item of items) {
      const data = await readFile(path.join(artifactsRoot, item.id, "1024-dark.png"));
      cells.push(`<figure><img src="data:image/png;base64,${data.toString("base64")}"><figcaption>${escapeHtml(item.id)}</figcaption></figure>`);
    }
    await page.setContent(`<!doctype html><style>html,body{margin:0;background:#111;color:#eee;font:12px system-ui}.grid{display:grid;grid-template-columns:repeat(4,300px);gap:4px;padding:4px}figure{margin:0;width:300px;height:220px;display:grid;grid-template-rows:196px 24px;background:#18181b}img{width:100%;height:196px;object-fit:contain}figcaption{padding:3px 6px}</style><main class="grid">${cells.join("")}</main>`);
    await page.screenshot({ path: path.join(reportsRoot, `contact-${safeName(kind)}.png`), fullPage: true });
  }
  await context.close();
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function safeName(value) {
  return String(value).toLowerCase().replace(/[^a-z0-9]+/gu, "-").replace(/^-|-$/gu, "");
}
