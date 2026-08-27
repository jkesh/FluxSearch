# 技术栈

## 前端

| 类别 | 技术 | 说明 |
|------|------|------|
| 框架 | React 19 | 组件化 UI |
| 构建 | Vite 6 | 开发服务器、HMR、生产打包 |
| 语言 | TypeScript | 类型安全 |
| 样式 | Tailwind CSS 3 | 原子化 CSS |
| 组件库 | DaisyUI 4 | 基于 Tailwind 的 UI 组件 |
| 路由 | React Router 7 | 客户端路由 |
| 实时通信 | 原生 WebSocket | `useWebSocket` Hook 封装 |

## 后端

| 类别 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.25+ | 后端唯一语言 |
| HTTP 框架 | [Gin](https://github.com/gin-gonic/gin) | 路由、中间件、JSON 绑定 |
| WebSocket | [gorilla/websocket](https://github.com/gorilla/websocket) | 流式 RAG、事件推送 |
| SQL | [pgx](https://github.com/jackc/pgx) | PostgreSQL 驱动（手写 SQL） |
| 配置 | `config/local` + 环境变量 + `app.settings.json` | 本地与集群配置分离 |

## 数据与中间件

| 类别 | 技术 | 用途 |
|------|------|------|
| 业务数据库 | PostgreSQL 16 | 文档、会话、评测元数据 |
| 向量数据库 | Milvus 2.4 (standalone) | Dense / Hybrid 向量检索 |
| 元数据 | etcd 3.5 | Milvus 元数据存储 |
| 缓存 / 队列 / 事件 | Redis 7 | 导入队列、重索引队列、Pub/Sub 事件 |
| 对象存储 | MinIO | 原始文件、导入 staging |
| 消息队列 | Kafka（规划） | V1 以 Redis Pub/Sub 代替 |

## 检索与 AI

| 类别 | 技术 | 说明 |
|------|------|------|
| 文档解析 | `ledongthuc/pdf`、`nguyenthenguyen/docx`、`goldmark` | PDF / Word / Markdown |
| 分块 | `internal/chunker` | 递归字符分块（可配置 max/overlap） |
| Embedding | OpenAI-compatible API / Ollama | 百炼、本地 Ollama 等 |
| Hybrid Embedding | FlagEmbedding BGE-M3（HTTP） | Dense + Sparse 联合向量 |
| 稀疏检索 | Milvus Sparse / 内存 BM25 | Sparse Hybrid 或 Dense+BM25 |
| 融合 | Milvus HybridSearch / 加权 RRF | Sparse Hybrid 用前者；Dense+BM25 用后者 |
| 精排 | HTTP Cross-Encoder（OpenAI-compatible） | FlagEmbedding rerank API |
| LLM | OpenAI-compatible HTTP Client | RAG 答案生成 |
| 评测 | Python + BEIR 数据集 | `eval/scifact`、`eval/cqadupstack_unix` |

## 运维

| 类别 | 技术 | 说明 |
|------|------|------|
| 容器编排 | K3s / Kubernetes | 当前运行于 K3s 单节点 |
| 入口 | Traefik | 集群已有 Ingress |
| 监控 | Prometheus + Grafana | 集群已有监控栈 |
| 应用监控 | `cmd/monitor` | 依赖健康与基础指标 |
| GitOps | ArgoCD | 集群已有（规划中） |

## 技术决策摘要

| 决策 | 选择 | 原因 |
|------|------|------|
| 前后端分离 | React + Go API | UI 迭代与后端解耦，WebSocket 流式体验好 |
| 后端语言 | 纯 Go | 统一技术栈，Worker / API 共享代码 |
| 实时通信 | WebSocket | RAG Token 流式、导入/索引事件推送 |
| V1 事件总线 | Redis Pub/Sub | 轻量、与现有 Redis 复用，schema 兼容 Kafka |
| Embedding | Remote API 优先 | 百炼 / Ollama / FlagEmbedding 快速接入 |
| Hybrid 检索 | Milvus HybridSearch | 避免维护独立 Sparse 引擎 |
| Rerank | HTTP Cross-Encoder | 与 Embedding 共用 FlagEmbedding 服务 |
| 向量库 | Milvus standalone | 支持 Hybrid Search，运维成本可控 |
| 离线评测 | Python BEIR 流水线 | 数据集生态成熟，报告输出 JSON |
