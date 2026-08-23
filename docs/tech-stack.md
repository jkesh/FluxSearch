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
| SQL | [sqlc](https://sqlc.dev) + [pgx](https://github.com/jackc/pgx) | 类型安全 SQL（规划中） |
| 配置 | `config/local` + 环境变量 | 本地与集群配置分离 |

## 数据与中间件

| 类别 | 技术 | 用途 |
|------|------|------|
| 业务数据库 | PostgreSQL 16 | 用户、文档、会话、模型版本 |
| 向量数据库 | Milvus 2.4 (standalone) | Dense / Sparse 向量检索 |
| 元数据 | etcd 3.5 | Milvus 元数据存储 |
| 缓存 | Redis 7 | 缓存、限流、分布式锁 |
| 对象存储 | MinIO | 原始文件、ONNX 模型 |
| 消息队列 | Kafka | 文档事件、异步索引（V1+） |

## 检索与 AI

| 类别 | 技术 | 说明 |
|------|------|------|
| 文档解析 | `ledongthuc/pdf`、`nguyenthenguyen/docx`、`goldmark` | PDF / Word / Markdown |
| 分块 | `internal/chunker` | 固定窗口 / 语义分块 |
| Embedding | ONNX Runtime Go / OpenAI-compatible API | 向量生成 |
| 稀疏检索 | Bleve / Milvus Sparse Vector | BM25 关键词匹配 |
| 融合 | `internal/retrieval/fusion` | RRF 多路召回合并 |
| 精排 | ONNX Cross-Encoder | Top100 → Top5 |
| LLM | OpenAI-compatible HTTP Client | RAG 答案生成 |

## 运维

| 类别 | 技术 | 说明 |
|------|------|------|
| 容器编排 | K3s / Kubernetes | 当前运行于 K3s 单节点 |
| 入口 | Traefik | 集群已有 Ingress |
| 监控 | Prometheus + Grafana | 集群已有监控栈 |
| GitOps | ArgoCD | 集群已有（规划中） |
| 指标 | prometheus/client_golang | 应用指标暴露 |

## 技术决策摘要

| 决策 | 选择 | 原因 |
|------|------|------|
| 前后端分离 | React + Go API | UI 迭代与后端解耦，WebSocket 流式体验好 |
| 后端语言 | 纯 Go | 统一技术栈，Worker / API / 评测共享代码 |
| 实时通信 | WebSocket | RAG Token 流式、索引进度推送 |
| Embedding 推理 | ONNX 优先 | 避免额外 Python 推理服务 |
| 向量库 | Milvus standalone | 支持 Hybrid Search，运维成本可控 |
| SQL 生成 | sqlc | 编译期类型检查，优于 ORM 魔法 |
