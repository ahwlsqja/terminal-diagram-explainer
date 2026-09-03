export function buildStandaloneHtml(widgetHtml, payload) {
  const scriptIndex = widgetHtml.lastIndexOf("<script>");
  if (scriptIndex < 0) throw new Error("widget bundle has no inline script marker");
  const payloadScript = `<script>window.__TERMINAL_DIAGRAM_STANDALONE_PAYLOAD__=${scriptSafeJson(payload)};</script>\n`;
  return `${widgetHtml.slice(0, scriptIndex)}${payloadScript}${widgetHtml.slice(scriptIndex)}`;
}

function scriptSafeJson(value) {
  return JSON.stringify(value)
    .replaceAll("<", "\\u003c")
    .replaceAll(">", "\\u003e")
    .replaceAll("&", "\\u0026")
    .replaceAll("\u2028", "\\u2028")
    .replaceAll("\u2029", "\\u2029");
}
