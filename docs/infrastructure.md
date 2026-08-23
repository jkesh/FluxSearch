# 基础设施

## 部署环境

| 项目 | 值 |
|------|-----|
| 集群 | K3s v1.36（单节点） |
| 节点 | `platform-01` @ `<your-k8s-node>` |
| 命名空间 | `fluxsearch` |
| 存储类 | `local-path` |

## 已部署组件

| 组件 | 类型 | 集群内地址 | 状态 |
|------|------|-----------|------|
| PostgreSQL | StatefulSet | `postgres.fluxsearch.svc:5432` | ✅ Running |
| Redis | StatefulSet | `redis.fluxsearch.svc:6379` | ✅ Running |
| MinIO | Deployment | `minio.fluxsearch.svc:9000` | ✅ Running |
| etcd | Deployment | `etcd.fluxsearch.svc:2379` | ✅ Running |
| Milvus | Deployment (standalone) | `milvus.fluxsearch.svc:19530` | ✅ Running |

### Milvus 部署说明

采用 **standalone + 外接 etcd + MinIO** 轻量方案（非 Helm 全量 Pulsar 模式）：

- Milvus v2.4.17，`milvus run standalone`
- 外接 etcd（元数据）和 MinIO（对象存储）
- 避免 Pulsar 组件带来的资源开销

## 资源配置

命名空间 `ResourceQuota`：

| 资源 | 限额 |
|------|------|
| CPU requests | 3 核 |
| Memory requests | 12 Gi |
| CPU limits | 5 核 |
| Memory limits | 20 Gi |
| PVC 数量 | 8 |

## 配置管理

连接信息存放在 `config/local/`（不提交 Git）：

```text
config/
├── infra.example.env       # 模板
├── infra.example.yaml      # 模板
└── local/
    ├── infra.env           # 实际配置
    └── infra.yaml          # 实际配置
```

K8s Secret `fluxsearch-infra`（命名空间 `fluxsearch`）包含应用运行时所需的环境变量。

### 首次配置

```bash
cp config/infra.example.env config/local/infra.env
cp config/infra.example.yaml config/local/infra.yaml
# 填入实际密码
```

## 本地访问（port-forward）

```bash
kubectl port-forward -n fluxsearch svc/postgres 5432:5432 &
kubectl port-forward -n fluxsearch svc/redis   6379:6379 &
kubectl port-forward -n fluxsearch svc/milvus  19530:19530 &
kubectl port-forward -n fluxsearch svc/minio   9000:9000 &
```

| 服务 | 本地地址 |
|------|----------|
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |
| Milvus | `127.0.0.1:19530` |
| MinIO | `127.0.0.1:9000` |

## 存储职责

### PostgreSQL — 业务数据

```text
users / organizations / documents / collections
permissions / conversations / messages
training_jobs / model_versions / index_versions
```

### Milvus — 检索索引

```text
chunk_id / document_id / document_version
content / dense_vector / sparse_vector
metadata / embedding_model_version
```

### MinIO — 对象存储

```text
原始文档（PDF / Word / Markdown / HTML）
训练数据集 / ONNX 模型文件
```

### Redis — 缓存与协调

```text
查询缓存 / 限流 / Session
分布式锁 / 短生命周期任务状态
```

### Kafka — 事件流（V1+）

```text
document.created / document.updated / document.deleted
embedding.requested / index.updated
query.logged / feedback.created
```

## 应用部署（规划）

```text
fluxsearch-api       Deployment  (1~3 replicas)
fluxsearch-worker    Deployment  (按队列深度扩展)
frontend             Deployment  (nginx 托管 dist/)
```

入口通过集群已有 Traefik Ingress 暴露，TLS 由 cert-manager 管理。

## 运维脚本

| 脚本 | 用途 |
|------|------|
| `deploy/scripts/setup-infra.sh` | 初始基础设施部署 |
| `deploy/scripts/fix-milvus-v2.sh` | Milvus standalone 修复 |
| `deploy/scripts/check-status.sh` | 检查 Pod / PVC 状态 |
| `deploy/scripts/remote-run.py` | 远程执行部署脚本 |

## 网络与代理

远程节点拉取 Docker Hub / Helm Chart 可能超时。本地开发机 v2rayN 代理：

- SOCKS5：`127.0.0.1:10808`
- 通过 SSH 隧道转发：`remote-run.py --tunnel`

集群已配置 daocloud 镜像加速（`docker.io` / `quay.io` / `registry.k8s.io`）。
