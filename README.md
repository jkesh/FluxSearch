# FluxSearch

FluxSearch 是一个面向实时数据场景的 **RAG / Search Retrieval Platform**，支持实时文档索引、Hybrid Search、神经检索、Reranking、检索评测以及周期性模型训练。

目标不是只实现一个“上传文档并对话”的 RAG Demo，而是构建一套可持续迭代的检索系统。

---

## Features

- 实时文档新增、更新、删除
- 文档解析、清洗与 Chunking
- Dense Vector Retrieval
- Sparse / BM25 Retrieval
- Hybrid Search
- RRF（Reciprocal Rank Fusion）
- Cross-Encoder Reranking
- RAG Context Builder
- Answer Citation
- Retriever / Reranker 模型训练
- Hard Negative Mining
- Retrieval Evaluation
- 模型与索引版本管理
- 用户行为反馈闭环
- 支持灰度发布与后续 Kubernetes 部署

---

## Architecture

```text
                         ┌──────────────┐
                         │   Frontend   │
                         │ React + TS   │
                         └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐
                         │  Go Backend  │
                         │ Gin / Chi    │
                         └──────┬───────┘
                                │
            ┌───────────────────┼───────────────────┐
            │                   │                   │
            ▼                   ▼                   ▼
       PostgreSQL             Redis               MinIO
       业务数据/权限        Cache / Lock        原始文件
                                │
                                ▼
                              Kafka
                                │
                                ▼
                         Python Worker
                     Parse / Clean / Chunk
                                │
                                ▼
                         Embedding Service
                      Bi-Encoder / BGE / E5
                                │
                                ▼
                              Milvus
                     ┌──────────┴──────────┐
                     ▼                     ▼
                Dense Search          Sparse / BM25
                     │                     │
                     └──────────┬──────────┘
                                ▼
                            RRF Fusion
                                │
                                ▼
                             Top 100
                                │
                                ▼
                         Cross-Encoder
                            Reranker
                                │
                                ▼
                             Top 5~10
                                │
                                ▼
                         Context Builder
                                │
                                ▼
                               LLM
                                │
                                ▼
                      Answer + Citation
```

---

## Tech Stack

| Module | Technology |
|---|---|
| Frontend | React + TypeScript |
| Backend | Go + Gin / Chi |
| Business Database | PostgreSQL |
| Vector Database | Milvus |
| Cache | Redis |
| Message Queue | Kafka |
| Object Storage | MinIO |
| Data Processing | Python |
| Document Parser | Docling / PyMuPDF |
| Embedding | BGE / E5 / Custom Bi-Encoder |
| Sparse Retrieval | BM25 / Sparse Vector |
| Fusion | RRF |
| Reranker | Cross-Encoder |
| LLM | OpenAI-compatible API / Local LLM |
| Monitoring | Prometheus + Grafana |
| Deployment | Docker Compose / Kubernetes |

---

## Retrieval Pipeline

```text
User Query
    │
    ▼
Query Processing
    │
    ├───────────────────┐
    ▼                   ▼
Dense Retrieval      BM25 / Sparse
Bi-Encoder           Retrieval
    │                   │
    └─────────┬─────────┘
              ▼
           RRF Fusion
              │
              ▼
            Top 100
              │
              ▼
       Cross-Encoder
          Reranker
              │
              ▼
           Top 5~10
              │
              ▼
       Context Builder
              │
              ▼
             LLM
              │
              ▼
      Answer + Citation
```

FluxSearch 使用多阶段检索：

1. Dense Retrieval 负责语义召回。
2. BM25 / Sparse Retrieval 负责关键词与精确匹配。
3. RRF 合并多个 Retriever 的候选结果。
4. Cross-Encoder 对候选结果进行精排。
5. Context Builder 负责去重、Token Budget 和引用构建。
6. LLM 基于最终 Context 生成回答。

---

## Realtime Indexing

数据更新与模型训练相互独立。

```text
Document Create / Update / Delete
              │
              ▼
             Kafka
              │
              ▼
        Ingestion Worker
              │
              ▼
        Parse + Chunk
              │
              ▼
           Embedding
              │
              ▼
            Milvus
              │
              ▼
        Search Available
```

新数据进入系统后直接使用当前 Retriever 生成 Embedding 并写入索引，不需要重新训练模型。

每个 Chunk 建议保存：

```text
chunk_id
document_id
document_version
chunk_hash
content
dense_vector
sparse_vector
page
section
metadata
embedding_model_version
```

文档更新时通过 `chunk_hash` 判断内容是否变化，只重新处理发生变化的 Chunk。

---

## Training Pipeline

```text
Query Logs
Click Logs
Feedback
Dwell Time
    │
    ▼
Training Dataset Builder
    │
    ▼
Hard Negative Mining
    │
    ▼
Bi-Encoder Fine-tuning
    │
    ▼
Cross-Encoder Fine-tuning
    │
    ▼
Offline Evaluation
    │
    ▼
Model Registry
    │
    ▼
Build New Index
    │
    ▼
Canary Deployment
```

推荐训练策略：

| Task | Frequency |
|---|---|
| Document Indexing | Realtime |
| Query / Feedback Logging | Realtime |
| Dataset Generation | Daily |
| Hard Negative Mining | Daily |
| Retrieval Evaluation | Daily |
| Bi-Encoder Fine-tuning | Weekly |
| Reranker Fine-tuning | Weekly |
| Full Reindex | On Model Upgrade |
| Model Deployment | After Evaluation |

---

## Storage Responsibilities

### PostgreSQL

保存业务数据：

```text
users
organizations
documents
collections
permissions
conversations
messages
training_jobs
model_versions
index_versions
```

### Milvus

负责检索索引：

```text
chunk_id
document_id
document_version
content
dense_vector
sparse_vector
metadata
embedding_model_version
```

### MinIO

保存：

```text
PDF
Word
Markdown
HTML
Images
Datasets
Model Artifacts
```

### Redis

负责：

```text
Cache
Rate Limit
Session
Distributed Lock
Short-lived Task State
```

### Kafka

主要事件：

```text
document.created
document.updated
document.deleted

embedding.requested
index.updated

query.logged
feedback.created
```

---

## Model & Index Versioning

模型升级时不直接覆盖生产模型。

```text
Retriever V1 → Index V1
Retriever V2 → Index V2
Retriever V3 → Index V3
```

新模型发布流程：

```text
Train Model
    │
    ▼
Offline Evaluation
    │
    ▼
Build New Index
    │
    ▼
5% Traffic
    │
    ▼
20% Traffic
    │
    ▼
100% Traffic
```

确认新版本稳定后再回收旧模型和旧索引。

---

## Evaluation

### Retrieval Metrics

```text
Recall@1
Recall@5
Recall@10
MRR
nDCG@10
```

### System Metrics

```text
QPS
P50 Latency
P95 Latency
P99 Latency
Embedding Latency
Retrieval Latency
Rerank Latency
Index Build Time
Memory Usage
```

### RAG Metrics

```text
Answer Correctness
Faithfulness
Citation Accuracy
Context Relevance
```

---

## Project Structure

```text
FluxSearch/

├── backend/
│   ├── api/
│   ├── document/
│   ├── search/
│   ├── retrieval/
│   ├── rerank/
│   ├── generation/
│   └── storage/
│
├── worker/
│   ├── parser/
│   ├── chunker/
│   ├── embedding/
│   └── ingestion/
│
├── training/
│   ├── dataset/
│   ├── hard_negative/
│   ├── retriever/
│   └── reranker/
│
├── evaluation/
│   ├── retrieval/
│   └── rag/
│
├── frontend/
│
├── deployments/
│   ├── docker/
│   └── kubernetes/
│
├── docs/
│
└── README.md
```

---

## Roadmap

### V0 — Minimal RAG

- [ ] Document parsing
- [ ] Chunking
- [ ] Embedding
- [ ] Milvus vector search
- [ ] Basic RAG generation

### V1 — Realtime Ingestion

- [ ] Go Backend
- [ ] PostgreSQL
- [ ] MinIO
- [ ] Kafka
- [ ] Realtime document update
- [ ] Incremental indexing

### V2 — Hybrid Retrieval

- [ ] Dense Retrieval
- [ ] BM25 / Sparse Retrieval
- [ ] RRF Fusion
- [ ] Metadata Filter

### V3 — Reranking

- [ ] Cross-Encoder
- [ ] Multi-stage retrieval
- [ ] Context Builder
- [ ] Citation

### V4 — Evaluation

- [ ] Retrieval benchmark dataset
- [ ] Recall@K
- [ ] MRR
- [ ] nDCG
- [ ] Latency benchmark
- [ ] RAG evaluation

### V5 — Neural Retrieval Training

- [ ] Domain Bi-Encoder fine-tuning
- [ ] Hard Negative Mining
- [ ] Cross-Encoder fine-tuning
- [ ] Model Registry

### V6 — Feedback Loop

- [ ] Query logging
- [ ] Click feedback
- [ ] Dwell time
- [ ] Training dataset generation
- [ ] Scheduled training

### V7 — Productionization

- [ ] Model versioning
- [ ] Index versioning
- [ ] Canary deployment
- [ ] Prometheus
- [ ] Grafana
- [ ] Kubernetes

---

## Design Principles

FluxSearch 遵循以下原则：

- 数据实时更新，模型周期训练。
- Retrieval 与 Generation 解耦。
- 业务数据库与检索索引解耦。
- Retriever 优先保证 Recall。
- Reranker 优先提高 Precision。
- 所有模型和索引必须可版本化。
- 所有检索优化必须通过 Evaluation 验证。
- 优先保证可观测性和可替换性，避免绑定单一模型。

---

## Status

FluxSearch is currently under development.

Current stage:

```text
Architecture Design
        ↓
Minimal Retrieval Pipeline
        ↓
Realtime Indexing
        ↓
Hybrid Search
        ↓
Neural Retrieval Training
```

---

## License

TBD
