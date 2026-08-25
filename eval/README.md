# FluxSearch 检索评测

基于 BEIR 数据集的检索评测流水线，对接 `POST /api/v1/search`，输出 Hit@K / MRR@K / Recall@K。

## 可用数据集

| 数据集 | 语料 | 测试 query | 说明 |
|--------|------|------------|------|
| [SciFact](scifact/README.md) | 5,183 | 300 | **推荐**：规模小、跑得快、BEIR 分数较高，便于对比 |
| [CQADupStack / Unix](cqadupstack_unix/README.md) | 47,382 | 1,072 | Unix/Linux 重复问题检索，难度大、耗时长 |

## SciFact 快速开始（推荐）

```powershell
pip install -r eval/requirements.txt

python eval/scifact/download.py
python eval/scifact/setup_collection.py
$env:FLUXSEARCH_MILVUS_COLLECTION = "fluxsearch_eval_scifact"
go run ./cmd/ensure-milvus -recreate

python eval/scifact/import_corpus.py --limit 200 --workers 2
python eval/scifact/run_eval.py --query-limit 50 --top-k 10
```

Makefile：`make scifact-download scifact-setup scifact-setup-milvus scifact-import-smoke scifact-run-smoke`

## CQADupStack / Unix

详见 [cqadupstack_unix/README.md](cqadupstack_unix/README.md)。

Makefile：`make eval-download eval-setup eval-setup-milvus eval-import-smoke eval-run-smoke`
