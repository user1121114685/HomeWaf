#!/usr/bin/env bash
# HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.
# HomeWaf builds the checked-in frontend and pinned lockfile locally.
set -euo pipefail
cd "$(dirname "$0")/../.."
node scripts/build.mjs --web-only
