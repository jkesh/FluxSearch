# 从服务器拉取 kubeconfig 到本机（首次配置用）
# 用法：.\scripts\fetch-kubeconfig.ps1
# 需要：ssh 可登录 deploy@your-k8s-node

param(
    [string]$Server = "your-k8s-node.example.com",
    [string]$User = "deploy",
    [string]$OutFile = "$env:USERPROFILE\.kube\fluxsearch-config"
)

$ErrorActionPreference = "Stop"
$kubeDir = Split-Path $OutFile -Parent
if (-not (Test-Path $kubeDir)) {
    New-Item -ItemType Directory -Path $kubeDir -Force | Out-Null
}

Write-Host "从 ${User}@${Server} 拉取 kubeconfig ..." -ForegroundColor Cyan

# 读取远程 k3s 配置并替换 server 地址
$remoteCmd = "sudo cat /etc/rancher/k3s/k3s.yaml | sed 's/127.0.0.1/$Server/g'"
ssh "${User}@${Server}" $remoteCmd | Set-Content -Path $OutFile -Encoding UTF8

Write-Host "已保存到: $OutFile" -ForegroundColor Green
Write-Host ""
Write-Host "临时使用：" -ForegroundColor Yellow
Write-Host "  `$env:KUBECONFIG = `"$OutFile`""
Write-Host ""
Write-Host "或永久设置：" -ForegroundColor Yellow
Write-Host "  [Environment]::SetEnvironmentVariable('KUBECONFIG', '$OutFile', 'User')"
Write-Host ""
Write-Host "然后运行：" -ForegroundColor Yellow
Write-Host "  .\scripts\port-forward.ps1"
