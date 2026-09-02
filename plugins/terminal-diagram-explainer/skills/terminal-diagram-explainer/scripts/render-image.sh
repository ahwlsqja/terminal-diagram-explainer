#!/usr/bin/env bash
set -euo pipefail

image_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
image_artifacts=$("${image_script_dir}/render-artifacts.sh")
node -e '
  const artifacts = JSON.parse(process.argv[1]);
  if (!artifacts.png) process.exit(1);
  process.stdout.write(`${artifacts.png}\n`);
' "${image_artifacts}"
