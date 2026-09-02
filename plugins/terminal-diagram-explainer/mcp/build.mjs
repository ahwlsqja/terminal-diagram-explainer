import { build } from "esbuild";
import { readFile, mkdir, writeFile } from "node:fs/promises";
import path from "node:path";

const result = await build({
  entryPoints: ["src/widget.mjs"],
  bundle: true,
  write: false,
  format: "iife",
  platform: "browser",
  target: ["es2022"],
  minify: true,
  legalComments: "eof",
  metafile: true,
  define: { "process.env.NODE_ENV": '"production"' },
});
const javascript = result.outputFiles[0]?.text;
if (!javascript) throw new Error("widget bundle produced no JavaScript");
const safeJavascript = javascript.replace(/<\/script/giu, "<\\/script");

const html = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'none'; img-src data: blob:; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'">
<style>
:root { color-scheme: light dark; font-family: var(--font-sans, ui-sans-serif, system-ui, sans-serif); }
* { box-sizing: border-box; }
html, body { margin: 0; min-width: 0; background: transparent; color: light-dark(#171717, #f4f4f5); }
button { font: inherit; color: inherit; }
#diagram-widget { width: 100%; min-width: 0; background: transparent; }
.toolbar { display: flex; align-items: center; gap: 4px; min-height: 40px; padding: 4px 6px; border-bottom: 1px solid light-dark(#dedede, #3f3f46); }
.title { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; }
.tool-button { display: inline-grid; place-items: center; width: 34px; height: 34px; padding: 0; border: 0; background: transparent; border-radius: 4px; cursor: pointer; }
.tool-button:hover { background: light-dark(#ededed, #34343a); }
.tool-button:focus-visible { outline: 2px solid light-dark(#155eef, #8ab4f8); outline-offset: 1px; }
.tool-button[aria-pressed="true"] { background: light-dark(#dedede, #45454d); }
#zoom-value { min-width: 48px; text-align: center; font-variant-numeric: tabular-nums; }
#diagram-frame { position: relative; min-height: 320px; height: calc(100vh - 40px); max-height: 900px; overflow: hidden; }
#diagram-viewport { position: absolute; inset: 0; overflow: hidden; touch-action: none; cursor: grab; background: light-dark(#ffffff, #18181b); }
#diagram-viewport[data-dragging="true"] { cursor: grabbing; }
#diagram-viewport, #diagram-surface, #diagram-surface * { user-select: none; -webkit-user-select: none; }
#diagram-surface { position: absolute; left: 0; top: 0; transform-origin: 0 0; will-change: transform; }
#diagram-surface svg { display: block; max-width: none; max-height: none; }
#source-panel { position: absolute; inset: 0; margin: 0; padding: 12px; overflow: auto; background: light-dark(#ffffff, #18181b); }
#source-code { font-family: var(--font-mono, ui-monospace, SFMono-Regular, Consolas, monospace); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
#diagram-status { position: absolute; left: 12px; bottom: 10px; max-width: calc(100% - 24px); padding: 4px 6px; font-size: 12px; color: light-dark(#52525b, #a1a1aa); pointer-events: none; }
#diagram-status[data-state="error"] { color: light-dark(#b42318, #fda29b); }
#diagram-status:empty { display: none; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
@media (max-width: 480px) { #diagram-frame { min-height: 320px; height: calc(100vh - 76px); } .toolbar { flex-wrap: wrap; } }
</style>
</head>
<body>
<main id="diagram-widget" aria-label="Interactive software diagram">
  <div class="toolbar" role="toolbar" aria-label="Diagram controls">
    <span id="diagram-title" class="title">Software diagram</span>
    <button class="tool-button" type="button" data-action="zoom-out" aria-label="Zoom out">−</button>
    <button class="tool-button" type="button" data-action="zoom-in" aria-label="Zoom in">+</button>
    <button class="tool-button" type="button" data-action="fit" aria-label="Fit diagram">↙↗</button>
    <button class="tool-button" type="button" data-action="reset" aria-label="Reset to 100%">1:1</button>
    <button class="tool-button" type="button" data-action="source" aria-label="Show Mermaid source" aria-pressed="false">&lt;/&gt;</button>
    <output id="zoom-value" aria-live="polite">100%</output>
  </div>
  <div id="diagram-frame">
    <div id="diagram-viewport" aria-label="Pan and zoom diagram"><div id="diagram-surface"></div></div>
    <pre id="source-panel" hidden><code id="source-code"></code></pre>
    <div id="diagram-status" role="status" aria-live="polite"></div>
  </div>
  <p class="sr-only">Use the toolbar, mouse wheel, or drag gesture to inspect the diagram.</p>
</main>
<script>${safeJavascript}</script>
</body>
</html>`;

await mkdir("dist", { recursive: true });
await writeFile("dist/widget.html", html, "utf8");
await writeFile("dist/THIRD_PARTY_NOTICES.md", await createThirdPartyNotices(result.metafile), "utf8");
console.log(`Wrote dist/widget.html (${Buffer.byteLength(html)} bytes)`);

function packageNameFromInput(input) {
  const marker = "node_modules/";
  const index = input.lastIndexOf(marker);
  if (index < 0) return null;
  const parts = input.slice(index + marker.length).split("/");
  return parts[0]?.startsWith("@") ? `${parts[0]}/${parts[1]}` : parts[0];
}

async function readLicenseFiles(packageRoot) {
  const names = ["LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE", "LICENCE.md", "NOTICE", "NOTICE.md"];
  const texts = [];
  for (const name of names) {
    try {
      const text = await readFile(path.join(packageRoot, name), "utf8");
      texts.push({ name, text: text.trim() });
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
    }
  }
  if (texts.length === 0) {
    try {
      const readme = await readFile(path.join(packageRoot, "README.md"), "utf8");
      const match = readme.match(/(?:^##?\s+License\s*$|^\(The MIT License\)\s*$)[\s\S]*$/imu);
      if (match) texts.push({ name: "README.md license section", text: match[0].trim() });
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
    }
  }
  return texts;
}

async function createThirdPartyNotices(metafile) {
  const packageNames = new Set();
  for (const input of Object.keys(metafile.inputs)) {
    const name = packageNameFromInput(input);
    if (name) packageNames.add(name);
  }
  const records = [];
  for (const name of [...packageNames].sort()) {
    const packageRoot = path.join("node_modules", ...name.split("/"));
    const manifest = JSON.parse(await readFile(path.join(packageRoot, "package.json"), "utf8"));
    const licenseFiles = await readLicenseFiles(packageRoot);
    if (licenseFiles.length === 0) {
      throw new Error(`bundled package has no distributable license text: ${name}`);
    }
    records.push({
      name,
      version: manifest.version,
      license: manifest.license ?? "SEE LICENSE FILE",
      repository:
        typeof manifest.repository === "string" ? manifest.repository : manifest.repository?.url,
      licenseFiles,
    });
  }

  const lines = [
    "# Third-party notices for the bundled diagram widget",
    "",
    "This file is generated from the exact esbuild input graph. The project does not copy Paseo source code.",
    "",
  ];
  for (const record of records) {
    lines.push(`## ${record.name}@${record.version}`, "", `License: ${record.license}`);
    if (record.repository) lines.push(`Repository: ${record.repository}`);
    for (const file of record.licenseFiles) {
      lines.push("", `### ${file.name}`, "", "```text", file.text, "```");
    }
    lines.push("");
  }
  return `${lines.join("\n").trim()}\n`;
}
