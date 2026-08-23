.PHONY: build build-api build-worker build-frontend run-api run-worker run-frontend dev test tidy

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
