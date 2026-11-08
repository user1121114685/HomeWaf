#!/usr/bin/env bash
# HomeWaf installer. No default vendor URL; verify publisher checksum first.
set -euo pipefail
archive=${1:-}
checksum=${2:-}
destination=${3:-/opt/homewaf}
if [[ -z "$archive" || ! "$checksum" =~ ^[0-9a-fA-F]{64}$ ]]; then
  echo 'Usage: bash install_homewaf.sh ARCHIVE_OR_HTTPS_URL SHA256 [ABSOLUTE_DESTINATION]' >&2
  echo 'Stop the old service and back up conf/data/ssl before installing.' >&2
  exit 2
fi
[[ "$destination" = /* && "$destination" != / ]] || { echo 'Destination must be an absolute application directory' >&2; exit 2; }
temporary=$(mktemp -d)
trap 'rm -rf -- "$temporary"' EXIT
if [[ "$archive" = https://* ]]; then
  curl --fail --location --proto '=https' --proto-redir '=https' --max-time 300 "$archive" -o "$temporary/package.tar.gz"
elif [[ "$archive" = *://* ]]; then
  echo 'Only HTTPS downloads are accepted' >&2; exit 2
else
  cp -- "$archive" "$temporary/package.tar.gz"
fi
printf '%s  %s\n' "$checksum" "$temporary/package.tar.gz" | sha256sum --check --status
tar -tzf "$temporary/package.tar.gz" > "$temporary/entries"
if grep -Eq '(^/|(^|/)\.\.(/|$)|\\)' "$temporary/entries"; then
  echo 'Unsafe archive paths' >&2; exit 1
fi
tar -tvzf "$temporary/package.tar.gz" > "$temporary/types"
if grep -Eq '^[lh]' "$temporary/types"; then
  echo 'Archive links are not accepted' >&2; exit 1
fi
mkdir "$temporary/unpacked"
tar --no-same-owner --no-same-permissions -xzf "$temporary/package.tar.gz" -C "$temporary/unpacked"
case $(uname -m) in
  x86_64) binary=HomeWafLinux64 ;;
  aarch64|arm64) binary=HomeWafLinuxArm64 ;;
  *) echo 'Unsupported architecture' >&2; exit 1 ;;
esac
for required in "$binary" LICENSE NOTICE THIRD_PARTY_NOTICES.md LICENSES; do
  [[ -e "$temporary/unpacked/$required" ]] || { echo "Missing $required" >&2; exit 1; }
done
install -d -m 0750 "$destination"
install -m 0755 "$temporary/unpacked/$binary" "$destination/$binary.new"
mv -f -- "$destination/$binary.new" "$destination/$binary"
cp -- "$temporary/unpacked/LICENSE" "$temporary/unpacked/NOTICE" "$temporary/unpacked/THIRD_PARTY_NOTICES.md" "$destination/"
cp -R -- "$temporary/unpacked/LICENSES" "$destination/"
if [[ -d "$temporary/unpacked/docs" ]]; then cp -R -- "$temporary/unpacked/docs" "$destination/"; fi
echo "Installed $destination/$binary. Read docs/modernization/MIGRATION.md before starting."
echo "Service installation is explicit: cd '$destination' and run ./$binary install, then ./$binary start."
