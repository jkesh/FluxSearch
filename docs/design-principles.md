# 设计原则

FluxSearch 的架构与设计约束，指导技术决策与代码评审。

## 架构原则

### 前后端分离

React 负责 UI 交互与状态展示，Go 负责业务逻辑、检索计算与实时推送。前后端通过 REST + WebSocket 契约通信，可独立开发、部署与扩缩容。

### WebSocket 优先

以下场景走 WebSocket，不走轮询：

- RAG 答案流式输出（Token 级推送）
- 文档索引进度通知
- 长时间任务的实时状态

REST 用于 CRUD、一次性查询和健康检查。

### Go 后端 Monorepo

API、Worker、Scheduler、Trainer、Eval 共享 `internal/` 业务逻辑，按职责编译为独立二进制。避免多语言协作成本，统一工具链与部署方式。

### 推理本地化

Embedding 与 Rerank 优先通过 ONNX 在 Go 进程内推理，避免维护额外 Python 推理服务。仅在 ONNX 不可用时回退到 Remote API。

## 数据原则

### 实时索引，周期训练

- 文档变更 → 实时索引（使用当前 Embedding 模型）
- 模型优化 → 周期导出 ONNX → 评测 → 灰度切换
- 索引与训练解耦，互不阻塞

### 业务库与检索索引分离

- PostgreSQL：业务元数据、权限、会话
- Milvus：向量检索索引
- 两者通过 `document_id` / `chunk_id` 关联，独立扩缩容与备份

### 增量更新

通过 `chunk_hash` 检测内容变化，仅重新处理变更 Chunk，避免全量重建。

## 检索原则

### Retriever 保 Recall，Reranker 提 Precision

- Dense + Sparse 多路召回，RRF 合并，尽可能覆盖相关文档
- Cross-Encoder 精排缩小到 Top 5~10，提升最终精度

### 评测驱动迭代

所有检索策略变更（分块策略、融合参数、模型切换）必须通过 `cmd/eval` 离线评测验证，用数据而非直觉决策。

## 工程原则

### 模型与索引可版本化

```text
Retriever Vn → Index Vn
```

升级不覆盖旧版本，支持灰度流量切换与快速回滚。

### 可观测性

全链路暴露 Prometheus 指标：QPS、延迟分位、各阶段耗时、索引构建时间。关键路径结构化日志。

### 可替换性

`internal/` 各模块通过接口抽象，避免绑定单一供应商：

- Embedding：ONNX ↔ Remote API
- LLM：OpenAI-compatible 任意后端
- Sparse：Bleve ↔ Milvus Sparse

### 配置外置

连接信息与密钥存放在 `config/local/`（本地）和 K8s Secret（集群），不硬编码在源码中。

## 反模式（避免）

| 反模式 | 原因 |
|--------|------|
| 在 Handler 中直接操作 Milvus / PG | 绕过 storage 层，难以测试与替换 |
| 前端轮询 RAG 结果 | 延迟高、浪费资源，应使用 WebSocket |
| 模型升级直接覆盖生产索引 | 无法回滚，应灰度切换 |
| 跳过评测直接调参上线 | 无法量化收益，可能引入退化 |
| 单机部署全部中间件无资源限制 | 易 OOM，应设 ResourceQuota |
