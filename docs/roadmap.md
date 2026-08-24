# 路线图

## 当前进度

```text
✅ K8s 基础设施（PostgreSQL / Redis / MinIO / Milvus / etcd）
✅ V0 Minimal RAG（导入 / 检索 / 对话 / 对话历史）
✅ V1 实时摄取（Redis 事件总线、独立 Worker、异步重索引）
⏳ V2 ~ V7
```

## V0 — Minimal RAG ✅

目标：端到端可用——上传文档、向量检索、基础 RAG 对话。

- [x] `cmd/api`：Gin REST + WebSocket
- [x] `frontend/`：对话 / 检索 / 导入 / 文档 / 设置
- [x] `internal/parser`：PDF / Markdown / DOCX / TXT
- [x] `internal/chunker`：递归分块
- [x] `internal/embedding`：百炼 / Ollama（OpenAI 兼容 API）
- [x] `internal/llm`：RAG 流式生成
- [x] `internal/storage/milvus`：向量写入与检索
- [x] `internal/storage/postgres`：文档 / chunk / 对话元数据
- [x] 文档上传 API + 导入页（Redis 异步队列）
- [x] 向量检索 REST + 检索页
- [x] RAG 对话 WebSocket + 引用来源
- [x] 对话历史（`conversations` / `messages`）
- [x] 文档删除、文档/片段去重、设置页、Reindex 协调器
- [x] MinIO 原文件存储

## V1 — Realtime Ingestion ✅

目标：文档增删改实时反映到索引。

- [x] PostgreSQL 文档元数据 + migrations
- [x] MinIO 原始文件存储
- [x] Redis 导入队列 + 可选 API 内 Worker（`FLUXSEARCH_IMPORT_WORKER_IN_API`）
- [x] `cmd/worker` 独立进程（导入 + 重索引双队列）
- [x] Redis Pub/Sub 领域事件（`fluxsearch:events`，Kafka 兼容 schema，可后续迁移）
- [x] WebSocket 事件推送（`/ws/events`：导入进度、文档创建/更新/删除、重索引状态）
- [x] 异步重新导入 `POST /documents/:id/reimport`
- [x] 异步重新分块 `POST /documents/:id/rechunk?async=true`
- [ ] Kafka 事件发布与消费（当前以 Redis Pub/Sub 代替）

## V2 — Hybrid Retrieval

目标：Dense + Sparse 混合检索，提升 Recall。

- [x] Dense Retrieval（Milvus）
- [ ] BM25 / Sparse（Bleve 或 Milvus Sparse）
- [ ] `internal/retrieval/fusion`：RRF
- [ ] Metadata Filter

## V3 — Reranking

目标：精排提升 Precision，带引用回答。

- [ ] ONNX Cross-Encoder Reranker
- [ ] 多阶段检索 Top100 → Top5
- [ ] Context Builder + Citation 增强

## V4 — Evaluation

目标：可量化的检索与 RAG 评测。

- [ ] `cmd/eval`：Recall@K / MRR / nDCG
- [ ] 延迟基准测试
- [ ] RAG 评测（LLM-as-Judge）

## V5 — Model Pipeline

目标：模型导出、注册与热切换。

- [ ] `cmd/trainer`：数据集导出、Hard Negative Mining
- [ ] ONNX 模型注册与版本管理（MinIO）
- [ ] 模型热切换

## V6 — Feedback Loop

目标：用户行为驱动持续优化。

- [ ] Query / Click / Dwell 日志
- [ ] `cmd/scheduler`：定时数据集生成
- [ ] 反馈驱动 Reindex 触发

## V7 — Productionization

目标：生产级部署与可观测性。

- [ ] 模型与索引版本灰度
- [ ] Prometheus metrics 全链路
- [ ] Kubernetes Helm / Kustomize 应用部署
- [ ] Worker 水平扩展
- [ ] 前端 CDN / Ingress 部署
- [ ] 用户认证与多租户

## 里程碑时间线（参考）

| 阶段 | 状态 | 预估工期（1 人全职） |
|------|------|---------------------|
| V0 | ✅ 基本完成 | 2~3 周 |
| V1 | ✅ 基本完成 | 2~3 周 |
| V2~V3 | ⏳ | 3~4 周 |
| V4 | ⏳ | 2~3 周 |
| V5~V7 | ⏳ | 按需推进 |
