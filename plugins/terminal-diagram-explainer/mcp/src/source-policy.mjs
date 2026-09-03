const encoder = new TextEncoder();

export const MAX_MERMAID_BYTES = 256 * 1024;
export const MAX_MERMAID_LINES = 2048;

const controlCharacters = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f\u200b\u200e\u200f\u202a-\u202e\u2066-\u2069\ufeff]/u;
const externalOrConfigSyntax =
  /@\s*\{|url\s*\(|@import\b|themecss\b|%%\s*\{|\b(?:href|link)\s+["']?https?:/iu;
const clickDirective = /^\s*click\s+[A-Za-z_][\w-]*(?:\s|$)/imu;
const encodedMarkup = /&#(?:x[0-9a-f]+|[0-9]+);?/iu;
const activeMarkup = /<(?!\/?(?:br|i)\s*\/?>)[a-z!/][^>]*>/iu;

function decodeCodePoint(value) {
  if (!Number.isInteger(value) || value < 0 || value > 0x10ffff) {
    throw new RangeError("invalid Unicode code point");
  }
  return String.fromCodePoint(value);
}

function normalizeEscapes(source) {
  try {
    return source
      .replace(/\\u\{([0-9a-f]{1,6})\}/giu, (_, hex) => decodeCodePoint(Number.parseInt(hex, 16)))
      .replace(/\\U([0-9a-f]{8})/gu, (_, hex) => decodeCodePoint(Number.parseInt(hex, 16)))
      .replace(/\\u([0-9a-f]{4})/giu, (_, hex) => String.fromCharCode(Number.parseInt(hex, 16)))
      .replace(/\\x([0-9a-f]{2})/giu, (_, hex) => String.fromCharCode(Number.parseInt(hex, 16)))
      .replace(/["'`\\]/gu, "");
  } catch {
    return null;
  }
}

function matchesUnsafeSyntax(source) {
  const withoutStereotypes = source.replace(/<<[A-Za-z][\w-]*>>/gu, "");
  return (
    externalOrConfigSyntax.test(source) ||
    clickDirective.test(source) ||
    encodedMarkup.test(source) ||
    activeMarkup.test(withoutStereotypes)
  );
}

export function containsUnsafeMermaidSource(source) {
  if (typeof source !== "string" || matchesUnsafeSyntax(source)) return true;
  const normalized = normalizeEscapes(source);
  return normalized === null || matchesUnsafeSyntax(normalized);
}

function normalizeQuotedFlowLabels(source) {
  const lines = source.split("\n");
  const header = lines.find((line) => {
    const trimmed = line.trimStart();
    return trimmed !== "" && !trimmed.startsWith("%%");
  });
  if (!header || !/^(?:flowchart|graph)\s+/u.test(header.trimStart())) return source;
  return lines
    .map((line) => {
      if (line.trimStart().startsWith("%%")) return line;
      let normalized = line;
      for (let edge = 0; edge < 16; edge += 1) {
        const match = normalized.match(/^(.*\S)\s+--\s+"([^"|\r\n]*)"\s+(-->|-\.->)\s+(\S.*)$/u);
        if (!match) break;
        const [, left, label, arrow, right] = match;
        normalized = `${left} ${arrow}|${label}| ${right}`;
      }
      return normalized;
    })
    .join("\n");
}

export function validateRenderInput(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new TypeError("render arguments must be an object");
  }
  const unexpected = Object.keys(value).filter((key) => !new Set(["source", "title", "theme"]).has(key));
  if (unexpected.length > 0) {
    throw new TypeError(`unexpected render argument: ${unexpected[0]}`);
  }
  const source = value.source;
  if (typeof source !== "string" || source.trim() === "") {
    throw new TypeError("source must be a non-empty string");
  }
  if (encoder.encode(source).byteLength > MAX_MERMAID_BYTES) {
    throw new RangeError(`source size limit exceeded: ${MAX_MERMAID_BYTES} bytes`);
  }
  if (source.split("\n").length > MAX_MERMAID_LINES) {
    throw new RangeError(`source line limit exceeded: ${MAX_MERMAID_LINES}`);
  }
  if (controlCharacters.test(source)) {
    throw new TypeError("source contains a forbidden control or bidi character");
  }
  if (containsUnsafeMermaidSource(source)) {
    throw new TypeError("unsafe Mermaid source is not allowed");
  }

  const title = value.title === undefined ? "Software diagram" : value.title;
  if (typeof title !== "string" || title.trim() === "" || title.length > 80) {
    throw new TypeError("title must be a non-empty string of at most 80 characters");
  }
  if (controlCharacters.test(title)) {
    throw new TypeError("title contains a forbidden control or bidi character");
  }

  const theme = value.theme ?? "auto";
  if (!new Set(["auto", "light", "dark"]).has(theme)) {
    throw new TypeError("theme must be auto, light, or dark");
  }

  return { source: normalizeQuotedFlowLabels(source), title: title.trim(), theme };
}
