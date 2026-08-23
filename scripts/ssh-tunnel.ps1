# Windows 本地 SSH 隧道（推荐，最稳定）
# 前提：先在服务器上启动 127.0.0.1 转发（见下方说明）
# 用法：.\scripts\ssh-tunnel.ps1
# 保持此窗口运行，不要关闭

param(
    [string]$Server = "your-k8s-node.example.com",
    [string]$User = "deploy"
)

Write-Host @"

FluxSearch SSH 隧道
===================
此脚本将服务器 127.0.0.1 上的端口映射到本机 127.0.0.1
请确保已在服务器执行（另一个 SSH 窗口）:

  sudo bash server-port-forward.sh

或使用仅本机绑定的命令:
  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
  sudo kubectl port-forward -n fluxsearch svc/postgres 5432:5432 &
  sudo kubectl port-forward -n fluxsearch svc/redis 6379:6379 &
  ...

然后 config/local/infra.env 使用 127.0.0.1

按 Ctrl+C 停止隧道

"@ -ForegroundColor Cyan

ssh -N `
  -L 5432:127.0.0.1:5432 `
  -L 6379:127.0.0.1:6379 `
  -L 9000:127.0.0.1:9000 `
  -L 19530:127.0.0.1:19530 `
  -L 2379:127.0.0.1:2379 `
  "${User}@${Server}"
