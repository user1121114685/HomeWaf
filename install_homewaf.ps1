# HomeWaf online installer. Supports Windows PowerShell 5.1 and PowerShell 7.
[CmdletBinding()]
param(
    [ValidateSet('install', 'uninstall')][string]$Action = 'install',
    [string]$Version = 'latest',
    [string]$Directory = (Join-Path $env:ProgramData 'HomeWaf'),
    [switch]$NoService,
    [string]$ArchivePath,
    [string]$ChecksumsPath
)
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$repo = 'https://github.com/user1121114685/HomeWaf'
$Directory = [IO.Path]::GetFullPath($Directory)
if ($Directory -eq [IO.Path]::GetPathRoot($Directory)) { throw 'Use an application directory, not a drive root.' }
$architecture = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
if ($architecture -ne 'AMD64') { throw "Unsupported Windows architecture: $architecture. This release supports x64 Windows." }
if (-not $NoService) {
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Open PowerShell as Administrator, then run the installation command again.' }
}
$binary = Join-Path $Directory 'HomeWaf64.exe'
function Invoke-HomeWaf([string]$Command) {
    & $binary $Command
    if ($LASTEXITCODE -ne 0) { throw "HomeWaf $Command failed (exit $LASTEXITCODE)." }
}
if ($Action -eq 'uninstall') {
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
    $archive = Join-Path $temporary $asset
    $checksumFile = Join-Path $temporary 'checksums.txt'
    Write-Host "Installing HomeWaf $Version (Windows/x64) to $Directory"
    if ($ArchivePath) { Copy-Item -LiteralPath $ArchivePath -Destination $archive } else { Invoke-WebRequest -UseBasicParsing -Uri "$repo/releases/download/$Version/$asset" -OutFile $archive }
    if ($ChecksumsPath) { Copy-Item -LiteralPath $ChecksumsPath -Destination $checksumFile } else { Invoke-WebRequest -UseBasicParsing -Uri "$repo/releases/download/$Version/HomeWaf_${number}_checksums.txt" -OutFile $checksumFile }
    $pattern = '^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($asset) + '$'
    $entries = @(Get-Content -LiteralPath $checksumFile | Where-Object { $_ -match $pattern })
    if ($entries.Count -ne 1) { throw "No unique SHA256 checksum for $asset" }
    $null = $entries[0] -match $pattern
    $expected = $Matches[1]
    if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $expected) { throw 'Archive SHA256 verification failed.' }
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
