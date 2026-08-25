# 架构设计

## 1. 总体架构

FluxSearch 采用**前后端分离 + Go 后端 Monorepo** 架构。前端负责交互与展示，后端负责业务逻辑、检索、流式生成与异步任务；多个 Go 二进制共享 `internal/` 代码库。

```text
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│              React + Vite + Tailwind + DaisyUI               │
└──────────────────────────┬──────────────────────────────────┘
                           │ REST + WebSocket
┌──────────────────────────▼──────────────────────────────────┐
│                      Application Layer                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  fluxsearch │  │  fluxsearch │  │ fluxsearch-monitor  │  │
│  │    -api     │  │   -worker   │  │ ensure-milvus       │  │
│  │ Gin + WS    │  │  Ingestion  │  │ (scheduler/trainer/ │  │
│  │             │  │  + Reindex  │  │  eval 占位)         │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
└─────────┼─────────────────┼─────────────────────┼─────────────┘
          │                 │                     │
┌─────────▼─────────────────▼─────────────────────▼─────────────┐
│                      Infrastructure Layer                      │
│   PostgreSQL   Redis   MinIO   Milvus   etcd                   │
│   (Kafka 规划中，V1 以 Redis Pub/Sub 代替)                      │
└───────────────────────────────────────────────────────────────┘
```

## 2. 核心数据流

### 2.1 检索与 RAG（当前实现）

```text
User Query
    │
    ▼
internal/retrieval.Service
    │
    ├─ SearchHybridEnabled ──▶ Milvus HybridSearch (Dense + Sparse)
    │
    └─ 否则 ──▶ Milvus Dense Search
    │
    ▼
按 document_id 聚合（每文档保留最高分 chunk）
    │
    ├─ SearchRerankEnabled ──▶ HTTP Cross-Encoder Rerank
    │
    └─ 否则 ──▶ 直接截断 Top-K
    │
    ▼
internal/chat.Service → LLM 流式生成
    │
    ▼
WebSocket 推送 Answer + Sources
```

响应字段 `mode` 示例：`dense` / `hybrid` / `dense+rerank` / `hybrid+rerank`。

### 2.2 实时索引（V1）

```text
Document Create / Update / Delete / Reimport / Rechunk
              │
              ▼
        API (Go)
              │
    ┌─────────┴──────────┐
    ▼                    ▼
Redis 导入队列      Redis Pub/Sub 事件
fluxsearch:import   fluxsearch:events
fluxsearch:reindex       │
    │                    ▼
    ▼              API 订阅 → WS /ws/events
fluxsearch-worker
(API 内嵌 Worker 可选)
    │
Parse → Chunk → Embedding → Milvus
    │
    ▼
PostgreSQL 元数据 + MinIO 原文件
```

生产环境建议 `FLUXSEARCH_IMPORT_WORKER_IN_API=false`，由独立 Worker 消费队列。

## 3. 服务划分

| 二进制 | 路径 | 职责 | 状态 |
|--------|------|------|------|
| `fluxsearch-api` | `cmd/api` | Gin HTTP API、WebSocket、检索、RAG、设置 | ✅ 已实现 |
| `fluxsearch-worker` | `cmd/worker` | Redis 导入 + 重索引队列消费 | ✅ 已实现 |
| `fluxsearch-monitor` | `cmd/monitor` | 基础设施健康与指标采集 | ✅ 已实现 |
| `ensure-milvus` | `cmd/ensure-milvus` | Milvus collection 创建 / 重建 | ✅ 已实现 |
| `fluxsearch-scheduler` | `cmd/scheduler` | 定时任务 | ⏳ 占位 |
| `fluxsearch-trainer` | `cmd/trainer` | 训练数据 / 模型注册 | ⏳ 占位 |
| `fluxsearch-eval` | `cmd/eval` | Go 评测 CLI | ⏳ 占位（评测见 `eval/` Python 流水线） |

## 4. 前后端通信

| 场景 | 协议 | 端点 |
|------|------|------|
| 文档 CRUD | REST | `/api/v1/documents` |
| 检索查询 | REST | `/api/v1/search` |
| RAG 流式对话 | WebSocket | `/api/v1/ws/chat` |
| 导入 / 索引 / 文档事件 | WebSocket | `/api/v1/ws/events` |
| 对话历史 | REST | `/api/v1/conversations` |
| 健康检查 | REST | `/healthz` |

开发模式下 Vite（`:5173`）通过 proxy 转发至 Gin（`:8080`），WebSocket 同样走代理。

## 5. 包依赖方向

```text
frontend/          独立构建，通过 HTTP/WS 通信
    │
    ▼
cmd/*  →  internal/*  →  pkg/* (暂无)
              │
              ▼
         storage/     PostgreSQL / Redis / MinIO / Milvus
```

主要 domain 包：

| 包 | 职责 |
|----|------|
| `bootstrap` | 依赖注入、Worker / API 初始化 |
| `ingestion` | 解析、分块、Embedding、写入索引 |
| `importqueue` | Redis 导入与重索引队列 |
| `events` | Redis Pub/Sub 领域事件 |
| `retrieval` | 检索编排（Dense / Hybrid / Rerank） |
| `chat` | RAG 对话与流式生成 |
| `conversation` | 对话类型定义（避免循环依赖） |
| `settings` | 应用设置与 Reindex 协调 |

## 6. 部署拓扑（目标）

```text
                    Traefik Ingress
                          │
            ┌─────────────┴─────────────┐
            ▼                           ▼
      frontend (静态)              fluxsearch-api
      nginx / CDN                  (多副本)
                                         │
            ┌────────────────────────────┼────────────────┐
            ▼                            ▼                ▼
      fluxsearch-worker            PostgreSQL          Milvus
      (水平扩展)                    Redis / MinIO       etcd
```

当前已在 K3s 单节点部署基础设施层（`fluxsearch` 命名空间），应用层待 Helm / Kustomize 部署。
