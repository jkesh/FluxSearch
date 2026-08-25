# BEIR SciFact — 科学事实检索评测

基于 [BEIR SciFact](https://huggingface.co/datasets/BeIR/scifact) 的检索评测流水线，对接 FluxSearch `POST /api/v1/search`，输出 **Hit@K / MRR@K / Recall@K**。

相比 CQADupStack / Unix，SciFact 语料更小、BEIR 基准分数更高，适合快速对比检索效果。

## 数据集

| 项 | 值 |
|---|---|
| 语料 | 5,183 篇科学论文摘要 |
| 查询 | 300 条（test） |
| 任务 | 科学声明验证（claim → supporting/refuting abstracts） |
| BEIR 参考 | nDCG@10 约 0.65–0.68（较好模型） |
| 指标 | Hit@1/5/10、MRR@10、Recall@10 |

## 前置条件

1. FluxSearch API 已运行（`go run ./cmd/api`）
2. Embedding 已配置
3. 已创建 SciFact 评测 collection（与 Unix 评测 **独立**，互不影响）

### 1. 应用 PostgreSQL 迁移

```bash
python eval/scifact/setup_collection.py
# 或: psql "$FLUXSEARCH_DATABASE_URL" -f migrations/007_eval_scifact.sql
```

评测 collection ID：`00000000-0000-0000-0000-0000000000e2`

### 2. 创建 Milvus collection

**先停止 API**，再重建 collection：

```powershell
$env:FLUXSEARCH_MILVUS_COLLECTION = "fluxsearch_eval_scifact"
go run ./cmd/ensure-milvus -recreate

go run ./cmd/api
```

验证：

```powershell
python eval/scifact/check_setup.py
```

## 快速开始

```powershell
pip install -r eval/requirements.txt

# 1. 下载（约 5MB）
python eval/scifact/download.py

# 2. 导入语料（smoke：200 篇）
python eval/scifact/import_corpus.py --limit 200 --workers 2

# 3. 评测（50 条 query）
python eval/scifact/run_eval.py --query-limit 50 --top-k 10

# 4. 全量（5183 篇 + 300 query，约 1–2 小时）
python eval/scifact/import_corpus.py --resume --workers 2
python eval/scifact/run_eval.py --top-k 10
```

Makefile：

```bash
make scifact-download
make scifact-setup scifact-setup-milvus
make scifact-import-smoke scifact-run-smoke
make scifact-import scifact-run
```

## 输出文件

```text
eval/data/scifact/
  corpus.jsonl
  queries.jsonl
  qrels.json
  id_map.json
  import_state.json

eval/reports/
  scifact-YYYYMMDD-HHMMSS.json
```

## 与 Unix 评测对比

| | SciFact | CQADupStack / Unix |
|---|---|---|
| 语料规模 | 5,183 | 47,382 |
| 测试 query | 300 | 1,072 |
| 全量导入耗时 | ~1–2 小时 | 数小时 |
| BEIR nDCG@10（参考） | ~0.65+ | ~0.30–0.37 |
| Collection ID | `...0e2` | `...0e1` |

两个评测使用不同 collection，可同时保留索引结果并交叉对比。
