#!/usr/bin/env bash
set -euo pipefail

artifact_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
artifact_temp_root="${TMPDIR:-/tmp}"
artifact_work_dir=$(mktemp -d "${artifact_temp_root%/}/term-diagram-artifacts.XXXXXX")
artifact_source="${artifact_work_dir}/diagram.mmd"
artifact_svg="${artifact_work_dir}/diagram.svg"
artifact_png="${artifact_work_dir}/diagram.png"
artifact_html="${artifact_work_dir}/diagram.html"

awk '{ print }' >"${artifact_source}"
"${artifact_script_dir}/render.sh" --svg <"${artifact_source}" >"${artifact_svg}"
"${artifact_script_dir}/render.sh" --html <"${artifact_source}" >"${artifact_html}"

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

if [[ ! -s "${artifact_png}" || ! -s "${artifact_html}" ]]; then
  printf 'diagram artifact 생성 결과가 비어 있습니다.\n' >&2
  exit 1
fi

printf '{"png":"%s","html":"%s"}\n' "${artifact_png}" "${artifact_html}"
