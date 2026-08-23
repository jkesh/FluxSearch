# 将 K8s fluxsearch 命名空间服务转发到本机
# 前提：kubectl 已配置并可访问远程 K3s 集群
# 用法：.\scripts\port-forward.ps1

$ErrorActionPreference = "Stop"
$Ns = "fluxsearch"

Write-Host "FluxSearch port-forward (namespace: $Ns)" -ForegroundColor Cyan
Write-Host "按 Ctrl+C 停止所有转发`n"

$forwards = @(
    @{ Name = "PostgreSQL"; Svc = "postgres"; Local = 5432; Remote = 5432 },
    @{ Name = "Redis";      Svc = "redis";    Local = 6379; Remote = 6379 },
    @{ Name = "MinIO";      Svc = "minio";    Local = 9000; Remote = 9000 },
    @{ Name = "Milvus";     Svc = "milvus";   Local = 19530; Remote = 19530 },
    @{ Name = "etcd";       Svc = "etcd";     Local = 2379; Remote = 2379 }
)

$jobs = @()
foreach ($f in $forwards) {
    Write-Host ("  {0,-12} localhost:{1} -> {2}:{3}" -f $f.Name, $f.Local, $f.Svc, $f.Remote)
    $jobs += Start-Job -ScriptBlock {
        param($ns, $svc, $local, $remote)
        kubectl port-forward -n $ns "svc/$svc" "${local}:${remote}"
    } -ArgumentList $Ns, $f.Svc, $f.Local, $f.Remote
}

Write-Host "`n转发已启动。另开终端运行: go run ./cmd/api" -ForegroundColor Green
Write-Host "设置页: http://localhost:5173/settings`n"

try {
    while ($true) { Start-Sleep -Seconds 60 }
} finally {
    $jobs | Stop-Job -PassThru | Remove-Job -Force
}
