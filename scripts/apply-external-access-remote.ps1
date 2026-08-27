# 在远程服务器上应用 K8s 外网访问配置
# 用法: .\scripts\apply-external-access-remote.ps1
# 需要: ssh deploy@113.128.132.69 可登录

param(
    [string]$Server = "113.128.132.69",
    [string]$User = "deploy",
    [string]$PublicIP = "113.128.132.69"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Script = Join-Path $Root "deploy\scripts\apply-external-access.sh"

if (-not (Test-Path $Script)) {
    Write-Error "找不到 $Script"
}

Write-Host "上传 apply-external-access.sh 到 ${User}@${Server} ..." -ForegroundColor Cyan
scp $Script "${User}@${Server}:/tmp/apply-external-access.sh"

Write-Host "执行外网访问配置 PUBLIC_IP=$PublicIP ..." -ForegroundColor Cyan
ssh "${User}@${Server}" "chmod +x /tmp/apply-external-access.sh && FLUXSEARCH_PUBLIC_IP=$PublicIP sudo -E bash /tmp/apply-external-access.sh"

Write-Host ""
Write-Host "请更新 config/local/infra.env 中所有 192.168.0.200 为 $PublicIP" -ForegroundColor Yellow
Write-Host "FLUXSEARCH_MONITOR_URL=http://${PublicIP}:8090/api/v1/status" -ForegroundColor Green
