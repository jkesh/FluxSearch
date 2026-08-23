# FluxSearch

面向实时数据场景的 **RAG / Search Retrieval Platform**——支持文档索引、向量检索、流式对话与知识库管理。

## 技术概览

| 层 | 技术 |
|----|------|
| 前端 | React 19 · Vite · TypeScript · Tailwind · DaisyUI |
| 后端 | Go · Gin · WebSocket |
| 基础设施 | PostgreSQL · Redis · MinIO · Milvus · K3s |

## 快速开始

```bash
# 1. 配置（首次）
mkdir -p config/local
cp config/infra.example.env config/local/infra.env
# 编辑 config/local/infra.env 填入数据库等连接信息

# 2. 后端 API（:8080）
go run ./cmd/api
# 或 Windows: .\scripts\dev.ps1 api

# 3. 前端（:5173）
cd frontend && npm run dev
# 或: .\scripts\dev.ps1 frontend
```

访问 http://localhost:5173

## 功能概览

| 模块 | 能力 |
|------|------|
| 文档导入 | PDF / MD / DOCX / TXT 解析、分块、Embedding、写入 Milvus |
| 导入队列 | Redis 异步队列 + WebSocket 进度推送 |
| 向量检索 | Dense 检索（Milvus + 百炼/Ollama Embedding） |
| RAG 对话 | 知识库检索 + LLM 流式回答 + 引用来源 |
| 对话历史 | PostgreSQL 持久化、多轮上下文 |
| 文档管理 | 列表、详情、删除（PG + Milvus + MinIO） |
| 去重 | 文档级 / 片段级可配置 |
| 设置 | Embedding / LLM / Milvus 索引 / 去重 / 全量 Reindex |

## 文档

| 文档 | 说明 |
|------|------|
| [架构设计](docs/architecture.md) | 系统架构、服务划分、数据流 |
| [API 规范](docs/api.md) | REST 与 WebSocket 接口 |
| [开发指南](docs/development.md) | 本地开发、项目结构 |
| [基础设施](docs/infrastructure.md) | K8s 部署、存储、配置 |
| [路线图](docs/roadmap.md) | 版本规划与当前进度 |
| [配置说明](config/README.md) | 环境变量与密钥管理 |

## 当前状态（2026-08）

```text
✅ V0 Minimal RAG — 文档导入、向量检索、RAG 对话、对话历史
🚧 V1 实时摄取 — Redis 队列与 MinIO 已就绪，Kafka / 独立 Worker 待实现
⏳ V2~V7 — Hybrid Search、Rerank、评测、生产化
```

详见 [路线图](docs/roadmap.md)。

## License

TBD
