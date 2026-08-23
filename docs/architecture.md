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
│  │  fluxsearch │  │  fluxsearch │  │ fluxsearch-scheduler│  │
│  │    -api     │  │   -worker   │  │  fluxsearch-trainer │  │
│  │ Gin + WS    │  │  Ingestion  │  │  fluxsearch-eval    │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
└─────────┼─────────────────┼─────────────────────┼─────────────┘
          │                 │                     │
┌─────────▼─────────────────▼─────────────────────▼─────────────┐
│                      Infrastructure Layer                      │
│   PostgreSQL   Redis   MinIO   Kafka   Milvus   etcd           │
└───────────────────────────────────────────────────────────────┘
```

## 2. 核心数据流

### 2.1 检索与 RAG

```text
User Query
    │
    ▼
Query Processing (Go)
    │
    ├───────────────────┐
    ▼                   ▼
Dense Retrieval      BM25 / Sparse
    │                   │
    └─────────┬─────────┘
              ▼
           RRF Fusion → Top 100
              ▼
       Cross-Encoder Reranker → Top 5~10
              ▼
       Context Builder
              ▼
          LLM Client
              ▼
      Answer + Citation
              │
              ▼
    WebSocket 流式推送至前端
```

### 2.2 实时索引

```text
Document Create / Update / Delete
              │
              ▼
        API (Go) ──publish──▶ Kafka
              │
              ▼
     Ingestion Worker (Go)
              │
    Parse → Chunk → Embedding
              │
              ▼
          Milvus Index
              │
              ▼
     WebSocket 通知索引进度
```

数据更新与模型训练相互独立：新文档使用当前 Embedding 模型直接索引，无需重训。

## 3. 服务划分

| 二进制 | 路径 | 职责 | 状态 |
|--------|------|------|------|
| `fluxsearch-api` | `cmd/api` | Gin HTTP API、WebSocket、检索、RAG 流式生成 | 🚧 骨架完成 |
| `fluxsearch-worker` | `cmd/worker` | Kafka 消费、解析、Chunk、Embedding、写 Milvus | ⏳ 待实现 |
| `fluxsearch-scheduler` | `cmd/scheduler` | 定时任务：数据集生成、评测、Reindex | ⏳ 待实现 |
| `fluxsearch-trainer` | `cmd/trainer` | 训练数据导出、Hard Negative Mining、模型注册 | ⏳ 待实现 |
| `fluxsearch-eval` | `cmd/eval` | 离线检索 / RAG 评测 CLI | ⏳ 待实现 |

## 4. 前后端通信

| 场景 | 协议 | 端点 |
|------|------|------|
| 文档 CRUD | REST | `/api/v1/documents` |
| 检索查询 | REST | `/api/v1/search` |
| RAG 流式对话 | WebSocket | `/api/v1/ws/chat` |
| 任务 / 索引进度 | WebSocket | `/api/v1/ws/events` |
| 健康检查 | REST | `/healthz` |

开发模式下 Vite（`:5173`）通过 proxy 转发至 Gin（`:8080`），WebSocket 同样走代理。

## 5. 包依赖方向

```text
frontend/          独立构建，通过 HTTP/WS 通信
    │
    ▼
cmd/*  →  internal/*  →  pkg/*
              │
              ▼
         storage/     基础设施适配层（PG / Redis / MinIO / Milvus / Kafka）
```

`internal/` 各模块通过接口解耦，便于单测与替换实现（如切换 Embedding 后端）。

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
                                         │
                                       Kafka
```

当前已在 K3s 单节点部署基础设施层（`fluxsearch` 命名空间），应用层待部署。
