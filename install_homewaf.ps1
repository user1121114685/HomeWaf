# HomeWaf online installer. Supports Windows PowerShell 5.1 and PowerShell 7.
[CmdletBinding()]
param(
    [ValidateSet('install', 'upgrade', 'update', 'uninstall')][string]$Action = 'install',
    [string]$Version = 'latest',
    [string]$Directory = (Join-Path $env:ProgramData 'HomeWaf'),
    [switch]$NoService,
    [switch]$Docker,
    [string]$ArchivePath,
    [string]$ChecksumsPath,
    [switch]$PurgeData,
    [switch]$Yes
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$repo = 'https://github.com/user1121114685/HomeWaf'
$Directory = [IO.Path]::GetFullPath($Directory)
if ($Action -eq 'update') { $Action = 'upgrade' }
if ($PurgeData -and ($Action -ne 'uninstall' -or -not $Yes)) { throw '-PurgeData requires -Action uninstall -Yes.' }
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
function Test-CommandExit([string]$Executable, [string[]]$CommandArgs) {
    # PowerShell 5.1 can turn redirected native stderr into a terminating error.
    # Read-only capability/readiness probes must inspect the exit code instead.
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & $Executable @CommandArgs *> $null
        return $LASTEXITCODE -eq 0
    } finally { $ErrorActionPreference = $previousPreference }
}
if ($Action -eq 'uninstall') {
    if ($Docker) {
        $metadata = (& docker container inspect homewaf | ConvertFrom-Json)[0]
        if ($LASTEXITCODE -ne 0 -or $metadata.Config.Labels.'io.homewaf.installer' -ne 'release') { throw 'Refusing to remove a container not owned by this installer.' }
        & docker stop --time 120 homewaf
        if ($LASTEXITCODE -ne 0) { throw 'Cannot stop the HomeWaf container gracefully.' }
        & docker rm homewaf
        if ($LASTEXITCODE -ne 0) { throw 'Cannot remove the stopped container.' }
        if ($PurgeData) {
            foreach ($volume in @('homewaf-conf','homewaf-data','homewaf-keys','homewaf-logs','homewaf-ssl')) {
                if (Test-CommandExit 'docker' @('volume','inspect',$volume)) { & docker volume rm $volume; if ($LASTEXITCODE -ne 0) { throw "Cannot remove data volume $volume." } }
            }
        }
        Write-Host 'HomeWaf container removed. Named data volumes are retained unless -PurgeData -Yes was specified.'
        return
    }
    if (-not (Test-Path -LiteralPath $binary)) { throw "HomeWaf is not installed in $Directory" }
    $dataTargets = @()
    if ($PurgeData) {
        $marker = Join-Path $Directory '.homewaf-installer'
        if (-not (Test-Path -LiteralPath $marker) -or ((Get-Item -LiteralPath $marker -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -or (Get-Content -LiteralPath $marker -Raw).Trim() -ne 'HomeWaf') { throw 'Data removal requires an installer-owned application directory.' }
        $reserved = @($env:USERPROFILE, $env:ProgramData, $env:ProgramFiles, ${env:ProgramFiles(x86)}, $env:SystemRoot, (Join-Path $env:SystemDrive 'Users')) | Where-Object { $_ }
        if ($reserved -contains $Directory.TrimEnd('\')) { throw 'Refusing data removal from a shared system directory.' }
        if ((Get-Item -LiteralPath $Directory -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing data removal through a directory link.' }
        foreach ($name in @('conf','data','.homewaf-keys','logs','ssl')) {
            $target = [IO.Path]::GetFullPath((Join-Path $Directory $name))
            if (-not $target.StartsWith($Directory.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid data removal target.' }
            if (Test-Path -LiteralPath $target) {
                if ((Get-Item -LiteralPath $target -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Refusing to purge linked data: $target" }
                $dataTargets += $target
            }
        }
    }
    $existing = Get-Service -Name HomeWafService -ErrorAction SilentlyContinue
    if ($existing -and $NoService) {
        $serviceInfo = Get-CimInstance Win32_Service -Filter "Name='HomeWafService'"
        if ($serviceInfo.PathName.Trim('"') -eq $binary) { throw 'This directory belongs to the installed service; uninstall without -NoService from an elevated shell.' }
        $existing = $null # Never control another installation when removing a foreground copy.
    }
    if ($existing) {
        $serviceInfo = Get-CimInstance Win32_Service -Filter "Name='HomeWafService'"
        if ($serviceInfo.PathName.Trim('"') -ne $binary) { throw 'Existing service uses another directory; use its original -Directory.' }
        if ($NoService) { throw 'This directory belongs to the installed service; uninstall without -NoService from an elevated shell.' }
        if ($existing.Status -ne 'Stopped') { Invoke-HomeWaf 'stop'; $existing.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(120)) }
        Invoke-HomeWaf 'uninstall'
    } else {
        if (Test-CommandExit $binary @('console-info','--wait','1s')) { throw 'Stop the foreground instance before uninstalling.' }
    }
    Remove-Item -LiteralPath $binary -Force
    foreach ($target in $dataTargets) { Remove-Item -LiteralPath $target -Recurse -Force }
    Write-Host "HomeWaf program removed. Configuration, data and keys remain in $Directory unless -PurgeData -Yes was specified."
    return
}
if ($Action -eq 'upgrade' -and -not $Docker) {
    if (-not (Test-Path -LiteralPath $binary)) { throw "HomeWaf is not installed in $Directory" }
    if ((Test-CommandExit $binary @('update','--help')) -and -not $ArchivePath -and -not $ChecksumsPath) {
        Push-Location -LiteralPath $Directory
        try {
            & $binary apikey init
            if ($LASTEXITCODE -ne 0) { throw 'Cannot initialize the local update credential.' }
            & $binary update --version $Version --wait 15m
            if ($LASTEXITCODE -ne 0) { throw 'Update did not complete; inspect update --status before retrying.' }
        } finally { Pop-Location }
        return
    }
}
if ($Action -eq 'upgrade' -and $Docker) {
    if (-not (Test-CommandExit 'docker' @('container','inspect','homewaf'))) { throw 'No existing HomeWaf container; use install first.' }
}
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('homewaf-install-' + [Guid]::NewGuid().ToString('N'))
$helper = $null
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
    # Use .NET directly: Windows PowerShell launched from PowerShell 7 can
    # inherit a module path that does not expose the Get-FileHash function.
    $stream = [IO.File]::OpenRead($archive)
    $hasher = [Security.Cryptography.SHA256]::Create()
    try { $actual = [BitConverter]::ToString($hasher.ComputeHash($stream)).Replace('-', '') }
    finally { $stream.Dispose(); $hasher.Dispose() }
    if ($actual -ne $expected) { throw 'Archive SHA256 verification failed.' }
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
        if ($oldImage) { & docker stop --time 120 homewaf; if ($LASTEXITCODE -ne 0) { throw 'Cannot stop the existing container.' }; & docker rm homewaf; if ($LASTEXITCODE -ne 0) { throw 'Cannot remove the stopped container.' } }
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
        Write-Host 'HomeWaf Docker console (on the Docker host): http://127.0.0.1:26666'
        Write-Host 'Initial credentials (valid only until the password is changed):'
        & docker exec homewaf sh -c 'if [ -f /app/data/initial_password.txt ]; then cat /app/data/initial_password.txt; else echo "Existing instance: use your current password."; fi'
        if ($LASTEXITCODE -ne 0) { throw 'Could not read initial credentials from the container.' }
        Write-Host 'Use -Action upgrade -Docker to update. Named data volumes are preserved; single-container recreation has a brief interruption.'
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
    if ($Action -eq 'upgrade') {
        # A verified new CLI can upgrade an older instance even when its WebUI
        # is broken. The running server owns download validation and restart.
        $helper = Join-Path $Directory ('.homewaf-upgrade-' + [Guid]::NewGuid().ToString('N') + '.exe')
        Copy-Item -LiteralPath (Join-Path $unpacked 'HomeWaf64.exe') -Destination $helper
        if (-not (Test-CommandExit $helper @('update','--help'))) { throw 'This release has no command-line update client; choose a newer release.' }
        Push-Location -LiteralPath $Directory
        try {
            & $binary apikey init
            if ($LASTEXITCODE -ne 0) { throw 'Cannot initialize the local update credential.' }
            & $helper update --version $Version --wait 15m
            if ($LASTEXITCODE -ne 0) { throw 'Update did not complete; inspect update --status before retrying.' }
        } finally { Pop-Location }
        return
    }
    $existing = Get-Service -Name HomeWafService -ErrorAction SilentlyContinue
    if ($existing -and -not $NoService) {
        $serviceInfo = Get-CimInstance Win32_Service -Filter "Name='HomeWafService'"
        if ($serviceInfo.PathName.Trim('"') -ne $binary) { throw 'Existing service uses another directory; use its original -Directory.' }
        if ($existing.Status -ne 'Stopped') { Invoke-HomeWaf 'stop'; $existing.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(60)) }
    }
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    $marker = Join-Path $Directory '.homewaf-installer'
    if ((Test-Path -LiteralPath $marker) -and ((Get-Item -LiteralPath $marker -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Invalid installer ownership marker.' }
    [IO.File]::WriteAllText($marker, 'HomeWaf')
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
        if (Test-CommandExit $binary @('console-info','--help')) {
            & $binary console-info --wait 90s --show-initial-password
            if ($LASTEXITCODE -ne 0) { throw 'HomeWaf console is not ready; check the service and logs.' }
        } else {
            Write-Host 'HomeWaf service started. Open a server IP reachable from your device:'
            $addresses = [Net.NetworkInformation.NetworkInterface]::GetAllNetworkInterfaces() |
                Where-Object { $_.OperationalStatus -eq 'Up' -and $_.NetworkInterfaceType -ne 'Loopback' } |
                ForEach-Object { $_.GetIPProperties().UnicastAddresses } |
                Where-Object { $_.Address.AddressFamily -eq 'InterNetwork' } |
                ForEach-Object { $_.Address.IPAddressToString } | Select-Object -Unique
            foreach ($address in $addresses) { Write-Host "  http://${address}:26666/" }
            if (-not $addresses) { Write-Host '  http://<server-ip>:26666/ (replace <server-ip> with this server address)' }
            $ready = $false
            for ($attempt = 0; $attempt -lt 90; $attempt++) {
                try { if ((Invoke-WebRequest -UseBasicParsing -Uri http://127.0.0.1:26666/ -TimeoutSec 2).StatusCode -eq 200) { $ready = $true; break } } catch {}
                Start-Sleep -Seconds 1
            }
            if (-not $ready) { throw 'Management console did not become ready. Check the HomeWaf service and logs.' }
            $initialPassword = Join-Path $Directory 'data/initial_password.txt'
            if (Test-Path -LiteralPath $initialPassword -PathType Leaf) {
                Write-Host 'Initial credentials (valid only until the password is changed):'
                Get-Content -LiteralPath $initialPassword -Encoding UTF8
            } else { Write-Host 'Existing instance: use your current password.' }
        }
    } else { Write-Host "Installed. Start with: & '$binary'" }
    Write-Host "Account: admin. Initial credentials are also saved in $Directory\data\initial_password.txt."
    Write-Host "Keep $Directory\.homewaf-keys together with your backups."
} finally {
    if ($helper -and (Test-Path -LiteralPath $helper)) { Remove-Item -LiteralPath $helper -Force }
    $temporaryFull = [IO.Path]::GetFullPath($temporary)
    $temporaryRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([char[]]@('\','/')) + [IO.Path]::DirectorySeparatorChar
    if (-not $temporaryFull.StartsWith($temporaryRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Invalid temporary cleanup target.' }
    Remove-Item -LiteralPath $temporaryFull -Recurse -Force
}
