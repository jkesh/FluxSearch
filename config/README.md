# FluxSearch 配置目录

连接信息与密钥的存放规范。实际密码文件不提交 Git。

## 文件说明

| 文件 | 用途 | Git 跟踪 |
|------|------|----------|
| `infra.example.env` | 环境变量模板 | ✅ |
| `infra.example.yaml` | YAML 配置模板 | ✅ |
| `app.settings.example.json` | 应用设置模板（Embedding/LLM 等） | ✅ |
| `local/infra.env` | 实际连接信息（.env） | ❌ |
| `local/infra.yaml` | 实际连接信息（YAML） | ❌ |
| `local/app.settings.json` | 实际 API Key 与模型配置 | ❌ |

## 首次使用

```bash
mkdir -p config/local
cp config/infra.example.env config/local/infra.env
cp config/infra.example.yaml config/local/infra.yaml
cp config/app.settings.example.json config/local/app.settings.json
# 编辑填入实际密码与 API Key
```

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

详细基础设施说明见 [docs/infrastructure.md](../docs/infrastructure.md)。
