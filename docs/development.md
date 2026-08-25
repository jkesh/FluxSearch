# 开发指南

## 环境要求

| 工具 | 版本 |
|------|------|
| Go | 1.25+ |
| Node.js | 20+ |
| npm | 10+ |
| Python | 3.10+（检索评测 `eval/` 需要） |
| Make | 可选 |
| kubectl | 连接 K8s 集群时需要 |

Go 模块代理（国内网络）：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

## 项目结构

```text
FluxSearch/
├── cmd/                        # 可执行入口
│   ├── api/                    # Gin API + WebSocket ✅
│   ├── worker/                 # 导入 + 重索引 Worker ✅
│   ├── monitor/                # 基础设施监控 ✅
│   ├── ensure-milvus/          # Milvus collection 管理 ✅
│   ├── scheduler/              # 定时任务（占位）
│   ├── trainer/                # 训练数据（占位）
│   └── eval/                   # Go 评测 CLI（占位，见 eval/ Python）
│
├── internal/                   # 私有业务逻辑
│   ├── api/
│   │   ├── handler/            # REST Handlers
│   │   └── ws/                 # WebSocket Hub
│   ├── bootstrap/              # 依赖注入、Stores 初始化
│   ├── chat/                   # RAG 对话
│   ├── chunker/                # 递归分块
│   ├── config/                 # 环境变量
│   ├── conversation/           # 对话类型
│   ├── document/               # 文档模型
│   ├── embedding/              # Dense / Hybrid Embedding
│   ├── events/                 # Redis Pub/Sub 事件
│   ├── importqueue/            # Redis 导入 + 重索引队列
│   ├── ingestion/              # 摄取编排
│   ├── llm/                    # LLM 客户端
│   ├── monitor/                # 监控采集
│   ├── parser/                 # PDF / MD / DOCX / TXT
│   ├── reindex/                # 全量 Reindex 协调
│   ├── rerank/                 # HTTP Cross-Encoder
│   ├── retrieval/              # 检索编排
│   ├── settings/               # 应用设置
│   └── storage/                # PG / Redis / MinIO / Milvus
│
├── eval/                       # Python 检索评测流水线 ✅
│   ├── scifact/                # BEIR SciFact
│   ├── cqadupstack_unix/       # CQADupStack Unix
│   ├── reports/                # JSON 评测报告
│   └── requirements.txt
│
├── frontend/                   # React 前端
│   ├── src/
│   │   ├── components/         # ChatComposer / HistorySidebar 等
│   │   ├── pages/              # Chat / Search / Import / Documents / Settings
│   │   ├── hooks/              # useWebSocket
│   │   └── lib/                # API Client
│   └── vite.config.ts
│
├── config/                     # 环境配置
├── deploy/scripts/             # 远程部署脚本
├── docs/                       # 技术文档
├── migrations/                 # SQL 迁移（001 ~ 007）
├── scripts/                    # 辅助脚本（如 FlagEmbedding 服务）
├── go.mod
├── Makefile
└── README.md
```

## 本地开发

### 1. 配置

```bash
mkdir -p config/local
cp config/infra.example.env config/local/infra.env
cp config/app.settings.example.json config/local/app.settings.json
# 编辑填入实际连接信息与 API Key
```

### 2. 连接集群中间件（可选）

本地 API 需要 PostgreSQL / Milvus 等时，通过 port-forward：

```bash
kubectl port-forward -n fluxsearch svc/postgres 5432:5432 &
kubectl port-forward -n fluxsearch svc/redis   6379:6379 &
kubectl port-forward -n fluxsearch svc/milvus  19530:19530 &
kubectl port-forward -n fluxsearch svc/minio   9000:9000 &
```

使用 `config/local/infra.env` 中 `*_LOCAL` 地址连接。

### 3. 启动服务

```bash
# 终端 1：API
make run-api          # 监听 :8080

# 终端 2（可选，生产推荐独立 Worker）
make run-worker

# 终端 3：前端
make run-frontend     # 监听 :5173
```

开发默认 API 内嵌 Worker（`FLUXSEARCH_IMPORT_WORKER_IN_API=true`）。生产可将 Worker 独立部署并关闭 API 内嵌消费。

### 4. 验证

- 前端：http://localhost:5173
- 健康检查：http://localhost:8080/healthz
- WebSocket 对话页测试流式回复
- 导入页观察 `/ws/events` 进度推送

## 构建

```bash
make build              # fluxsearch-api + fluxsearch-worker
make build-api
make build-worker
make build-frontend     # 输出 frontend/dist/
```

## 常用命令

```bash
make run-api
make run-worker
make run-frontend
make test
make tidy

# 检索评测（SciFact 冒烟）
make scifact-download scifact-setup scifact-setup-milvus scifact-import-smoke scifact-run-smoke
```

详见 [eval/README.md](../eval/README.md)。

## 前端开发说明

### Vite Proxy

`frontend/vite.config.ts` 将 `/api` 和 `/healthz` 代理到 `localhost:8080`，WebSocket 同样走代理（`ws: true`）。

### 新增页面

1. 在 `frontend/src/pages/` 创建页面组件
2. 在 `frontend/src/App.tsx` 注册路由
3. REST 调用放 `frontend/src/lib/api.ts`
4. WebSocket 使用 `frontend/src/hooks/useWebSocket.ts`

## 后端开发说明

### 新增 REST 端点

1. 在 `internal/api/handler/` 添加 Handler 方法
2. 在 `internal/api/router.go` 注册路由
3. 更新 [api.md](api.md)

### 新增 WebSocket 事件

1. 在 `internal/events/types.go` 定义事件类型
2. 业务完成后 `events.Bus.Publish`
3. `bootstrap.WireEventBridge` 自动转发至 `/ws/events`
4. 更新 [api.md](api.md)

### 代码规范

- `internal/` 不对外 import
- 基础设施访问统一经过 `internal/storage/`
- Handler 层薄，业务逻辑放 `internal/` 各 domain 包

## 远程部署

使用 `deploy/scripts/remote-run.py` 在 K3s 节点执行脚本：

```bash
python deploy/scripts/remote-run.py <host> <user> <password> <script.sh>
```

带本地 v2rayN 代理隧道（SOCKS5 `:10808`）：

```bash
python deploy/scripts/remote-run.py <host> <user> <password> <script.sh> --tunnel
```
