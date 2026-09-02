#!/usr/bin/env bash
set -euo pipefail
umask 077

mermaid_install_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
mermaid_install_repo_root=$(cd -- "${mermaid_install_script_dir}/.." && pwd)
mermaid_runtime_source="${mermaid_install_repo_root}/tools/mermaid-cli"

if [[ -n "${CODEX_HOME:-}" ]]; then
  mermaid_codex_home="${CODEX_HOME}"
elif [[ -n "${HOME:-}" ]]; then
  mermaid_codex_home="${HOME}/.codex"
else
  printf 'CODEX_HOME 또는 HOME이 필요합니다.\n' >&2
  exit 1
fi

mermaid_runtime_root="${mermaid_codex_home}/lib/terminal-diagram-explainer/mermaid-cli-runtime"
mermaid_releases_root="${mermaid_runtime_root}/releases"
mermaid_pointer="${mermaid_runtime_root}/runtime.json"

if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
  printf 'Node.js 22.12 이상과 npm이 필요합니다.\n' >&2
  exit 127
fi
if ! node -e '
  const [major, minor] = process.versions.node.split(".").map(Number);
  process.exit(major > 22 || (major === 22 && minor >= 12) ? 0 : 1);
'; then
  printf 'Node.js 22.12 이상이 필요합니다. 현재=%s\n' "$(node --version)" >&2
  exit 1
fi
if [[ ! -f "${mermaid_runtime_source}/package-lock.json" ]]; then
  printf '고정된 mermaid-cli package-lock.json이 없습니다.\n' >&2
  exit 1
fi

mkdir -p "${mermaid_releases_root}"
mermaid_lock_digest=$(node -e '
  const { createHash } = require("node:crypto");
  const { readFileSync } = require("node:fs");
  process.stdout.write(createHash("sha256").update(readFileSync(process.argv[1])).digest("hex").slice(0, 16));
' "${mermaid_runtime_source}/package-lock.json")
mermaid_release="${mermaid_releases_root}/mmdc-11.16.0-puppeteer-25.9.0-${mermaid_lock_digest}"
mermaid_stage=""
mermaid_health_root=""
mermaid_health_output=""

mermaid_cleanup() {
  if [[ -n "${mermaid_stage}" && -d "${mermaid_stage}" ]]; then
    rm -rf -- "${mermaid_stage}"
  fi
  if [[ -n "${mermaid_health_root}" && -d "${mermaid_health_root}" ]]; then
    rm -rf -- "${mermaid_health_root}"
  fi
}

mermaid_health_check() {
  local release_root=$1
  local release_mmdc="${release_root}/node_modules/.bin/mmdc"
  local release_cache="${release_root}/.cache/puppeteer"
  [[ -x "${release_mmdc}" && -d "${release_cache}" ]] || return 1
  mermaid_health_root=$(mktemp -d "${TMPDIR:-/tmp}/terminal-diagram-mermaid-health.XXXXXX")
  printf '%s\n' 'flowchart LR' 'Health[Health check] --> Ready[Ready]' >"${mermaid_health_root}/health.mmd"
  mermaid_health_output="${mermaid_health_root}/health.png"
  PUPPETEER_CACHE_DIR="${release_cache}" "${release_mmdc}" \
    --input "${mermaid_health_root}/health.mmd" \
    --output "${mermaid_health_output}" \
    --outputFormat png \
    --theme neutral \
    --backgroundColor white \
    --quiet >/dev/null 2>&1
  local health_status=$?
  if [[ ${health_status} -eq 0 && -s "${mermaid_health_output}" ]]; then
    rm -rf -- "${mermaid_health_root}"
    mermaid_health_root=""
    return 0
  fi
  rm -rf -- "${mermaid_health_root}"
  mermaid_health_root=""
  return 1
}

mermaid_write_pointer() {
  local release_root=$1
  local pointer_temp
  pointer_temp=$(mktemp "${mermaid_runtime_root}/runtime.json.XXXXXX")
  node -e '
    const { writeFileSync } = require("node:fs");
    const path = require("node:path");
    const [file, release] = process.argv.slice(1);
    writeFileSync(file, `${JSON.stringify({
      mmdcVersion: "11.16.0",
      puppeteerVersion: "25.9.0",
      binary: path.join(release, "node_modules", ".bin", process.platform === "win32" ? "mmdc.cmd" : "mmdc"),
      cacheDir: path.join(release, ".cache", "puppeteer"),
    }, null, 2)}\n`, { mode: 0o600 });
  ' "${pointer_temp}" "${release_root}"
  mv "${pointer_temp}" "${mermaid_pointer}"
}

if mermaid_health_check "${mermaid_release}"; then
  mermaid_write_pointer "${mermaid_release}"
  printf '개인 전용 Mermaid CLI가 이미 정상입니다: %s\n' "${mermaid_release}"
  exit 0
fi

mermaid_stage=$(mktemp -d "${mermaid_releases_root}/.staging.XXXXXX")
trap mermaid_cleanup EXIT
cp "${mermaid_runtime_source}/package.json" "${mermaid_runtime_source}/package-lock.json" "${mermaid_stage}/"
(
  cd "${mermaid_stage}"
  npm ci --omit=dev --ignore-scripts --no-audit --no-fund
  PUPPETEER_CACHE_DIR="${mermaid_stage}/.cache/puppeteer" \
    "${mermaid_stage}/node_modules/.bin/puppeteer" browsers install chrome-headless-shell
)

mermaid_mmdc="${mermaid_stage}/node_modules/.bin/mmdc"
mermaid_version=$(PUPPETEER_CACHE_DIR="${mermaid_stage}/.cache/puppeteer" "${mermaid_mmdc}" --version)
if [[ "${mermaid_version}" != "11.16.0" ]]; then
  printf '예상하지 않은 mermaid-cli 버전: %s\n' "${mermaid_version}" >&2
  exit 1
fi
if ! mermaid_health_check "${mermaid_stage}"; then
  printf 'Mermaid CLI smoke render가 실패했습니다. 기존 정상 runtime은 유지합니다.\n' >&2
  exit 1
fi
if [[ -e "${mermaid_release}" ]]; then
  mermaid_release="${mermaid_releases_root}/mmdc-11.16.0-puppeteer-25.9.0-${mermaid_lock_digest}-repair-$(date -u +%Y%m%d%H%M%S)"
fi
mv "${mermaid_stage}" "${mermaid_release}"
mermaid_stage=""
mermaid_write_pointer "${mermaid_release}"
trap - EXIT
printf '개인 전용 Mermaid CLI 설치 완료: %s (%s)\n' "${mermaid_release}" "${mermaid_version}"
