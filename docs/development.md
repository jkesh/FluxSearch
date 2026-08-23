# 开发指南

## 环境要求

| 工具 | 版本 |
|------|------|
| Go | 1.25+ |
| Node.js | 20+ |
| npm | 10+ |
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
│   ├── api/                    # Gin API + WebSocket
│   ├── worker/                 # 摄取 Worker
│   ├── scheduler/              # 定时任务
│   ├── trainer/                # 训练数据 / 模型注册
│   └── eval/                   # 离线评测 CLI
│
├── internal/                   # 私有业务逻辑
│   ├── api/
│   │   ├── handler/            # REST Handlers
│   │   └── ws/                 # WebSocket Hub
│   ├── document/               # 文档 CRUD
│   ├── parser/                 # 文档解析
│   ├── chunker/                # 文本分块
│   ├── embedding/              # 向量生成
│   ├── ingestion/              # 摄取编排
│   ├── search/                 # 查询处理
│   ├── retrieval/              # Dense / Sparse / RRF
│   ├── rerank/                 # Cross-Encoder
│   ├── generation/             # RAG + LLM Client
│   ├── training/               # 训练数据
│   ├── evaluation/             # 评测指标
│   └── storage/                # 基础设施适配
│
├── frontend/                   # React 前端
│   ├── src/
│   │   ├── components/
│   │   ├── pages/              # ChatPage / SearchPage
│   │   ├── hooks/              # useWebSocket
│   │   └── lib/                # API Client
│   ├── vite.config.ts
│   └── package.json
│
├── config/                     # 环境配置
│   ├── infra.example.env
│   └── local/                  # 本地私有配置（gitignore）
│
├── deploy/scripts/             # 远程部署脚本
├── docs/                       # 技术文档
├── migrations/                 # SQL 迁移
├── go.mod
├── Makefile
└── README.md
```

## 本地开发

### 1. 配置

```bash
cp config/infra.example.env config/local/infra.env
# 编辑填入实际连接信息
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
# 终端 1：后端
make run-api          # 监听 :8080

# 终端 2：前端
make run-frontend     # 监听 :5173
```

### 4. 验证

- 前端：http://localhost:5173
- 健康检查：http://localhost:8080/healthz
- WebSocket 对话页测试流式回复

## 构建

```bash
# 后端二进制
make build-api
make build-worker

# 前端生产包
make build-frontend   # 输出 frontend/dist/
```

## 常用命令

```bash
make run-api          # 启动 API
make run-frontend     # 启动前端 dev server
make test             # Go 单元测试
make tidy             # go mod tidy
```

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

1. 在 `internal/api/ws/hub.go` 扩展消息处理
2. 定义 JSON 消息结构
3. 更新 [api.md](api.md)

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
