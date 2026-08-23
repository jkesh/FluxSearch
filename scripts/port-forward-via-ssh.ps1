# 在远程服务器上启动 port-forward（绑定 0.0.0.0）
# 用法：.\scripts\port-forward-via-ssh.ps1 -Server your-node -User deploy

param(
    [string]$Server = "your-k8s-node.example.com",
    [string]$User = "deploy"
)

$ScriptPath = Join-Path $PSScriptRoot "..\deploy\scripts\server-port-forward.sh"
if (-not (Test-Path $ScriptPath)) {
    Write-Error "找不到 $ScriptPath"
    exit 1
}

Write-Host "上传并执行 server-port-forward.sh ..." -ForegroundColor Cyan
Write-Host "需要输入 SSH 密码与 sudo 密码`n" -ForegroundColor Yellow

scp $ScriptPath "${User}@${Server}:/tmp/server-port-forward.sh"

ssh "${User}@${Server}" "chmod +x /tmp/server-port-forward.sh && sudo bash /tmp/server-port-forward.sh"

Write-Host "`nPort-forward 已在远程启动。本机可通过 $Server 访问各服务端口。" -ForegroundColor Green
