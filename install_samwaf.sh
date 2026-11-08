#!/usr/bin/env bash
# HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.
# Legacy filename retained only to point operators to the renamed installer.
set -euo pipefail
exec bash "$(dirname "$0")/install_homewaf.sh" "$@"
