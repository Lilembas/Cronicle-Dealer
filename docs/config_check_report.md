# Cronicle-Next 配置文件核查报告

本报告针对 `config.yaml` 中的配置项进行了代码层面的全路径追踪，旨在识别哪些配置在当前版本中实际未生效。

## 1. 核心核查结论

经过对 `internal/config`, `internal/manager`, `internal/worker` 以及 `pkg/logger` 等核心模块的审查，以下配置项被确认为**无效**或**待完善**。

---

## 2. 详细失效清单

### 2.1 调度与清理 (Manager)
| 配置项 | 路径 | 原因分析 | 状态 |
| :--- | :--- | :--- | :--- |
| **TickInterval** | `manager.scheduler.tick_interval` | 项目采用 `robfig/cron/v3` 事件驱动调度，而非基于 Tick 的轮询，该参数在 `Scheduler` 逻辑中被忽略。 | ❌ 无效 |
| **LogRetentionDays**| `storage.log_retention_days` | 虽然实现了 `CleanupOldLogs` 清理函数，但没有任何系统触发器（定时任务）来调用它。 | ⚠️ 逻辑缺失 |
| **MaxLogSizeMB** | `storage.max_log_size_mb` | 任务日志存储逻辑仅执行 `Append` 操作，并未引入文件大小检查机制。 | ❌ 无效 |

### 2.2 节点注册与执行 (Worker)
| 配置项 | 路径 | 原因分析 | 状态 |
| :--- | :--- | :--- | :--- |
| **NodeID** | `worker.node.node_id` | `RegisterNodeRequest` 协议中不包含 ID 字段。注册时 ID 由 Manager 根据 Hostname 或随机生成，配置文件指定的值被忽略。 | ❌ 无效 |
| **DefaultTimeout** | `worker.executor.default_timeout` | 在任务下发和执行过程中，未发现引用此值作为 Job 缺省超时的逻辑。 | ❌ 无效 |

### 2.3 安全验证 (Security)
| 配置项 | 路径 | 原因分析 | 状态 |
| :--- | :--- | :--- | :--- |
| **WorkerToken** | `security.worker_token` | Token 仅在注册响应中下发给 Worker，但在心跳、结果上报等后续 RPC 调用中，Manager 未进行任何 Token 匹配校验。 | 🚧 设计占位 |

---

## 3. 建议修复方案

1. **日志清理**：在 `internal/manager/manager.go` 启动时增加一个每日运行一次的高级定时任务，调用 `storage.CleanupOldLogs(cfg.Storage.LogRetentionDays)`。
2. **Token 校验**：为 gRPC 服务端增加一个拦截器（Interceptor），校验所有来自 Worker 的请求元数据（Metadata）中是否包含正确的 `security_token`。
3. **参数注入**：在 `internal/manager/scheduler.go` 触发任务时，如果 `job.Timeout` 为 0，则自动注入 `cfg.Worker.Executor.DefaultTimeout`。
4. **清理配置**：如果确定不再使用基于 Tick 的调度，建议从 `config.go` 结构体和 `config.yaml` 中移除 `tick_interval`。

---
*报告生成时间：2026-04-16*
