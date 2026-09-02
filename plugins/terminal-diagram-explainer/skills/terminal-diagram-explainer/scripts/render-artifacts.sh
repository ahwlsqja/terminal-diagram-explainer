#!/usr/bin/env bash
set -euo pipefail
umask 077

artifact_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
artifact_mcp_root=$(cd -- "${artifact_script_dir}/../../../mcp" && pwd)
artifact_temp_root="${TMPDIR:-/tmp}"
artifact_work_dir=$(mktemp -d "${artifact_temp_root%/}/term-diagram-artifacts.XXXXXX")
artifact_source="${artifact_work_dir}/diagram.mmd"
artifact_svg="${artifact_work_dir}/diagram.svg"
artifact_png="${artifact_work_dir}/diagram.png"
artifact_html="${artifact_work_dir}/diagram.html"
artifact_renderer="mermaid-cli"

node "${artifact_mcp_root}/src/render-standalone.mjs" \
  --title "Software diagram" \
  --theme light \
  --source-output "${artifact_source}" >"${artifact_html}"

if ! node "${artifact_mcp_root}/src/render-mermaid.mjs" \
  --input "${artifact_source}" \
  --svg "${artifact_svg}" \
  --png "${artifact_png}" \
  --theme light >/dev/null 2>"${artifact_work_dir}/mmdc.stderr"; then
  artifact_renderer="terminal-svg"
  printf 'mermaid-cli 렌더링을 사용할 수 없어 bounded Go renderer로 전환합니다.\n' >&2
  "${artifact_script_dir}/render.sh" --svg <"${artifact_source}" >"${artifact_svg}"
  if command -v sips >/dev/null 2>&1; then
    sips -s format png "${artifact_svg}" --out "${artifact_png}" >/dev/null
  elif command -v rsvg-convert >/dev/null 2>&1; then
    rsvg-convert "${artifact_svg}" -o "${artifact_png}"
  elif command -v magick >/dev/null 2>&1; then
    magick "${artifact_svg}" "${artifact_png}"
  else
    printf 'SVG를 PNG로 변환할 로컬 도구(sips, rsvg-convert, magick)가 없습니다.\n' >&2
    exit 127
  fi
fi

if [[ ! -s "${artifact_png}" || ! -s "${artifact_html}" ]]; then
  printf 'diagram artifact 생성 결과가 비어 있습니다.\n' >&2
  exit 1
fi

node -e 'process.stdout.write(`${JSON.stringify({png:process.argv[1],svg:process.argv[2],html:process.argv[3],renderer:process.argv[4]})}\n`)' \
  "${artifact_png}" "${artifact_svg}" "${artifact_html}" "${artifact_renderer}"
