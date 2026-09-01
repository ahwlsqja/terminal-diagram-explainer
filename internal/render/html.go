package render

import (
	"fmt"
	"strings"
)

// HTML wraps the validated SVG backend in a self-contained pan/zoom viewer.
// It does not fetch data or load external scripts; interaction is presentation-only.
func HTML(terminal string) (string, error) {
	svg, err := SVG(terminal)
	if err != nil {
		return "", err
	}
	svg = strings.Replace(svg, "<svg ", `<svg id="diagram-svg" `, 1)
	var output strings.Builder
	output.Grow(len(svg) + 8_000)
	output.WriteString(`<!doctype html>
<html lang="ko">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Interactive software flow diagram</title>
<style>
:root { color-scheme: dark; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
* { box-sizing: border-box; }
html, body { width: 100%; height: 100%; margin: 0; overflow: hidden; background: #171717; color: #f3f3f3; }
body { display: grid; grid-template-rows: auto minmax(0, 1fr); }
.diagram-toolbar { display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-bottom: 1px solid #3a3a3a; background: #202020; }
.diagram-toolbar button { min-width: 40px; min-height: 36px; padding: 6px 10px; border: 1px solid #505050; border-radius: 6px; background: #292929; color: inherit; font: inherit; cursor: pointer; }
.diagram-toolbar button:hover { background: #353535; }
.diagram-toolbar button:focus-visible { outline: 2px solid #8ab4f8; outline-offset: 2px; }
#zoom-value { min-width: 56px; text-align: center; font-variant-numeric: tabular-nums; }
#diagram-viewport { position: relative; overflow: hidden; min-width: 0; min-height: 0; touch-action: none; cursor: grab; background: #1e1e1e; }
#diagram-viewport[data-dragging="true"] { cursor: grabbing; }
#diagram-surface { position: absolute; left: 0; top: 0; transform-origin: 0 0; will-change: transform; }
#diagram-surface svg { display: block; max-width: none; max-height: none; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
</head>
<body>
<div class="diagram-toolbar" role="toolbar" aria-label="다이어그램 보기 도구">
  <button type="button" data-action="zoom-out" aria-label="다이어그램 축소">−</button>
  <button type="button" data-action="zoom-in" aria-label="다이어그램 확대">+</button>
  <button type="button" data-action="fit" aria-label="화면에 맞추기">화면 맞춤</button>
  <button type="button" data-action="reset" aria-label="원본 크기로 복원">100%</button>
  <output id="zoom-value" aria-live="polite">100%</output>
</div>
<div id="diagram-viewport" aria-label="확대·축소와 드래그가 가능한 소프트웨어 흐름도">
  <div id="diagram-surface">`)
	output.WriteString(svg)
	output.WriteString(`</div>
</div>
<p class="sr-only">마우스 휠 또는 도구 모음으로 확대하고, 다이어그램을 드래그해 이동할 수 있습니다.</p>
<script>
(() => {
  const viewport = document.getElementById('diagram-viewport');
  const surface = document.getElementById('diagram-surface');
  const svg = document.getElementById('diagram-svg');
  const zoomValue = document.getElementById('zoom-value');
  let scale = 1;
  let offsetX = 0;
  let offsetY = 0;
  let dragging = false;
  let lastX = 0;
  let lastY = 0;
  let fitted = true;

  const dimensions = () => {
    const viewBox = svg.viewBox.baseVal;
    return { width: viewBox.width || svg.width.baseVal.value, height: viewBox.height || svg.height.baseVal.value };
  };

  const apply = () => {
    surface.style.transform = 'translate(' + offsetX + 'px, ' + offsetY + 'px) scale(' + scale + ')';
    zoomValue.value = Math.round(scale * 100) + '%';
    zoomValue.textContent = zoomValue.value;
  };

  const fit = () => {
    const size = dimensions();
    const padding = 24;
    scale = Math.min((viewport.clientWidth - padding * 2) / size.width, (viewport.clientHeight - padding * 2) / size.height, 1);
    scale = Math.max(scale, 0.1);
    offsetX = (viewport.clientWidth - size.width * scale) / 2;
    offsetY = (viewport.clientHeight - size.height * scale) / 2;
    fitted = true;
    apply();
  };

  const zoomAt = (factor, clientX, clientY) => {
    const rect = viewport.getBoundingClientRect();
    const pointX = clientX - rect.left;
    const pointY = clientY - rect.top;
    const diagramX = (pointX - offsetX) / scale;
    const diagramY = (pointY - offsetY) / scale;
    scale = Math.min(4, Math.max(0.1, scale * factor));
    offsetX = pointX - diagramX * scale;
    offsetY = pointY - diagramY * scale;
    fitted = false;
    apply();
  };

  document.querySelector('[data-action="zoom-in"]').addEventListener('click', () => zoomAt(1.2, viewport.clientWidth / 2, viewport.clientHeight / 2));
  document.querySelector('[data-action="zoom-out"]').addEventListener('click', () => zoomAt(1 / 1.2, viewport.clientWidth / 2, viewport.clientHeight / 2));
  document.querySelector('[data-action="fit"]').addEventListener('click', fit);
  document.querySelector('[data-action="reset"]').addEventListener('click', () => {
    scale = 1;
    offsetX = 16;
    offsetY = 16;
    fitted = false;
    apply();
  });

  viewport.addEventListener('wheel', event => {
    event.preventDefault();
    zoomAt(event.deltaY < 0 ? 1.12 : 1 / 1.12, event.clientX, event.clientY);
  }, { passive: false });

  viewport.addEventListener('pointerdown', event => {
    dragging = true;
    lastX = event.clientX;
    lastY = event.clientY;
    viewport.dataset.dragging = 'true';
    viewport.setPointerCapture(event.pointerId);
  });
  viewport.addEventListener('pointermove', event => {
    if (!dragging) return;
    offsetX += event.clientX - lastX;
    offsetY += event.clientY - lastY;
    lastX = event.clientX;
    lastY = event.clientY;
    fitted = false;
    apply();
  });
  const endDrag = event => {
    if (!dragging) return;
    dragging = false;
    viewport.dataset.dragging = 'false';
    if (viewport.hasPointerCapture(event.pointerId)) viewport.releasePointerCapture(event.pointerId);
  };
  viewport.addEventListener('pointerup', endDrag);
  viewport.addEventListener('pointercancel', endDrag);

  new ResizeObserver(() => { if (fitted) fit(); }).observe(viewport);
  fit();
})();
</script>
</body>
</html>`)
	if output.Len() > 8*1024*1024 {
		return "", fmt.Errorf("%w: HTML output %d bytes", ErrOutputBounds, output.Len())
	}
	return output.String(), nil
}
