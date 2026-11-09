# HomeWaf online installer. Supports Windows PowerShell 5.1 and PowerShell 7.
[CmdletBinding()]
param(
    [ValidateSet('install', 'uninstall')][string]$Action = 'install',
    [string]$Version = 'latest',
    [string]$Directory = (Join-Path $env:ProgramData 'HomeWaf'),
    [switch]$NoService,
    [switch]$Docker,
    [string]$ArchivePath,
    [string]$ChecksumsPath
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$repo = 'https://github.com/user1121114685/HomeWaf'
$Directory = [IO.Path]::GetFullPath($Directory)
if ($Directory -eq [IO.Path]::GetPathRoot($Directory)) { throw 'Use an application directory, not a drive root.' }
$architecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
if ($Docker) {
    $dockerPlatform = & docker info --format '{{.OSType}}/{{.Architecture}}'
    if ($LASTEXITCODE -ne 0 -or $dockerPlatform -notmatch '^linux/(x86_64|amd64|arm64|aarch64)$') { throw 'Start Docker and select Linux containers.' }
    $dockerArch = if ($Matches[1] -in @('arm64','aarch64')) { 'arm64' } else { 'x86_64' }
} elseif ($architecture -ne 'AMD64') { throw "Unsupported Windows architecture: $architecture. This release supports x64 Windows." }
if (-not $NoService -and -not $Docker) {
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Open PowerShell as Administrator, then run the installation command again.' }
}
$binary = Join-Path $Directory 'HomeWaf64.exe'
function Invoke-HomeWaf([string]$Command) {
    & $binary $Command
    if ($LASTEXITCODE -ne 0) { throw "HomeWaf $Command failed (exit $LASTEXITCODE)." }
}
if ($Action -eq 'uninstall') {
    if ($Docker) { throw 'Remove the container with: docker rm -f homewaf (named data volumes are retained).' }
    if (-not (Test-Path -LiteralPath $binary)) { throw "HomeWaf is not installed in $Directory" }
    $existing = Get-Service -Name HomeWafService -ErrorAction SilentlyContinue
    if ($existing) {
        if ($existing.Status -ne 'Stopped') { Invoke-HomeWaf 'stop' }
        Invoke-HomeWaf 'uninstall'
    }
    Write-Host "Service removed. Application, configuration, data and keys remain in $Directory."
    return
}
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('homewaf-install-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporary | Out-Null
try {
    if ($Version -eq 'latest') {
        $release = Invoke-RestMethod -Uri 'https://api.github.com/repos/user1121114685/HomeWaf/releases/latest' -Headers @{Accept='application/vnd.github+json'}
        if ($release.draft -or $release.prerelease) { throw 'No stable release is available.' }
        $Version = $release.tag_name
    }
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9.-]+)?$') { throw "Invalid release version: $Version" }
    $number = $Version.Substring(1)
    $asset = "HomeWaf_Windows_x86_64_$number.zip"
    $checksumName = "HomeWaf_${number}_checksums.txt"
    if ($Docker) { $asset = "HomeWaf_Docker_Linux_${dockerArch}_$number.tar.gz"; $checksumName = "HomeWaf_${number}_docker_checksums.txt" }
    $archive = Join-Path $temporary $asset
    $checksumFile = Join-Path $temporary 'checksums.txt'
    if ($Docker) { Write-Host "Installing HomeWaf $Version Docker image (linux/$dockerArch)" } else { Write-Host "Installing HomeWaf $Version (Windows/x64) to $Directory" }
    if ($ArchivePath) { Copy-Item -LiteralPath $ArchivePath -Destination $archive } else { Invoke-WebRequest -UseBasicParsing -Uri "$repo/releases/download/$Version/$asset" -OutFile $archive }
    if ($ChecksumsPath) { Copy-Item -LiteralPath $ChecksumsPath -Destination $checksumFile } else { Invoke-WebRequest -UseBasicParsing -Uri "$repo/releases/download/$Version/$checksumName" -OutFile $checksumFile }
    $pattern = '^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($asset) + '$'
    $entries = @(Get-Content -LiteralPath $checksumFile | Where-Object { $_ -match $pattern })
    if ($entries.Count -ne 1) { throw "No unique SHA256 checksum for $asset" }
    $null = $entries[0] -match $pattern
    $expected = $Matches[1]
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $expected) { throw 'Archive SHA256 verification failed.' }
    if ($Docker) {
        $oldImage = $null
        $existingContainer = & docker container ls -aq --filter 'name=^/homewaf$'
        if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Docker containers.' }
        if ($existingContainer) {
            $metadata = (& docker container inspect homewaf | ConvertFrom-Json)[0]
            if ($metadata.Config.Labels.'io.homewaf.installer' -ne 'release') { throw 'Existing homewaf container was not created by this installer; use its original configuration.' }
            $oldImage = $metadata.Image
        }
        & docker load --input $archive
        if ($LASTEXITCODE -ne 0) { throw 'Cannot load the Docker release image.' }
        & docker image tag "homewaf:$Version" homewaf:latest
        if ($LASTEXITCODE -ne 0) { throw 'Cannot tag the Docker release image.' }
        if ($oldImage) { & docker stop homewaf; if ($LASTEXITCODE -ne 0) { throw 'Cannot stop the existing container.' }; & docker rm homewaf; if ($LASTEXITCODE -ne 0) { throw 'Cannot remove the stopped container.' } }
        $runArgs = @('-d','--name','homewaf','--label','io.homewaf.installer=release','--restart','unless-stopped',
            '-p','127.0.0.1:26666:26666','-p','80:80','-p','443:443','-p','443:443/udp',
            '-v','homewaf-conf:/app/conf','-v','homewaf-data:/app/data','-v','homewaf-keys:/app/.homewaf-keys',
            '-v','homewaf-logs:/app/logs','-v','homewaf-ssl:/app/ssl')
        & docker run @runArgs "homewaf:$Version"
        $ready = $false
        if ($LASTEXITCODE -eq 0) {
            for ($attempt=0; $attempt -lt 90; $attempt++) {
                try { if ((Invoke-WebRequest -UseBasicParsing -Uri http://127.0.0.1:26666/ -TimeoutSec 2).StatusCode -eq 200) { $ready=$true; break } } catch {}
                Start-Sleep -Seconds 1
            }
        }
        if (-not $ready) {
            & docker rm -f homewaf 2>$null
            if ($oldImage) { & docker run @runArgs $oldImage }
            throw 'Container startup failed. The previous image was restarted if present.'
        }
        Write-Host 'HomeWaf is running at http://127.0.0.1:26666'
        Write-Host 'Initial password: docker exec homewaf cat /app/data/initial_password.txt'
        Write-Host 'Run this command again to update; named data volumes are preserved.'
        return
    }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($archive)
    try {
        foreach ($entry in $zip.Entries) {
            if ($entry.FullName -match '(^[/\\]|(^|[/\\])\.\.([/\\]|$)|:)' -or (($entry.ExternalAttributes -shr 16) -band 0xf000) -eq 0xa000) { throw 'Unsafe ZIP entry.' }
        }
    } finally { $zip.Dispose() }
    $unpacked = Join-Path $temporary 'unpacked'
    [IO.Compression.ZipFile]::ExtractToDirectory($archive, $unpacked)
    foreach ($required in @('HomeWaf64.exe','LICENSE','NOTICE','THIRD_PARTY_NOTICES.md','LICENSES')) {
        if (-not (Test-Path -LiteralPath (Join-Path $unpacked $required))) { throw "Missing $required" }
    }
    $existing = Get-Service -Name HomeWafService -ErrorAction SilentlyContinue
    if ($existing -and -not $NoService) {
        $serviceInfo = Get-CimInstance Win32_Service -Filter "Name='HomeWafService'"
        if ($serviceInfo.PathName.Trim('"') -ne $binary) { throw 'Existing service uses another directory; use its original -Directory.' }
        if ($existing.Status -ne 'Stopped') { Invoke-HomeWaf 'stop'; $existing.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(60)) }
    }
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    Copy-Item -LiteralPath (Join-Path $unpacked 'HomeWaf64.exe') -Destination "$binary.new"
    Move-Item -LiteralPath "$binary.new" -Destination $binary -Force
    foreach ($name in @('LICENSE','NOTICE','THIRD_PARTY_NOTICES.md','LICENSES','docs')) {
        $source = Join-Path $unpacked $name
        if (Test-Path -LiteralPath $source) { Copy-Item -LiteralPath $source -Destination $Directory -Recurse -Force }
    }
    if (-not $NoService) {
        if (-not $existing) { Invoke-HomeWaf 'install' }
        Invoke-HomeWaf 'start'
        (Get-Service HomeWafService).WaitForStatus('Running', [TimeSpan]::FromSeconds(60))
        Write-Host 'HomeWaf service started. Open http://127.0.0.1:26666'
    } else { Write-Host "Installed. Start with: & '$binary'" }
    Write-Host "Account: admin. Initial password: $Directory\data\initial_password.txt (created on first start)."
    Write-Host "Keep $Directory\.homewaf-keys together with your backups."
} finally { Remove-Item -LiteralPath $temporary -Recurse -Force }
