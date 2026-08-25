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
cp config/app.settings.example.json config/local/app.settings.json
# 编辑 config/local/ 填入数据库连接与 API Key

# 2. 后端 API（:8080）
go run ./cmd/api

# 可选：独立 Worker（生产建议 FLUXSEARCH_IMPORT_WORKER_IN_API=false）
go run ./cmd/worker

# 3. 前端（:5173）
cd frontend && npm run dev
```

访问 http://localhost:5173

## 功能概览

| 模块 | 能力 |
|------|------|
| 文档导入 | PDF / MD / DOCX / TXT 解析、分块、Embedding、写入 Milvus |
| 导入队列 | Redis 异步队列 + 独立 Worker + WebSocket 进度/事件推送 |
| 文档更新 | 异步重新导入、异步重新分块（Redis 重索引队列） |
| 向量检索 | Dense / Hybrid（Milvus）+ 可选 Cross-Encoder 精排 |
| RAG 对话 | 知识库检索 + LLM 流式回答 + 引用来源 |
| 对话历史 | PostgreSQL 持久化、多轮上下文 |
| 文档管理 | 列表、详情、删除（PG + Milvus + MinIO） |
| 去重 | 文档级 / 片段级可配置 |
| 设置 | Embedding / LLM / 检索 / Milvus 索引 / 全量 Reindex |
| 检索评测 | BEIR SciFact / CQADupStack Python 流水线（`eval/`） |

## 文档

| 文档 | 说明 |
|------|------|
| [架构设计](docs/architecture.md) | 系统架构、服务划分、数据流 |
| [API 规范](docs/api.md) | REST 与 WebSocket 接口 |
| [开发指南](docs/development.md) | 本地开发、项目结构 |
| [基础设施](docs/infrastructure.md) | K8s 部署、存储、配置 |
| [检索与评测](docs/retrieval.md) | 检索流水线、索引、评测 |
| [路线图](docs/roadmap.md) | 版本规划与当前进度 |
| [配置说明](config/README.md) | 环境变量与密钥管理 |
| [评测指南](eval/README.md) | BEIR 离线检索评测 |

## 当前状态（2026-08）

```text
✅ V0 Minimal RAG — 文档导入、向量检索、RAG 对话、对话历史
✅ V1 实时摄取 — Redis 事件总线、独立 Worker、异步重索引
🚧 V2~V4 — Hybrid / Rerank / Python 评测已部分实现，持续完善中
⏳ V5~V7 — 模型流水线、反馈闭环、生产化
```

详见 [路线图](docs/roadmap.md)。

## License

TBD
