$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$baselineRoot = Join-Path $repoRoot '.cache/p9-baseline-201'
$baselineCommit = '52078dfc164a3cb60637f903ba9b630dc64d5f45'
if (-not (Test-Path -LiteralPath $baselineRoot)) {
    git -C $repoRoot worktree add --detach $baselineRoot $baselineCommit
    if ($LASTEXITCODE -ne 0) { throw 'Could not create isolated baseline worktree' }
}
if ((git -C $baselineRoot rev-parse HEAD) -ne $baselineCommit) { throw 'Wrong baseline revision' }
$runRoot = Join-Path $repoRoot ('.cache/pipeline-migration-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runRoot | Out-Null
$overlay = @{ Replace = @{} }
$overlay.Replace[(Join-Path $baselineRoot 'wafdb/zz_rehearsal_test.go')] = Join-Path $PSScriptRoot 'pipeline-migration/fixture_v201_test.go.txt'
$overlayPath = Join-Path $runRoot 'overlay.json'
[IO.File]::WriteAllText($overlayPath, ($overlay | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
$env:HOMEWAF_V201_DB = Join-Path $runRoot 'v201.db'
$env:CGO_ENABLED = '0'; $env:GOPROXY = 'off'; $env:GOSUMDB = 'off'
Push-Location $baselineRoot
try {
    $ErrorActionPreference = 'Continue'
    go test -overlay $overlayPath ./wafdb -run '^TestExportV201Fixture$' -count=1 -v
    $ErrorActionPreference = 'Stop'
    if ($LASTEXITCODE -ne 0) { throw 'Baseline fixture generation failed' }
} finally { Pop-Location }
Push-Location $repoRoot
try {
    $ErrorActionPreference = 'Continue'
    go test ./wafdb -run '^TestV201SchemaMigrationRehearsal$' -count=1 -v
    $ErrorActionPreference = 'Stop'
    if ($LASTEXITCODE -ne 0) { throw 'Migration rehearsal failed' }
    $ErrorActionPreference = 'Continue'
    go test ./service/waf_service -run '^TestV201BusinessConfigurationMigrationRehearsal$' -count=1 -v -timeout=120s
    $ErrorActionPreference = 'Stop'
    if ($LASTEXITCODE -ne 0) { throw 'Business configuration migration rehearsal failed' }
    Get-FileHash -LiteralPath $env:HOMEWAF_V201_DB -Algorithm SHA256 | Format-List
} finally { Pop-Location }
