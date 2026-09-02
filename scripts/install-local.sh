#!/usr/bin/env bash
set -euo pipefail

install_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
install_repo_root=$(cd -- "${install_script_dir}/.." && pwd)
if [[ -n "${CODEX_HOME:-}" ]]; then
  install_codex_home="${CODEX_HOME}"
elif [[ -n "${HOME:-}" ]]; then
  install_codex_home="${HOME}/.codex"
else
  printf 'CODEX_HOME 또는 HOME이 필요합니다.\n' >&2
  exit 1
fi
install_target_dir="${install_codex_home}/bin"
install_target="${install_target_dir}/term-diagram"
install_temp_root="${TMPDIR:-/tmp}"
install_temp=$(mktemp "${install_temp_root%/}/term-diagram.XXXXXX")
trap 'rm -f "${install_temp}"' EXIT

mkdir -p "${install_target_dir}"

if ! command -v go >/dev/null 2>&1; then
  printf 'Go 1.25 이상이 필요합니다.\n' >&2
  exit 1
fi

(
  cd "${install_repo_root}"
  GOTOOLCHAIN=local GOPROXY=off go test ./...
  CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off \
    go build -trimpath -buildvcs=false -o "${install_temp}" ./cmd/term-diagram
)

chmod 0755 "${install_temp}"
mv "${install_temp}" "${install_target}"
trap - EXIT

"${install_target}" -version
"${install_script_dir}/install-mermaid-cli.sh"
