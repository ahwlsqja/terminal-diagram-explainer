#!/usr/bin/env bash
set -euo pipefail

guidance_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
guidance_repo_root=$(cd -- "${guidance_script_dir}/.." && pwd)
guidance_fragment="${guidance_repo_root}/config/AGENTS.fragment.md"
if [[ -n "${CODEX_HOME:-}" ]]; then
  guidance_codex_home="${CODEX_HOME}"
elif [[ -n "${HOME:-}" ]]; then
  guidance_codex_home="${HOME}/.codex"
else
  printf 'CODEX_HOME 또는 HOME이 필요합니다.\n' >&2
  exit 1
fi
guidance_target="${guidance_codex_home}/AGENTS.md"
guidance_backup="${guidance_codex_home}/AGENTS.md.terminal-diagram-explainer.bak"
guidance_temp_root="${TMPDIR:-/tmp}"
guidance_temp=$(mktemp "${guidance_temp_root%/}/terminal-diagram-agents.XXXXXX")
trap 'rm -f "${guidance_temp}"' EXIT

if [[ ! -f "${guidance_fragment}" ]]; then
  printf '전역 지침 fragment가 없습니다: %s\n' "${guidance_fragment}" >&2
  exit 1
fi

mkdir -p "${guidance_codex_home}"

if [[ -f "${guidance_target}" ]]; then
  if [[ ! -f "${guidance_backup}" ]]; then
    cp "${guidance_target}" "${guidance_backup}"
  fi
  awk '
    $0 == "<!-- LOCAL:TERMINAL-DIAGRAM-EXPLANATION:START -->" { skipping = 1; next }
    $0 == "<!-- LOCAL:TERMINAL-DIAGRAM-EXPLANATION:END -->" { skipping = 0; next }
    !skipping && $0 == "" { blanks++; next }
    !skipping {
      for (i = 0; i < blanks; i++) print ""
      blanks = 0
      print
    }
  ' "${guidance_target}" > "${guidance_temp}"
else
  : > "${guidance_temp}"
fi

if [[ -s "${guidance_temp}" ]]; then
  printf '\n\n' >> "${guidance_temp}"
fi
awk '{ print }' "${guidance_fragment}" >> "${guidance_temp}"
mv "${guidance_temp}" "${guidance_target}"
trap - EXIT

printf '전역 Codex 지침 설치 완료: %s\n' "${guidance_target}"
