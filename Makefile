.PHONY: build build-api build-worker build-frontend run-api run-worker run-frontend dev test tidy \
	run-flagembedding eval-download eval-import-smoke eval-run-smoke eval-import eval-run eval-setup-milvus eval-reset eval-reset-import

BIN_DIR := bin

build: build-api build-worker

build-api:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/fluxsearch-api ./cmd/api

run-monitor:
	go run ./cmd/monitor

build-worker:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/fluxsearch-worker ./cmd/worker

build-monitor:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/fluxsearch-monitor ./cmd/monitor

build-frontend:
	cd frontend && npm run build

run-api:
	go run ./cmd/api

run-flagembedding:
	python scripts/flagembedding_server.py --port 8091

run-worker:
	go run ./cmd/worker

run-frontend:
	cd frontend && npm run dev

dev:
	@echo "Run in two terminals:"
	@echo "  make run-api"
	@echo "  make run-frontend"

test:
	go test ./...

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

# ── CQADupStack / Unix retrieval eval ─────────────────────
EVAL_PY := python eval/cqadupstack_unix
EVAL_MILVUS_COLLECTION := fluxsearch_eval_cqadupstack_unix

eval-download:
	python eval/cqadupstack_unix/download.py

eval-setup:
	python eval/cqadupstack_unix/setup_collection.py

eval-setup-milvus:
	FLUXSEARCH_MILVUS_COLLECTION=$(EVAL_MILVUS_COLLECTION) go run ./cmd/ensure-milvus -recreate

eval-import-smoke:
	python eval/cqadupstack_unix/import_corpus.py --limit 200 --workers 2

eval-run-smoke:
	python eval/cqadupstack_unix/run_eval.py --query-limit 50 --top-k 10

eval-import:
	python eval/cqadupstack_unix/import_corpus.py --resume --workers 2

eval-reset:
	python eval/cqadupstack_unix/reset_eval.py --workers 16 --yes

eval-reset-import: eval-reset
	python eval/cqadupstack_unix/import_corpus.py --workers 2

eval-run:
	python eval/cqadupstack_unix/run_eval.py --top-k 10
