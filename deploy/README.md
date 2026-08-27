# FluxSearch 部署脚本

## 本地一键启动（开发机）

在已配置 `config/local/infra.env`（基础设施连接）的前提下：

| 平台 | 启动 | 停止 |
|------|------|------|
| Windows | `.\deploy\scripts\deploy-local.ps1` | `.\deploy\scripts\stop-local.ps1` |
| Bash | `bash deploy/scripts/deploy-local.sh` | `bash deploy/scripts/stop-local.sh` |
| Make | `make deploy-local` | `make stop-local` |

首次运行自动创建：

- `config/local/infra.env` ← `config/infra.example.env`
- `config/local/deploy.env` ← `config/deploy.example.env`
- `config/local/app.settings.json` ← `config/app.settings.example.json`

### 配置分层

```text
infra.env      → Postgres / Redis / Milvus / MinIO / 密钥
deploy.env     → 端口、FlagEmbedding 参数、启动哪些进程
app.settings   → 检索 / 分块 / Milvus 索引（设置页可改）
```

日志：` .deploy/logs/`  
PID：` .deploy/pids/`

### deploy.env 常用项

```env
FLUXSEARCH_API_PORT=8080
FLUXSEARCH_FLAGEMBEDDING_DEVICE=cuda
FLUXSEARCH_FLAGEMBEDDING_SPARSE_DEVICE=cpu
FLUXSEARCH_FLAGEMBEDDING_FP16=true
FLUXSEARCH_FLAGEMBEDDING_MAX_LENGTH=512
FLUXSEARCH_DEPLOY_START_WORKER=false
```

## 远程 K8s 基础设施

| 脚本 | 说明 |
|------|------|
| `server-port-forward.sh` | 局域网 port-forward（与 externalIPs 勿同时用） |
| `apply-external-access.sh` | 为 Service 配置公网 IP（externalIPs） |
| `setup-infra-final.sh` | 集群内安装 Postgres / Redis / Milvus 等 |
| `deploy-monitor.sh` | 部署 Monitor 服务 |

详见 [config/README.md](../config/README.md)。
