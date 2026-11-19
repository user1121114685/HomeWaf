#!/usr/bin/env bash
# HomeWaf online installer for Linux and macOS.
set -euo pipefail
repo=https://github.com/user1121114685/HomeWaf
action=install
version=latest
destination=/opt/homewaf
service=1
docker_mode=0
archive=
checksums=
purge_data=0
yes=0
die() { printf 'HomeWaf: %s\n' "$*" >&2; exit 1; }
if [[ ${1:-} == install || ${1:-} == uninstall || ${1:-} == upgrade || ${1:-} == update ]]; then action=$1; shift; fi
[[ $action != update ]] || action=upgrade
while (($#)); do
  case "$1" in
    --version) version=${2:?Missing version}; shift 2 ;;
    --dir) destination=${2:?Missing directory}; shift 2 ;;
    --no-service) service=0; shift ;;
    --docker) docker_mode=1; service=0; shift ;;
    --archive) archive=${2:?Missing archive}; shift 2 ;;
    --checksums) checksums=${2:?Missing checksums file}; shift 2 ;;
    --purge-data) purge_data=1; shift ;;
    --yes) yes=1; shift ;;
    --help|-h) echo 'Usage: bash install_homewaf.sh [install|upgrade|uninstall] [--version latest|vX.Y.Z] [--dir /opt/homewaf] [--no-service|--docker] [--purge-data --yes]'; echo 'Native upgrade uses the authenticated local API and automatic smooth restart. Uninstall preserves data unless --purge-data --yes is explicit. Single-container Docker upgrades stop gracefully and recreate the container.'; exit 0 ;;
    *) die "Unknown option: $1" ;;
  esac
done
[[ $destination == /* && $destination != / ]] || die 'Use an absolute application directory, not /.'
((purge_data == 0)) || [[ $action == uninstall && $yes == 1 ]] || die '--purge-data requires uninstall --yes.'
if ((docker_mode)); then
  command -v docker >/dev/null || die 'Install Docker first.'
  [[ $(docker info --format '{{.OSType}}') == linux ]] || die 'Select Linux containers in Docker.'
  os=Linux
  machine=$(docker info --format '{{.Architecture}}')
else
case $(uname -s) in
  Linux) os=Linux ;;
  Darwin) os=Darwin ;;
  *) die 'Supported systems: Linux and macOS. On Windows use install_homewaf.ps1.' ;;
esac
machine=$(uname -m)
fi
case $machine in
  x86_64|amd64) arch=x86_64; suffix=64 ;;
  aarch64|arm64) arch=arm64; suffix=Arm64 ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac
binary=HomeWaf${os}${suffix}
if ((service)) && ((EUID != 0)); then die 'Service installation requires root; run this script with sudo.'; fi
if [[ $action == uninstall ]]; then
  if ((docker_mode)); then
    [[ $(docker inspect --format '{{index .Config.Labels "io.homewaf.installer"}}' homewaf) == release ]] || die 'Refusing to remove a container not owned by this installer.'
    docker stop --time 120 homewaf
    docker rm homewaf
    if ((purge_data)); then
      for volume in homewaf-conf homewaf-data homewaf-keys homewaf-logs homewaf-ssl; do
        if docker volume inspect "$volume" >/dev/null 2>&1; then docker volume rm "$volume"; fi
      done
    fi
    echo 'HomeWaf container removed. Named data volumes are retained unless --purge-data --yes was specified.'
    exit 0
  fi
  [[ -x $destination/$binary ]] || die "HomeWaf is not installed in $destination"
  destination=$(cd -- "$destination" && pwd -P)
  [[ $destination != / ]] || die 'Refusing to uninstall from the filesystem root.'
  if ((purge_data)); then
    [[ -f $destination/.homewaf-installer && ! -L $destination/.homewaf-installer && $(cat "$destination/.homewaf-installer") == HomeWaf ]] || die 'Data removal requires an installer-owned application directory.'
    [[ $destination != "${HOME:-/}" && $destination != /opt && $destination != /usr && $destination != /usr/local && $destination != /usr/local/bin && $destination != /etc && $destination != /var ]] || die 'Refusing data removal from a shared system directory.'
    for name in conf data .homewaf-keys logs ssl; do [[ ! -L $destination/$name ]] || die "Refusing to purge linked data directory: $name"; done
  fi
  if ((service)); then
    unit=/etc/systemd/system/HomeWafService.service
    [[ $os != Darwin ]] || unit=/Library/LaunchDaemons/HomeWafService.plist
    if [[ -f $unit ]]; then
      grep -Fq -- "$destination/$binary" "$unit" || die 'Existing service uses another directory; use its original --dir.'
      if [[ $os == Linux ]] && command -v systemctl >/dev/null; then
        if systemctl is-active --quiet HomeWafService; then "$destination/$binary" stop; fi
      elif [[ $os == Darwin ]] && command -v launchctl >/dev/null; then
        if launchctl print system/HomeWafService >/dev/null 2>&1; then "$destination/$binary" stop; fi
      else
        "$destination/$binary" stop
      fi
      "$destination/$binary" uninstall
    elif "$destination/$binary" console-info --wait 1s >/dev/null 2>&1; then
      die 'The instance is running without a recognized service definition; stop it before uninstalling.'
    fi
  elif "$destination/$binary" console-info --wait 1s >/dev/null 2>&1; then
    die 'Stop the foreground instance before uninstalling with --no-service.'
  fi
  rm -f -- "$destination/$binary"
  if ((purge_data)); then for name in conf data .homewaf-keys logs ssl; do rm -rf -- "$destination/$name"; done; fi
  echo "HomeWaf program removed. Configuration, data and keys remain in $destination unless --purge-data --yes was specified."
  exit 0
fi
if [[ $action == upgrade && $docker_mode == 1 ]]; then docker container inspect homewaf >/dev/null 2>&1 || die 'No existing HomeWaf container; use install first.'; fi
for command in curl tar install; do command -v "$command" >/dev/null || die "Required command not found: $command"; done
download() { curl --fail --silent --show-error --location --retry 3 --connect-timeout 20 --max-time 600 --proto '=https' --proto-redir '=https' "$1" -o "$2"; }
temporary=$(mktemp -d)
trap 'rm -rf -- "$temporary"' EXIT
if [[ $version == latest ]]; then
  effective=$(curl --fail --silent --show-error --location --retry 3 --connect-timeout 20 --max-time 60 --proto '=https' --proto-redir '=https' -o /dev/null -w '%{url_effective}' "$repo/releases/latest")
  version=${effective##*/}
fi
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9.-]+)?$ ]] || die "Invalid release version: $version"
[[ ! $version =~ ^v[012]\. ]] || die 'Use the installer supplied with the requested release for versions before v3.'
asset=HomeWaf_${os}_${arch}_${version#v}.tar.gz
checksum_name=HomeWaf_${version#v}_checksums.txt
if ((docker_mode)); then
  asset=HomeWaf_Docker_Linux_${arch}_${version#v}.tar.gz
  checksum_name=HomeWaf_${version#v}_docker_checksums.txt
fi
base=$repo/releases/download/$version
if ((docker_mode)); then echo "Installing HomeWaf $version Docker image (linux/$arch)"; else echo "Installing HomeWaf $version ($os/$arch) to $destination"; fi
if [[ -n $archive ]]; then cp -- "$archive" "$temporary/$asset"; else download "$base/$asset" "$temporary/$asset"; fi
if [[ -n $checksums ]]; then cp -- "$checksums" "$temporary/checksums.txt"; else download "$base/$checksum_name" "$temporary/checksums.txt"; fi
expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1 }' "$temporary/checksums.txt")
[[ $expected =~ ^[0-9a-fA-F]{64}$ ]] || die "No unique SHA256 checksum for $asset"
if command -v sha256sum >/dev/null; then actual=$(sha256sum "$temporary/$asset"); else actual=$(shasum -a 256 "$temporary/$asset"); fi
[[ ${actual%% *} == "$expected" ]] || die 'Archive SHA256 verification failed.'
if ((docker_mode)); then
  old_image=
  if docker container inspect homewaf >/dev/null 2>&1; then
    [[ $(docker inspect --format '{{index .Config.Labels "io.homewaf.installer"}}' homewaf) == release ]] || die 'Existing homewaf container was not created by this installer; update it using its own configuration.'
    old_image=$(docker inspect --format '{{.Image}}' homewaf)
  fi
  docker load --input "$temporary/$asset"
  docker run --rm --network none -v homewaf-data:/app/data:ro -v homewaf-keys:/app/.homewaf-keys:ro "homewaf:$version" check-install-format /app
  docker image tag "homewaf:$version" homewaf:latest
  if [[ -n $old_image ]]; then docker stop --time 120 homewaf; docker rm homewaf; fi
  run_args=(-d --name homewaf --label io.homewaf.installer=release --restart unless-stopped
    -p 127.0.0.1:26666:26666 -p 80:80 -p 443:443
    -v homewaf-conf:/app/conf -v homewaf-data:/app/data -v homewaf-keys:/app/.homewaf-keys
    -v homewaf-logs:/app/logs -v homewaf-ssl:/app/ssl)
  ready=0
  if docker run "${run_args[@]}" "homewaf:$version"; then
    for ((attempt=0;attempt<90;attempt++)); do
      if curl -fsS --max-time 2 http://127.0.0.1:26666/ -o /dev/null 2>/dev/null; then ready=1; break; fi
      sleep 1
    done
  fi
  if ((ready == 0)); then
    docker rm -f homewaf >/dev/null 2>&1 || true
    if [[ -n $old_image ]]; then docker run "${run_args[@]}" "$old_image"; fi
    die 'Container startup failed; check port availability and Docker logs. The previous image was restarted if present.'
  fi
  echo 'HomeWaf Docker console (on the Docker host): http://127.0.0.1:26666'
  echo 'Initial credentials (valid only until the password is changed):'
  docker exec homewaf sh -c 'if [ -f /app/data/initial_password.txt ]; then cat /app/data/initial_password.txt; else echo "Existing instance: use your current password."; fi'
  echo 'Use upgrade --docker to update. Named data volumes are preserved; single-container recreation has a brief interruption.'
  exit 0
fi
tar -tzf "$temporary/$asset" > "$temporary/entries"
if grep -Eq '(^/|(^|/)\.\.(/|$)|\\)' "$temporary/entries"; then die 'Unsafe archive paths.'; fi
tar -tvzf "$temporary/$asset" > "$temporary/types"
if grep -Eq '^[lhcbps]' "$temporary/types"; then die 'Archive links and special files are not accepted.'; fi
mkdir "$temporary/unpacked"
tar --no-same-owner --no-same-permissions -xzf "$temporary/$asset" -C "$temporary/unpacked"
for required in "$binary" LICENSE NOTICE THIRD_PARTY_NOTICES.md LICENSES; do
  [[ -e $temporary/unpacked/$required ]] || die "Missing $required"
done
"$temporary/unpacked/$binary" check-install-format "$destination"
if [[ $action == upgrade ]]; then
  [[ -x $destination/$binary ]] || die "HomeWaf is not installed in $destination"
  (cd -- "$destination"; "./$binary" apikey init; "./$binary" update --version "$version" --wait 15m)
  exit 0
fi
installed=0
if ((service)); then
  if [[ -f /etc/systemd/system/HomeWafService.service || -f /Library/LaunchDaemons/HomeWafService.plist ]]; then
    [[ -x $destination/$binary ]] || die 'Existing service uses another directory; use its original --dir.'
    "$destination/$binary" stop
    installed=1
  fi
fi
install -d -m 0750 "$destination"
[[ ! -L $destination/.homewaf-installer ]] || die 'Invalid installer ownership marker.'
printf 'HomeWaf\n' > "$destination/.homewaf-installer"
install -m 0755 "$temporary/unpacked/$binary" "$destination/$binary.new"
mv -f -- "$destination/$binary.new" "$destination/$binary"
for required in LICENSE NOTICE THIRD_PARTY_NOTICES.md LICENSES docs; do
  [[ ! -e $temporary/unpacked/$required ]] || cp -R "$temporary/unpacked/$required" "$destination/"
done
if ((service)); then
  if ((installed == 0)); then "$destination/$binary" install; fi
  "$destination/$binary" start
  "$destination/$binary" console-info --wait 90s --show-initial-password
else
  echo "Installed. Start with: $destination/$binary"
  echo "After startup, show the server address and initial password: $destination/$binary console-info --show-initial-password"
fi
echo "Account: admin. Initial credentials are also saved in $destination/data/initial_password.txt."
echo "Keep $destination/.homewaf-keys together with your backups."
