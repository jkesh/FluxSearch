# 检索与索引

## 1. 检索流水线（当前实现）

FluxSearch 检索由 `internal/retrieval.Service` 编排，按设置页开关组合以下阶段：

| 阶段 | 职责 | 实现 | 状态 |
|------|------|------|------|
| 1. Dense Retrieval | 语义召回 | Embedding → Milvus Search | ✅ |
| 2. Hybrid Retrieval | Dense + Sparse | FlagEmbedding Hybrid → Milvus HybridSearch | ✅（可选） |
| 2b. Dense + BM25 | 语义 + 关键词 | Dense ANN + 内存 BM25，加权 RRF | ✅（可选） |
| 3. Document 聚合 | 每文档保留最高分 chunk | `aggregateHitsByDocument` | ✅ |
| 4. Reranking | 精排 recallK → topK | HTTP Cross-Encoder（`internal/rerank`） | ✅（可选） |
| 5. Context + LLM | RAG 回答 + 引用 | `internal/chat` | ✅ |

API 响应字段 `mode`：

| mode | 含义 |
|------|------|
| `dense` | 纯向量检索 |
| `sparse_hybrid` | Milvus Hybrid Search（Dense + 学习型 Sparse） |
| `dense_bm25` | Dense + BM25 加权 RRF |
| `bm25` | 仅 BM25（Dense 权重为 0） |
| `dense+rerank` / `*_+rerank` | 上述任意模式 + Cross-Encoder |

### 设置项（`app.settings.json` / 设置页）

| 字段 | 说明 |
|------|------|
| `search_mode` | `dense` / `sparse_hybrid` / `dense_bm25` |
| `search_hybrid_enabled` | Sparse Hybrid 时启用 Milvus Sparse schema |
| `search_dense_weight` / `search_bm25_weight` | `dense_bm25` 模式融合权重 |
| `search_rerank_enabled` | 启用 Cross-Encoder 精排 |
| `search_recall_k` | 召回候选数（默认 50） |
| `search_top_k` | 最终返回数（默认 5） |
| `rerank_model` / `rerank_api_url` | 精排模型与 API |

Hybrid 模式需 Embedding 后端支持 Hybrid（如 FlagEmbedding BGE-M3）；Rerank 需启动 FlagEmbedding 服务：

```bash
make run-flagembedding   # scripts/flagembedding_server.py :8091
```

## 2. 实时索引（V1）

### 流程

```text
Document Create / Update / Delete / Reimport / Rechunk
              │
              ▼
        API ──LPUSH──▶ Redis 队列
              │
              ├── fluxsearch:import:queue   （批量导入）
              └── fluxsearch:reindex:queue  （单文档重导入 / 重分块）
              │
              ▼
     fluxsearch-worker（或 API 内嵌 Worker）
              │
    Parse → Chunk → Embed → Milvus + PostgreSQL
              │
              ▼
     Redis Pub/Sub → WS /ws/events
```

### Chunk 数据模型

```text
chunk_id
document_id
document_version
content_hash          # 内容哈希，用于去重
content
dense_vector
sparse_vector         # Hybrid 模式
page / section
metadata
```

文档级 / 片段级去重可在设置页配置。异步重导入与重分块通过 Redis 重索引队列处理。

## 3. 模型与索引版本管理

```text
Retriever V1 → Index V1
Retriever V2 → Index V2
```

升级流程（目标）：

```text
Export / 切换 Embedding 模型
        │
        ▼
Offline Evaluation (eval/ Python)
        │
        ▼
Full Reindex（settings 页或 Worker）
        │
        ▼
Canary → 全量切换
```

当前已通过设置页 Reindex 协调器触发全量重建；灰度切换待 V7 实现。

## 4. 训练流水线（规划）

```text
Query Logs / Click / Feedback
              │
              ▼
   Dataset Builder (cmd/trainer)
              │
              ▼
   Hard Negative Mining
              │
              ▼
   Export Training Set (JSONL)
              │
              ▼
   外部 Fine-tune 或 预训练模型
              │
              ▼
   Offline Evaluation → Reindex → Deploy
```

## 5. 评测体系

### 检索指标

| 指标 | 说明 |
|------|------|
| Hit@K | 前 K 命中至少一个相关文档的比例 |
| Recall@K | 召回率 |
| MRR@K | 平均倒数排名 |
| Latency P50/P95 | 端到端检索延迟 |

### 已实现：Python 评测流水线

位于 `eval/`，对接 `POST /api/v1/search`，输出 JSON 报告至 `eval/reports/`。

| 数据集 | 语料 | Query | 说明 |
|--------|------|-------|------|
| SciFact | 5,183 | 300 | 推荐冒烟与对比 |
| CQADupStack Unix | 47,382 | 1,072 | 大规模、难度高 |

快速开始见 [eval/README.md](../eval/README.md)。

示例（SciFact，Hybrid + Rerank 开启后）：

```text
Recall@10 ≈ 82.6%   MRR@10 ≈ 0.66   P50 ≈ 339ms
```

（具体数值随 Embedding / 检索配置变化，以 `eval/reports/*.json` 为准。）

### 规划

| 组件 | 状态 |
|------|------|
| `eval/` Python 流水线 | ✅ |
| `cmd/eval` Go CLI | ⏳ 占位 |
| RAG 评测（LLM-as-Judge） | ⏳ |
| 延迟基准自动化 | ⏳ |

所有检索策略变更应通过 `eval/` 离线评测验证后再上线。

## 6. RAG 流式生成

```text
用户提问 → WS /api/v1/ws/chat
    → retrieval.Search Top-K
    → 构建 Prompt + Sources
    → LLM 流式生成
    → { type: "token" } × N
    → { type: "done", sources, conversation_id }
```

索引与文档事件通过 `WS /api/v1/ws/events` 推送（`import_progress`、`document.*`）。
