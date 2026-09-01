#!/usr/bin/env bash
set -euo pipefail

image_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
image_temp_root="${TMPDIR:-/tmp}"
image_work_dir=$(mktemp -d "${image_temp_root%/}/term-diagram-image.XXXXXX")
image_svg="${image_work_dir}/diagram.svg"
image_png="${image_work_dir}/diagram.png"

"${image_script_dir}/render.sh" --svg >"${image_svg}"

if command -v sips >/dev/null 2>&1; then
  sips -s format png "${image_svg}" --out "${image_png}" >/dev/null
elif command -v rsvg-convert >/dev/null 2>&1; then
  rsvg-convert "${image_svg}" -o "${image_png}"
elif command -v magick >/dev/null 2>&1; then
  magick "${image_svg}" "${image_png}"
else
  printf 'SVG를 PNG로 변환할 로컬 도구(sips, rsvg-convert, magick)가 없습니다.\n' >&2
  exit 127
fi

if [[ ! -s "${image_png}" ]]; then
  printf 'PNG renderer가 빈 파일을 생성했습니다.\n' >&2
  exit 1
fi

printf '%s\n' "${image_png}"
