# 停止 deploy-local.ps1 启动的进程
$ErrorActionPreference = "SilentlyContinue"
$Root = Resolve-Path (Join-Path $PSScriptRoot "../../")
Set-Location $Root

function Load-EnvFile {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        $key = $line.Substring(0, $idx).Trim()
        $val = $line.Substring($idx + 1).Trim()
        if ($key -and -not [Environment]::GetEnvironmentVariable($key, "Process")) {
            [Environment]::SetEnvironmentVariable($key, $val, "Process")
        }
    }
}

Load-EnvFile (Join-Path $Root "config/local/deploy.env")
$PidDir = [Environment]::GetEnvironmentVariable("FLUXSEARCH_DEPLOY_PID_DIR", "Process")
if (-not $PidDir) { $PidDir = ".deploy/pids" }
$PidDir = Join-Path $Root $PidDir

$services = @("frontend", "api", "worker", "monitor", "flagembedding")
foreach ($name in $services) {
    $pidFile = Join-Path $PidDir "$name.pid"
    if (-not (Test-Path $pidFile)) { continue }
    $procId = Get-Content $pidFile -ErrorAction SilentlyContinue
    if ($procId) {
        $p = Get-Process -Id $procId -ErrorAction SilentlyContinue
        if ($p) {
            Write-Host "[stop] $name PID $procId"
            Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
        }
    }
    Remove-Item $pidFile -Force -ErrorAction SilentlyContinue
}

Write-Host "完成。"
