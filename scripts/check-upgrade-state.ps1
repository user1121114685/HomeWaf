# HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

# 诊断 Windows 自升级后仍被进程占用的旧程序。仅 -TryDelete 会尝试删除旧副本。

param(
    [Parameter(Mandatory)]
    [string] $Dir,
    [switch] $TryDelete
)

$ErrorActionPreference = 'Continue'
$resolved = Resolve-Path -LiteralPath $Dir -ErrorAction SilentlyContinue
if (-not $resolved -or -not (Test-Path -LiteralPath $resolved.Path -PathType Container)) {
    Write-Host "找不到 HomeWaf 目录 ${Dir}。" -ForegroundColor Red
    exit 1
}
$Dir = $resolved.Path

Write-Host ''
Write-Host "=== 观测目录: $Dir ===" -ForegroundColor Cyan

# --- 1) 进程与映像路径 -------------------------------------------------
Write-Host ''
Write-Host '[1] HomeWaf64 进程（关注 Path 是不是 .old）' -ForegroundColor Yellow
$procs = Get-Process -Name HomeWaf64 -ErrorAction SilentlyContinue |
         Where-Object { $_.Path -and [String]::Equals((Split-Path -LiteralPath $_.Path), $Dir, 'OrdinalIgnoreCase') }
if (-not $procs) {
    Write-Host '    (无) 该目录下没有运行中的 HomeWaf64'
} else {
    foreach ($p in $procs) {
        $name = Split-Path $p.Path -Leaf
        $pinned = $name -like '*.old*'
        $tag = if ($pinned) { '  <== 钉住了 .old，第二次升级会失败' } else { '' }
        $line = '    pid={0,-6} started={1}  {2}{3}' -f $p.Id, $p.StartTime.ToString('HH:mm:ss'), $p.Path, $tag
        if ($pinned) { Write-Host $line -ForegroundColor Red } else { Write-Host $line }
    }
}

# --- 2) 残留文件 -------------------------------------------------------
Write-Host ''
Write-Host '[2] .HomeWaf64.exe.* 残留（-Force 才看得到 HIDDEN）' -ForegroundColor Yellow
$leftovers = Get-ChildItem -File -Force -LiteralPath $Dir -Filter '.HomeWaf64.exe.old*' -ErrorAction Stop |
             Where-Object { $_.Name -match '^\.HomeWaf64\.exe\.old(?:\..+)?$' }
if (-not $leftovers) {
    Write-Host '    (无)'
} else {
    foreach ($f in $leftovers) {
        $line = '    {0,-40} {1,12:N0} bytes  attrs={2}  {3}' -f `
                $f.Name, $f.Length, $f.Attributes, $f.LastWriteTime.ToString('MM-dd HH:mm:ss')
        Write-Host $line -ForegroundColor Red
    }
}

# 可选验证：Windows 会拒绝删除仍由进程映射的旧程序。
if ($TryDelete -and $leftovers) {
    Write-Host ''
    Write-Host '[3] 尝试删除旧程序' -ForegroundColor Yellow
    foreach ($f in $leftovers) {
        try {
            Remove-Item -Force -LiteralPath $f.FullName -ErrorAction Stop
            Write-Host ('    {0} -> 删除成功（说明没被进程钉住）' -f $f.Name) -ForegroundColor Green
        } catch {
            Write-Host ('    {0} -> {1}' -f $f.Name, $_.Exception.Message) -ForegroundColor Red
        }
    }
}
