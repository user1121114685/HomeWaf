#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export CGO_ENABLED=0
fixtures=.cache/functional
mkdir -p "$fixtures" artifacts
version_flag='-X HomeWaf/internal/instance/global.GWAF_RELEASE_VERSION'
release_flag='-X HomeWaf/internal/instance/global.GWAF_RELEASE=true'
go build -tags homewaf_update_fixture -trimpath -ldflags "$release_flag $version_flag=v3.0.0-functional-test" -o "$fixtures/old" ./cmd/homewaf
go build -tags homewaf_update_fixture -trimpath -ldflags "$release_flag $version_flag=v3.0.1" -o "$fixtures/next" ./cmd/homewaf

run_upgrade_check() {
  local attempt=1
  while true; do
    if node scripts/verify-graceful-update.mjs "$@"; then
      return 0
    fi
    if [ "$attempt" -ge 3 ] || ! node --input-type=module -e '
      import { readFileSync } from "node:fs";
      try {
        const { error } = JSON.parse(readFileSync(process.argv[1], "utf8"));
        process.exit(error?.includes("bind: address already in use") ? 0 : 1);
      } catch {
        process.exit(1);
      }
    ' artifacts/graceful-update-verification.json; then
      return 1
    fi
    printf 'Retrying upgrade smoke after a port collision (%s/3)\n' "$((attempt + 1))" >&2
    attempt=$((attempt + 1))
  done
}

run_upgrade_check "$fixtures/old" "$fixtures/next" v3.0.1
run_upgrade_check "$fixtures/old" "$fixtures/next" v3.0.1 --via-installer
