# CQADupStack / Unix — CQADupStack Unix 检索评测

基于 [BEIR CQADupStack / Unix](https://huggingface.co/datasets/BeIR/cqadupstack) 的检索评测流水线，对接 FluxSearch `POST /api/v1/search`，输出 **Hit@K / MRR@K / Recall@K**。

## 数据集

| 项 | 值 |
|---|---|
| 语料 | 47,382 篇 Unix & Linux Stack Exchange 帖子 |
| 查询 | 1,072 条（test） |
| 任务 | 重复问题检索（duplicate question retrieval） |
| 指标 | Hit@1/5/10、MRR@10、Recall@10 |

## 前置条件

1. FluxSearch API 已运行（`go run ./cmd/api`）
2. Embedding 已配置（设置页或环境变量）
3. 已应用评测 collection 迁移并创建 Milvus collection

### 1. 应用 PostgreSQL 迁移

```bash
# 本地 PostgreSQL（按你的连接方式调整）
psql "$FLUXSEARCH_DATABASE_URL" -f migrations/006_eval_cqadupstack_unix.sql
```

评测 collection ID：`00000000-0000-0000-0000-0000000000e1`

### 2. 创建 Milvus collection

**必须先停止 API**，再重建 collection（否则 drop 可能失败，或 API 会用错误维度自动创建）：

```powershell
# 停止 go run ./cmd/api 后执行：
$env:FLUXSEARCH_MILVUS_COLLECTION = "fluxsearch_eval_cqadupstack_unix"
go run ./cmd/ensure-milvus -recreate
# 应输出：actual_dim=1024（与 app.settings.json 中 embedding_dim 一致）

# 重新启动 API
go run ./cmd/api
```

验证环境：

```powershell
python eval/cqadupstack_unix/check_setup.py
```

## 快速开始

```powershell
# 安装依赖
pip install -r eval/requirements.txt

# 1. 下载数据集（Hugging Face，约 50MB）
python eval/cqadupstack_unix/download.py

# 2. 导入语料（先 smoke：200 篇）
python eval/cqadupstack_unix/import_corpus.py --limit 200 --workers 2

# 3. 跑评测（50 条 query 快速验证）
python eval/cqadupstack_unix/run_eval.py --query-limit 50 --top-k 10

# 4. 全量导入 + 全量评测
python eval/cqadupstack_unix/import_corpus.py --resume --workers 2
python eval/cqadupstack_unix/run_eval.py --top-k 10
```

或使用 Makefile：

```bash
make eval-download
make eval-import-smoke    # 200 docs
make eval-run-smoke       # 50 queries
make eval-import          # 全量 47k（耗时数小时，取决于 embedding）
make eval-run             # 全量 1072 queries
```

## 输出文件

```text
eval/data/cqadupstack-unix/
  corpus.jsonl      # 语料
  queries.jsonl     # 查询
  qrels.json        # 相关文档标注
  id_map.json       # beir_id <-> flux document_id 映射
  import_state.json # 导入状态

eval/reports/
  cqadupstack-unix-YYYYMMDD-HHMMSS.json
```

## 参数说明

### import_corpus.py

| 参数 | 默认 | 说明 |
|---|---|---|
| `--api-url` | `http://localhost:8080` | API 地址 |
| `--limit` | `0`（全部） | 最多导入文档数 |
| `--workers` | `2` | 并行 worker（注意 embedding API 限流） |
| `--resume` | off | 跳过 id_map 中已有文档 |

### run_eval.py

| 参数 | 默认 | 说明 |
|---|---|---|
| `--top-k` | `10` | 传给 Search API 的 top_k |
| `--ks` | `1,5,10` | 计算的 K 值 |
| `--query-limit` | `0`（全部） | 最多评测 query 数 |

## 注意事项

- **全量 47k 文档**通过 API 逐篇 embedding，可能需要数小时；建议先用 `--limit 200` 验证流程。
- 评测使用**文档级命中**：返回 chunk 所属 `document_id` 映射回 BEIR `corpus-id`。
- 导入与评测使用独立 collection，不影响默认知识库。
- 重复导入同一内容会触发去重跳过；使用 `--resume` 可断点续传。

## 常见问题

**`vector dim 1024 not match collection definition, which has dim of 512`**

Milvus eval collection 维度与 API embedding 不一致。按顺序执行：

1. **停止 API**（Ctrl+C）
2. `$env:FLUXSEARCH_MILVUS_COLLECTION = "fluxsearch_eval_cqadupstack_unix"; go run ./cmd/ensure-milvus -recreate`
3. **重启 API**：`go run ./cmd/api`
4. `python eval/cqadupstack_unix/check_setup.py` 确认 `actual_dim` 与 `embedding_dim` 一致
5. 重新导入：`python eval/cqadupstack_unix/import_corpus.py --purge --limit 200`

**`documents_collection_id_fkey`**

先运行：`python eval/cqadupstack_unix/setup_collection.py`
