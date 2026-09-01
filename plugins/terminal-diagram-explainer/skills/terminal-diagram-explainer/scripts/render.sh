#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${CODEX_HOME:-}" ]]; then
  render_codex_home="${CODEX_HOME}"
elif [[ -n "${HOME:-}" ]]; then
  render_codex_home="${HOME}/.codex"
else
  printf 'CODEX_HOME 또는 HOME이 필요합니다.\n' >&2
  exit 1
fi
render_binary="${render_codex_home}/bin/term-diagram"

if [[ ! -x "${render_binary}" ]]; then
  printf 'term-diagram renderer가 설치되지 않았습니다: %q\n' "${render_binary}" >&2
  exit 127
fi

case "${1:-}" in
  "") exec "${render_binary}" -width 120 -fit ;;
  --ascii|-ascii) exec "${render_binary}" -ascii -width 120 -fit ;;
  --svg) exec "${render_binary}" -format svg -width 120 -fit ;;
  --html) exec "${render_binary}" -format html -width 120 -fit ;;
  *)
    printf '지원하지 않는 renderer 인자: %q\n' "$1" >&2
    exit 2
    ;;
esac
