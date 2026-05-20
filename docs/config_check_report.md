# Cronicle-Dealer 配置文件核查报告

本报告按当前 `internal/config/config.go` 的结构核查配置文件、环境变量和示例文档是否一致。

## 1. 读取逻辑

配置入口是 `config.Load(configPath)`：

1. 先注册默认值。
2. 读取 `config.yaml`。读取失败不会终止进程，会继续使用默认值和环境变量。
3. 启用 `CRONICLE_` 环境变量前缀。
4. 环境变量中的 `_` 对应配置路径里的 `.`。
5. 将最终配置反序列化到 `Config`。

示例：

| 配置路径 | 环境变量 |
| :--- | :--- |
| `manager.http_port` | `CRONICLE_MANAGER_HTTP_PORT` |
| `manager.database.path` | `CRONICLE_MANAGER_DATABASE_PATH` |
| `manager.security.auth_token` | `CRONICLE_MANAGER_SECURITY_AUTH_TOKEN` |
| `worker.manager_address` | `CRONICLE_WORKER_MANAGER_ADDRESS` |
| `worker.node.tags` | `CRONICLE_WORKER_NODE_TAGS` |
| `redis.host` | `CRONICLE_REDIS_HOST` |
| `logging.log_dir` | `CRONICLE_LOGGING_LOG_DIR` |

## 2. 有效顶层配置

当前代码只读取以下顶层键：

| 顶层键 | 状态 | 说明 |
| :--- | :--- | :--- |
| `manager` | 有效 | Manager HTTP/gRPC、心跳、重试、历史保留、安全和数据库配置 |
| `worker` | 有效 | Worker 连接 Manager、节点标签、执行器、认证和 node_id 持久化配置 |
| `redis` | 有效 | Redis 连接配置 |
| `logging` | 有效 | 日志级别、格式、输出、目录和清理策略 |

以下旧顶层键不会被当前 `Config` 结构读取：

| 旧顶层键 | 替代路径 |
| :--- | :--- |
| `server` | `manager.host`、`manager.http_port`、`manager.grpc_port` |
| `database` | `manager.database` |
| `security` | `manager.security` |
| `storage` | `logging.log_dir`、`logging.log_retention_days`、`logging.max_log_size_mb` |

## 3. 当前可生效配置

`config.example.yaml` 与当前代码结构一致，可作为 `config.yaml` 模板。

需要特别注意：

- Manager 数据库配置必须放在 `manager.database` 下。
- Manager JWT 和 Worker 通信 token 必须放在 `manager.security` 下。
- Worker 的 `worker.auth_token` 必须与 `manager.security.auth_token` 一致。
- Docker 环境变量也必须使用完整路径，例如 `CRONICLE_MANAGER_DATABASE_PATH`，不是 `CRONICLE_DATABASE_PATH`。
- `CRONICLE_WORKER_NODE_TAGS` 支持 JSON 数组格式（如 `["default","docker"]`）或逗号分隔格式（如 `default,docker`）。

## 4. 已核实的历史问题

以下旧报告中的问题已经不再成立：

- gRPC 已通过 interceptor 校验 `manager.security.auth_token`。
- 日志清理和日志大小截断已经在调度/健康检查逻辑中调用。
- Worker 已支持通过本地 `node_id_file` 持久化 node_id 并在注册时复用。

以下配置目前仍需要谨慎使用：

| 配置项 | 状态 | 说明 |
| :--- | :--- | :--- |
| `worker.node.node_id` | 不建议直接使用 | 当前 Worker 注册优先使用 `worker.node_id_file` 中按 hostname 保存的 node_id，配置项默认保留为空 |
| `worker.executor.default_timeout` | 配置存在 | 执行器配置字段已存在，使用时需结合任务执行逻辑确认是否满足期望 |

## 5. Docker 推荐配置

Docker 部署可以不挂载 `config.yaml`，直接使用环境变量。最小配置如下：

Manager：

```bash
CRONICLE_MANAGER_DATABASE_DRIVER=sqlite
CRONICLE_MANAGER_DATABASE_PATH=/app/data/cronicle.db
CRONICLE_MANAGER_SECURITY_JWT_SECRET=change-me-in-production
CRONICLE_MANAGER_SECURITY_AUTH_TOKEN=change-me-in-production
CRONICLE_REDIS_HOST=redis
CRONICLE_REDIS_PORT=6379
CRONICLE_LOGGING_LOG_DIR=/app/logs
```

Worker：

```bash
CRONICLE_WORKER_MANAGER_ADDRESS=manager:9090
CRONICLE_WORKER_AUTH_TOKEN=change-me-in-production
CRONICLE_WORKER_NODE_TAGS=default,docker
CRONICLE_WORKER_NODE_ID_FILE=/app/data/worker_nodes.json
CRONICLE_REDIS_HOST=redis
CRONICLE_REDIS_PORT=6379
CRONICLE_LOGGING_LOG_DIR=/app/logs
```
