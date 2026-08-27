# API 规范

Base URL：`http://localhost:8080`（开发）/ `https://<domain>`（生产）

JSON 请求使用 `Content-Type: application/json`；文件上传使用 `multipart/form-data`。

## REST API 总览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/api/v1/system/status` | 基础设施与指标 |
| GET/PUT | `/api/v1/settings` | 应用设置 |
| POST | `/api/v1/search` | 向量检索 |
| GET | `/api/v1/documents` | 文档列表 |
| GET | `/api/v1/documents/:id` | 文档详情 + chunks |
| POST | `/api/v1/documents` | 上传文档（同步） |
| POST | `/api/v1/documents/batch` | 批量上传（同步） |
| DELETE | `/api/v1/documents/:id` | 删除文档 |
| POST | `/api/v1/documents/:id/rechunk` | 重新分块（`?async=true` 异步） |
| POST | `/api/v1/documents/:id/reimport` | 异步重新导入（替换原文并重建索引） |
| POST | `/api/v1/import/jobs` | 创建异步导入任务 |
| GET | `/api/v1/import/jobs` | 导入任务列表 |
| GET | `/api/v1/import/jobs/:id` | 导入任务详情 |
| GET | `/api/v1/conversations` | 对话列表 |
| POST | `/api/v1/conversations` | 创建对话 |
| GET | `/api/v1/conversations/:id` | 对话详情 + 消息 |
| PATCH | `/api/v1/conversations/:id` | 更新标题 / 归档 |
| DELETE | `/api/v1/conversations/:id` | 删除对话 |
| GET | `/api/v1/conversations/:id/messages` | 消息分页 |

## WebSocket

| 路径 | 说明 |
|------|------|
| `WS /api/v1/ws/chat` | RAG 流式对话 |
| `WS /api/v1/ws/events` | 导入进度、文档生命周期与重索引事件 |

**事件类型（`ws/events`）**

| type | 说明 |
|------|------|
| `import_progress` | 异步导入任务进度（含 `job` 对象） |
| `document.created` | 文档导入完成 |
| `document.updated` | 文档更新或重索引完成 |
| `document.deleted` | 文档已删除 |
| `document.reindex` | 重导入/重分块队列状态（`status`: queued / processing / failed） |

开发环境：`ws://localhost:5173/api/v1/ws/chat`（Vite 代理）

---

### 健康检查

```
GET /healthz
```

```json
{ "status": "ok", "service": "fluxsearch-api" }
```

---

### 系统状态

```
GET /api/v1/system/status
```

返回各依赖连接状态与基础指标（文档数、chunk 数、Milvus 实体数等）。

---

### 向量检索

```
POST /api/v1/search
```

**请求**

```json
{ "query": "检索关键词", "top_k": 10 }
```

**响应 200**

```json
{
  "query": "检索关键词",
  "top_k": 10,
  "collection": "fluxsearch_default",
  "mode": "hybrid+rerank",
  "count": 3,
  "results": [
    {
      "chunk_id": "uuid",
      "document_id": "uuid",
      "document_title": "文档标题",
      "content": "匹配片段...",
      "score": 0.92,
      "page": 3,
      "section": "概述"
    }
  ]
}
```

`mode` 取值：`dense` / `sparse_hybrid` / `dense_bm25` / `bm25`，以及带 `+rerank` 后缀的变体。由设置项 `search_mode` 与 Dense/BM25 权重决定。

---

### 文档更新（异步）

#### 重新分块

```
POST /api/v1/documents/:id/rechunk?async=true
```

异步模式返回 `202 Accepted`：

```json
{ "message": "rechunk queued", "document_id": "uuid", "async": true }
```

省略 `async=true` 时同步执行并返回分块结果。

#### 重新导入

```
POST /api/v1/documents/:id/reimport
```

`multipart/form-data`，字段 `file`（必填）、`source_type`（可选）。

```json
{ "message": "reimport queued", "document_id": "uuid", "async": true }
```

进度与完成状态通过 `WS /api/v1/ws/events` 推送 `document.reindex` / `document.updated` 事件。

---

### 文档 CRUD

#### 列表

```
GET /api/v1/documents?collection_id=<uuid>&limit=20&offset=0
```

#### 详情

```
GET /api/v1/documents/:id
```

```json
{
  "document": { "id": "uuid", "title": "...", "status": "indexed", ... },
  "chunks": [ { "id": "uuid", "content": "...", "chunk_index": 0, ... } ]
}
```

#### 上传（同步）

```
POST /api/v1/documents
```

- `multipart/form-data`：`file`（必填）、`title`、`collection_id`
- 或 JSON：`title`、`content`、`source_type`

#### 删除

```
DELETE /api/v1/documents/:id
```

```json
{ "message": "document deleted", "document_id": "uuid" }
```

---

### 异步导入

```
POST /api/v1/import/jobs
```

`multipart/form-data`，字段 `files`（可多文件）。

```json
{
  "job": {
    "id": "uuid",
    "status": "pending",
    "total": 2,
    "done": 0,
    "failed": 0,
    "progress": 0,
    "items": []
  }
}
```

进度通过 `WS /api/v1/ws/events` 推送：

```json
{ "type": "import_progress", "job": { ... } }
```

---

### 对话历史

#### 列表

```
GET /api/v1/conversations?limit=50&offset=0
```

```json
{
  "collection_id": "00000000-0000-0000-0000-000000000001",
  "conversations": [
    {
      "id": "uuid",
      "title": "对话标题",
      "status": "active",
      "message_count": 4,
      "last_preview": "最近一条消息预览...",
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "limit": 50,
  "offset": 0
}
```

#### 详情（含消息）

```
GET /api/v1/conversations/:id
```

```json
{
  "conversation": { "id": "uuid", "title": "...", ... },
  "messages": [
    {
      "id": "uuid",
      "role": "user",
      "content": "问题",
      "sources": [],
      "created_at": "..."
    },
    {
      "id": "uuid",
      "role": "assistant",
      "content": "回答",
      "sources": [
        {
          "chunk_id": "uuid",
          "document_id": "uuid",
          "title": "文档名",
          "content": "引用片段",
          "score": 0.85,
          "page": 1
        }
      ],
      "created_at": "..."
    }
  ]
}
```

---

## WebSocket 对话协议

### 客户端 → 服务端

```json
{
  "type": "chat",
  "content": "用户问题",
  "conversation_id": "uuid"
}
```

`conversation_id` 可选；省略时自动创建新对话。

### 服务端 → 客户端

| type | 说明 | 示例 |
|------|------|------|
| `conversation` | 新对话已创建 | `{ "type": "conversation", "conversation_id": "uuid" }` |
| `sources` | 检索到的引用 | `{ "type": "sources", "sources": [...] }` |
| `token` | 流式文本片段 | `{ "type": "token", "content": "逐" }` |
| `done` | 生成结束 | `{ "type": "done", "done": true, "conversation_id": "uuid", "message_id": "uuid" }` |
| `error` | 错误 | `{ "type": "error", "error": "..." }` |

### 连接保活

- 服务端每 54s 发送 Ping
- 客户端断开时取消进行中的 LLM 流，避免 panic

---

## 错误响应

```json
{ "error": "错误描述" }
```

| HTTP 状态 | 含义 |
|-----------|------|
| 400 | 参数错误 |
| 404 | 资源不存在 |
| 500 | 服务端错误 |
| 503 | 依赖不可用 |

## 版本策略

- 当前前缀：`/api/v1`
- 破坏性变更通过 `/api/v2` 引入
