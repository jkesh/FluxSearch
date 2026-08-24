# FluxSearch 检索评测

## CQADupStack / Unix（技术文档代理基准）

面向 Unix/Linux 技术问答的 BEIR 检索评测，详见 [cqadupstack_unix/README.md](cqadupstack_unix/README.md)。

```powershell
pip install -r eval/requirements.txt

python eval/cqadupstack_unix/download.py
python eval/cqadupstack_unix/setup_collection.py
$env:FLUXSEARCH_MILVUS_COLLECTION = "fluxsearch_eval_cqadupstack_unix"
go run ./cmd/ensure-milvus

python eval/cqadupstack_unix/import_corpus.py --limit 200 --workers 2
python eval/cqadupstack_unix/run_eval.py --query-limit 50 --top-k 10
```

Makefile 快捷命令：`make eval-download eval-setup eval-setup-milvus eval-import-smoke eval-run-smoke`
