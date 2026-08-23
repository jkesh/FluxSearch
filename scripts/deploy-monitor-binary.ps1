# 构建 Linux monitor 二进制并部署到服务器
# 用法: .\scripts\deploy-monitor-binary.ps1

param(
    [string]$Server = "your-k8s-node.example.com",
    [string]$User = "deploy"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

$RemoteDir = "/home/$User/fluxsearch"

Write-Host "构建 Linux amd64 monitor ..." -ForegroundColor Cyan
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
if (Test-Path fluxsearch-monitor) { Remove-Item fluxsearch-monitor -Force }
go build -o fluxsearch-monitor ./cmd/monitor

Write-Host "上传到 $RemoteDir ..." -ForegroundColor Cyan
ssh "${User}@${Server}" "mkdir -p $RemoteDir"
scp fluxsearch-monitor "${User}@${Server}:${RemoteDir}/fluxsearch-monitor"
scp deploy/scripts/server-run-monitor.sh "${User}@${Server}:${RemoteDir}/server-run-monitor.sh"

Write-Host "启动 monitor ..." -ForegroundColor Cyan
ssh "${User}@${Server}" "chmod +x ${RemoteDir}/fluxsearch-monitor ${RemoteDir}/server-run-monitor.sh && INSTALL_DIR=${RemoteDir} bash ${RemoteDir}/server-run-monitor.sh"

Write-Host ""
Write-Host "请在 config/local/infra.env 确认:" -ForegroundColor Green
Write-Host "FLUXSEARCH_MONITOR_URL=http://${Server}:8090/api/v1/status"
Write-Host ""
Write-Host "然后重启本地 API: go run ./cmd/api" -ForegroundColor Yellow
