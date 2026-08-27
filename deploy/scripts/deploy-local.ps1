# FluxSearch 本地一键部署（PowerShell）
# 用法: .\deploy\scripts\deploy-local.ps1
# 停止: .\deploy\scripts\stop-local.ps1

param(
    [switch]$Build,
    [switch]$NoFrontend,
    [switch]$NoFlagEmbedding,
    [switch]$NoMonitor,
    [switch]$NoApi,
    [switch]$Worker
)

$ErrorActionPreference = "Stop"
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

function Init-Config {
    $localDir = Join-Path $Root "config/local"
    if (-not (Test-Path $localDir)) {
        New-Item -ItemType Directory -Force -Path $localDir | Out-Null
    }
    $pairs = @(
        @("config/infra.example.env", "config/local/infra.env"),
        @("config/deploy.example.env", "config/local/deploy.env"),
        @("config/app.settings.example.json", "config/local/app.settings.json")
    )
    foreach ($pair in $pairs) {
        $src = Join-Path $Root $pair[0]
        $dst = Join-Path $Root $pair[1]
        if (-not (Test-Path $dst)) {
            Copy-Item $src $dst
            Write-Host "已创建 $($pair[1])"
        }
    }
}

function Env-Bool {
    param([string]$Key, [bool]$Default = $false)
    $v = [Environment]::GetEnvironmentVariable($Key, "Process")
    if (-not $v) { return $Default }
    return $v -match '^(1|true|yes|on)$'
}

function Is-Running {
    param([string]$Name)
    $pidFile = Join-Path $PidDir "$Name.pid"
    if (-not (Test-Path $pidFile)) { return $false }
    $procId = Get-Content $pidFile -ErrorAction SilentlyContinue
    if (-not $procId) { return $false }
    $p = Get-Process -Id $procId -ErrorAction SilentlyContinue
    return $null -ne $p
}

function Start-Service {
    param(
        [string]$Name,
        [string]$FilePath,
        [string]$ArgumentList,
        [string]$WorkingDirectory = $Root
    )
    if (Is-Running $Name) {
        $existingPid = Get-Content (Join-Path $PidDir "$Name.pid")
        Write-Host "[skip] $Name 已在运行 (PID $existingPid)"
        return
    }
    $logPath = Join-Path $LogDir "$Name.log"
    Write-Host "[start] $Name"
    $proc = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList `
        -WorkingDirectory $WorkingDirectory `
        -RedirectStandardOutput $logPath -RedirectStandardError $logPath `
        -PassThru -WindowStyle Hidden
    Set-Content -Path (Join-Path $PidDir "$Name.pid") -Value $proc.Id -NoNewline
    Start-Sleep -Seconds 2
    if ($proc.HasExited) {
        Write-Host "[fail] $Name 启动失败，见 $logPath"
        Get-Content $logPath -Tail 20 -ErrorAction SilentlyContinue
        return
    }
    Write-Host "[ok] $Name PID $($proc.Id)"
}

Init-Config
Load-EnvFile (Join-Path $Root "config/local/infra.env")
Load-EnvFile (Join-Path $Root "config/local/deploy.env")

if ($Build -or (Env-Bool "FLUXSEARCH_DEPLOY_BUILD")) {
    Write-Host "=== 构建 ==="
    make build
    if (-not $NoFrontend) { make build-frontend }
}

$LogDir = [Environment]::GetEnvironmentVariable("FLUXSEARCH_DEPLOY_LOG_DIR", "Process")
if (-not $LogDir) { $LogDir = ".deploy/logs" }
$PidDir = [Environment]::GetEnvironmentVariable("FLUXSEARCH_DEPLOY_PID_DIR", "Process")
if (-not $PidDir) { $PidDir = ".deploy/pids" }
$LogDir = Join-Path $Root $LogDir
$PidDir = Join-Path $Root $PidDir
New-Item -ItemType Directory -Force -Path $LogDir, $PidDir | Out-Null

$startFlag = -not $NoFlagEmbedding -and (Env-Bool "FLUXSEARCH_FLAGEMBEDDING_ENABLED" $true) -and (Env-Bool "FLUXSEARCH_DEPLOY_START_FLAGEMBEDDING" $true)
$startMonitor = -not $NoMonitor -and (Env-Bool "FLUXSEARCH_DEPLOY_START_MONITOR" $true)
$startApi = -not $NoApi -and (Env-Bool "FLUXSEARCH_DEPLOY_START_API" $true)
$startFrontend = -not $NoFrontend -and (Env-Bool "FLUXSEARCH_DEPLOY_START_FRONTEND" $true)
$startWorker = $Worker -or (Env-Bool "FLUXSEARCH_DEPLOY_START_WORKER")

if ($startFlag) {
    $py = (Get-Command python -ErrorAction SilentlyContinue).Source
    if (-not $py) { $py = (Get-Command py -ErrorAction SilentlyContinue).Source }
    if (-not $py) { throw "未找到 python，请安装 Python 3.10+" }
    Start-Service -Name "flagembedding" -FilePath $py -ArgumentList "scripts/flagembedding_server.py"
}

if ($startMonitor) {
    $go = (Get-Command go -ErrorAction SilentlyContinue).Source
    if (-not $go) { throw "未找到 go" }
    Start-Service -Name "monitor" -FilePath $go -ArgumentList "run ./cmd/monitor"
}

if ($startApi) {
    $go = (Get-Command go -ErrorAction SilentlyContinue).Source
    Start-Service -Name "api" -FilePath $go -ArgumentList "run ./cmd/api"
}

if ($startWorker) {
    $go = (Get-Command go -ErrorAction SilentlyContinue).Source
    Start-Service -Name "worker" -FilePath $go -ArgumentList "run ./cmd/worker"
}

if ($startFrontend) {
    $npm = (Get-Command npm -ErrorAction SilentlyContinue).Source
    if (-not $npm) { throw "未找到 npm" }
    Start-Service -Name "frontend" -FilePath $npm -ArgumentList "run dev" -WorkingDirectory (Join-Path $Root "frontend")
}

$apiPort = [Environment]::GetEnvironmentVariable("FLUXSEARCH_API_PORT", "Process")
if (-not $apiPort) { $apiPort = "8080" }
$fePort = [Environment]::GetEnvironmentVariable("FLUXSEARCH_FRONTEND_PORT", "Process")
if (-not $fePort) { $fePort = "5173" }
$monPort = [Environment]::GetEnvironmentVariable("FLUXSEARCH_MONITOR_PORT", "Process")
if (-not $monPort) { $monPort = "8090" }
$flagHost = [Environment]::GetEnvironmentVariable("FLUXSEARCH_FLAGEMBEDDING_HOST", "Process")
if (-not $flagHost) { $flagHost = "127.0.0.1" }
$flagPort = [Environment]::GetEnvironmentVariable("FLUXSEARCH_FLAGEMBEDDING_PORT", "Process")
if (-not $flagPort) { $flagPort = "8091" }

Write-Host ""
Write-Host "=== FluxSearch 已启动 ==="
Write-Host "  前端:     http://127.0.0.1:$fePort"
Write-Host "  API:      http://127.0.0.1:$apiPort"
Write-Host "  Monitor:  http://127.0.0.1:$monPort/api/v1/status"
Write-Host "  FlagEmb:  http://${flagHost}:$flagPort/v1"
Write-Host "  日志目录: $LogDir"
Write-Host "  停止服务: .\deploy\scripts\stop-local.ps1"
Write-Host ""
