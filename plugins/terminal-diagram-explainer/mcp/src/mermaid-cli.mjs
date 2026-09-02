import { spawnSync } from "node:child_process";
import {
  accessSync,
  constants as fsConstants,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { validateRenderInput } from "./source-policy.mjs";

export const MMDC_VERSION = "11.16.0";

const MAX_ARTIFACT_BYTES = 8 * 1024 * 1024;
const RENDER_TIMEOUT_MS = 20_000;
const mcpRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const configPath = path.join(mcpRoot, "config", "mermaid-cli.json");

function codexHome() {
  return process.env.CODEX_HOME || path.join(os.homedir(), ".codex");
}

function configuredCommand() {
  const binary = process.env.TERMINAL_DIAGRAM_MMDC_BIN;
  if (!binary) return null;
  const raw = process.env.TERMINAL_DIAGRAM_MMDC_PREFIX_ARGS_JSON;
  let prefixArgs = [];
  if (raw) {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed) || !parsed.every((item) => typeof item === "string")) {
      throw new TypeError("TERMINAL_DIAGRAM_MMDC_PREFIX_ARGS_JSON must be a string array");
    }
    prefixArgs = parsed;
  }
  return {
    binary,
    prefixArgs,
    cacheDir: process.env.TERMINAL_DIAGRAM_PUPPETEER_CACHE_DIR || null,
  };
}

function installedCommand() {
  const runtimeRoot = path.join(
    codexHome(),
    "lib",
    "terminal-diagram-explainer",
    "mermaid-cli-runtime",
  );
  const releasesRoot = path.join(runtimeRoot, "releases");
  const pointerPath = path.join(runtimeRoot, "runtime.json");
  const pointer = JSON.parse(readFileSync(pointerPath, "utf8"));
  if (
    pointer?.mmdcVersion !== MMDC_VERSION ||
    typeof pointer.binary !== "string" ||
    typeof pointer.cacheDir !== "string" ||
    !path.isAbsolute(pointer.binary) ||
    !path.isAbsolute(pointer.cacheDir)
  ) {
    throw new TypeError("invalid Mermaid CLI runtime pointer");
  }
  const releasePrefix = `${path.resolve(releasesRoot)}${path.sep}`;
  if (
    !path.resolve(pointer.binary).startsWith(releasePrefix) ||
    !path.resolve(pointer.cacheDir).startsWith(releasePrefix)
  ) {
    throw new TypeError("Mermaid CLI runtime pointer escapes its release root");
  }
  return { binary: pointer.binary, prefixArgs: [], cacheDir: pointer.cacheDir };
}

export function resolveMermaidCli() {
  const candidates = [];
  try {
    const configured = configuredCommand();
    if (configured) candidates.push(configured);
  } catch {
    // Invalid explicit configuration must not weaken the installed runtime boundary.
  }
  try {
    candidates.push(installedCommand());
  } catch {
    // A missing or invalid pointer means the optional runtime is unavailable.
  }
  for (const command of candidates) {
    try {
      accessSync(command.binary, fsConstants.X_OK);
      accessSync(command.cacheDir ?? path.dirname(command.binary), fsConstants.R_OK);
      const version = spawnSync(command.binary, [...command.prefixArgs, "--version"], {
        encoding: "utf8",
        timeout: 3_000,
        maxBuffer: 128 * 1024,
        windowsHide: true,
      });
      if (version.status === 0 && version.stdout.trim() === MMDC_VERSION) {
        return command;
      }
    } catch {
      // Ignore malformed or incompatible candidates and keep the deterministic fallback available.
    }
  }
  return null;
}

function assertArtifact(format, bytes) {
  if (bytes.length === 0 || bytes.length > MAX_ARTIFACT_BYTES) {
    throw new RangeError(`Mermaid ${format} artifact must be between 1 and ${MAX_ARTIFACT_BYTES} bytes`);
  }
  if (format === "png") {
    const signature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
    if (!bytes.subarray(0, signature.length).equals(signature)) {
      throw new TypeError("Mermaid CLI returned an invalid PNG artifact");
    }
    return;
  }
  const svg = bytes.toString("utf8");
  if (
    !/<svg(?:\s|>)/iu.test(svg) ||
    /<(?:script|foreignObject|image)(?:\s|>)/iu.test(svg)
  ) {
    throw new TypeError("Mermaid CLI returned an unsafe or invalid SVG artifact");
  }
  const hrefs = [...svg.matchAll(/(?:href|xlink:href)\s*=\s*["']([^"']*)["']/giu)];
  const cssUrls = [...svg.matchAll(/url\s*\(\s*["']?([^"')]+)["']?\s*\)/giu)];
  const hasExternalReference =
    hrefs.some((match) => !match[1].startsWith("#")) ||
    cssUrls.some((match) => !match[1].startsWith("#"));
  if (/\son[a-z]+\s*=/iu.test(svg) || hasExternalReference) {
    throw new TypeError("Mermaid CLI SVG contains an active resource reference");
  }
}

export function renderMermaidArtifact(source, { format, theme = "light" } = {}) {
  if (!new Set(["svg", "png"]).has(format)) {
    throw new TypeError("Mermaid artifact format must be svg or png");
  }
  if (!new Set(["light", "dark"]).has(theme)) {
    throw new TypeError("Mermaid artifact theme must be light or dark");
  }
  validateRenderInput({ source, theme });
  const command = resolveMermaidCli();
  if (!command) {
    const error = new Error(`@mermaid-js/mermaid-cli ${MMDC_VERSION} is not installed`);
    error.code = "MMDC_UNAVAILABLE";
    throw error;
  }

  const workDir = mkdtempSync(path.join(os.tmpdir(), "term-diagram-mmdc."));
  const inputPath = path.join(workDir, "diagram.mmd");
  const outputPath = path.join(workDir, `diagram.${format}`);
  try {
    writeFileSync(inputPath, source, { encoding: "utf8", mode: 0o600 });
    const cliTheme = theme === "dark" ? "dark" : "neutral";
    const background = theme === "dark" ? "#18181b" : "white";
    const args = [
      ...command.prefixArgs,
      "--input",
      inputPath,
      "--output",
      outputPath,
      "--outputFormat",
      format,
      "--theme",
      cliTheme,
      "--backgroundColor",
      background,
      "--configFile",
      configPath,
      "--width",
      "1600",
      "--quiet",
    ];
    if (format === "png") args.push("--scale", "1.5");
    const result = spawnSync(command.binary, args, {
      encoding: "utf8",
      timeout: RENDER_TIMEOUT_MS,
      maxBuffer: 1024 * 1024,
      windowsHide: true,
      env: command.cacheDir
        ? { ...process.env, PUPPETEER_CACHE_DIR: command.cacheDir }
        : process.env,
    });
    if (result.error || result.status !== 0) {
      const detail = result.error?.message || result.stderr.trim() || `exit ${result.status}`;
      throw new Error(`Mermaid CLI render failed: ${detail}`);
    }
    const bytes = readFileSync(outputPath);
    assertArtifact(format, bytes);
    return bytes;
  } finally {
    rmSync(workDir, { recursive: true, force: true });
  }
}
