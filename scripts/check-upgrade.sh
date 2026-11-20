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
node scripts/verify-graceful-update.mjs "$fixtures/old" "$fixtures/next" v3.0.1
node scripts/verify-graceful-update.mjs "$fixtures/old" "$fixtures/next" v3.0.1 --via-installer
