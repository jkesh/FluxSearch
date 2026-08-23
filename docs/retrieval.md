# 检索与索引

## 1. 多阶段检索流水线

FluxSearch 检索分为六个阶段，Retriever 保证 Recall，Reranker 提升 Precision。

| 阶段 | 职责 | 实现 |
|------|------|------|
| 1. Dense Retrieval | 语义召回 | ONNX Bi-Encoder → Milvus |
| 2. Sparse Retrieval | 关键词精确匹配 | Bleve / Milvus BM25 |
| 3. RRF Fusion | 多路结果合并 | `internal/retrieval/fusion` |
| 4. Reranking | 精排 Top100 → Top5 | ONNX Cross-Encoder |
| 5. Context Builder | 去重、Token Budget、引用 | `internal/generation` |
| 6. LLM Generation | 生成带引用的回答 | OpenAI-compatible API |

## 2. 实时索引

### 流程

```text
Document Create / Update / Delete
              │
              ▼
        API ──publish──▶ Kafka
              │
              ▼
     Ingestion Worker
              │
    Parse → Chunk → Embed → Milvus
              │
              ▼
     chunk_hash 增量更新
```

### Chunk 数据模型

```text
chunk_id
document_id
document_version
chunk_hash          # 内容哈希，用于增量更新
content
dense_vector
sparse_vector
page
section
metadata
embedding_model_version
```

文档更新时通过 `chunk_hash` 判断内容是否变化，**仅重新处理变更的 Chunk**。

新数据进入系统后使用当前 Retriever 生成 Embedding 并写入索引，**不需要重新训练模型**。

## 3. 模型与索引版本管理

```text
Retriever V1 → Index V1
Retriever V2 → Index V2
Retriever V3 → Index V3
```

### 发布流程

```text
Export ONNX Model
        │
        ▼
Offline Evaluation (cmd/eval)
        │
        ▼
Build New Index (Worker 全量 / 增量)
        │
        ▼
Canary: 5% → 20% → 100%
        │
        ▼
回收旧模型与旧索引
```

模型升级不直接覆盖生产模型，确认新版本稳定后再切换。

## 4. 训练流水线

```text
Query Logs / Click / Feedback / Dwell Time
              │
              ▼
   Dataset Builder (Go)
              │
              ▼
   Hard Negative Mining (Go)
              │
              ▼
   Export Training Set (JSONL)
              │
              ▼
   外部 Fine-tune（可选）或 预训练模型 → ONNX
              │
              ▼
   Model Registry (Go + MinIO)
              │
              ▼
   Offline Evaluation → Reindex → Canary Deploy
```

### 推荐频率

| 任务 | 频率 | 组件 |
|------|------|------|
| Document Indexing | 实时 | `fluxsearch-worker` |
| Query / Feedback Logging | 实时 | `fluxsearch-api` |
| Dataset Generation | 每日 | `fluxsearch-scheduler` |
| Hard Negative Mining | 每日 | `fluxsearch-trainer` |
| Retrieval Evaluation | 每日 | `fluxsearch-eval` |
| Model Export / ONNX | 按需 | `fluxsearch-trainer` |
| Full Reindex | 模型升级时 | `fluxsearch-worker` |

## 5. 评测体系

### 检索指标

| 指标 | 说明 |
|------|------|
| Recall@1 / @5 / @10 | 召回率 |
| MRR | 平均倒数排名 |
| nDCG@10 | 归一化折损累积增益 |

### 系统指标

| 指标 | 说明 |
|------|------|
| QPS | 每秒查询数 |
| P50 / P95 / P99 Latency | 延迟分位 |
| Embedding / Retrieval / Rerank Latency | 各阶段耗时 |
| Index Build Time | 索引构建时间 |

### RAG 指标

| 指标 | 说明 |
|------|------|
| Answer Correctness | 答案正确性 |
| Faithfulness | 忠实度（不编造） |
| Citation Accuracy | 引用准确率 |
| Context Relevance | 上下文相关性 |

由 `cmd/eval` 输出 JSON / Markdown 评测报告。所有检索优化必须通过 Evaluation 验证后方可上线。

## 6. RAG 流式生成

RAG 对话通过 WebSocket 将 LLM Token 实时推送至前端：

```text
用户提问 → WS /api/v1/ws/chat
    → 检索 Top-K Chunks
    → 构建 Context
    → LLM 流式生成
    → { type: "token" } × N
    → { type: "done" }
```

索引进度通过 `WS /api/v1/ws/events` 推送 `{ type: "index_progress" }` 事件。
