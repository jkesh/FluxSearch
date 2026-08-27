# FluxSearch 配置目录

连接信息与密钥的存放规范。实际密码文件不提交 Git。

## 文件说明

| 文件 | 用途 | Git 跟踪 |
|------|------|----------|
| `infra.example.env` | 基础设施连接模板 | ✅ |
| `deploy.example.env` | 本地服务端口 / FlagEmbedding / 一键部署开关 | ✅ |
| `app.settings.example.json` | 应用设置模板（Embedding/LLM/检索等） | ✅ |
| `local/infra.env` | 实际连接信息（.env） | ❌ |
| `local/deploy.env` | 本地运行时 / 部署参数 | ❌ |
| `local/infra.yaml` | 实际连接信息（YAML） | ❌ |
| `local/app.settings.json` | 实际 API Key 与模型配置 | ❌ |

## 首次使用

```bash
mkdir -p config/local
cp config/infra.example.env config/local/infra.env
cp config/deploy.example.env config/local/deploy.env
cp config/infra.example.yaml config/local/infra.yaml
cp config/app.settings.example.json config/local/app.settings.json
# 编辑填入实际密码与 API Key
```

## 一键本地启动

**Windows（PowerShell）：**

```powershell
.\deploy\scripts\deploy-local.ps1
.\deploy\scripts\stop-local.ps1
```

**Git Bash / Linux / macOS：**

```bash
bash deploy/scripts/deploy-local.sh
bash deploy/scripts/stop-local.sh
```

首次运行会自动从 `*.example` 复制 `config/local/` 下的配置文件。进程日志在 `.deploy/logs/`，PID 在 `.deploy/pids/`。

主要可调参数：

| 文件 | 内容 |
|------|------|
| `config/local/infra.env` | Postgres / Redis / Milvus / MinIO / Embedding 密钥 |
| `config/local/deploy.env` | API/前端端口、FlagEmbedding 设备与模型、启动哪些服务 |
| `config/local/app.settings.json` | 检索、分块、Milvus 索引（也可在设置页修改） |

## 在代码中加载

### Go

```go
// 推荐：通过环境变量或 viper 加载 config/local/infra.env
// 生产环境使用 K8s Secret fluxsearch-infra
```

### Shell

```bash
set -a
source config/local/infra.env
set +a
```

## 本地开发 port-forward

```bash
kubectl port-forward -n fluxsearch svc/postgres 5432:5432 &
kubectl port-forward -n fluxsearch svc/redis   6379:6379 &
kubectl port-forward -n fluxsearch svc/milvus  19530:19530 &
kubectl port-forward -n fluxsearch svc/minio   9000:9000 &
```

使用 `infra.env` 中 `*_LOCAL` / `local_dev` 段地址连接。

### 常用环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `FLUXSEARCH_IMPORT_WORKER_IN_API` | `true` | API 是否消费导入/重索引队列；生产建议 `false` 并单独运行 Worker |
| `FLUXSEARCH_MILVUS_COLLECTION` | `fluxsearch_default` | 覆盖 Milvus collection（评测时使用 eval collection） |
| `FLUXSEARCH_EMBEDDING_*` | — | Embedding 提供商与模型（可被 `app.settings.json` 覆盖） |

详细基础设施说明见 [docs/infrastructure.md](../docs/infrastructure.md)。
