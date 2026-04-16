# 日志系统改造：gRPC 流传输 → Redis 直写 + Pub/Sub

## Context

当前架构中 Worker 通过 gRPC StreamLogs 将日志传输给 Master，这条链路承担了日志持久化和实时推送双重职责。如果 gRPC 流中断，后续日志全部丢失；`reportToMaster` 是异步 `go` 调用，可能在 Worker 退出前未完成。

改造目标：**Worker 直写 Redis 保证日志完整性，Redis Pub/Sub 负责实时推送，gRPC StreamLogs 整条链路移除。**

## 改造后架构

```
Worker 进程 stdout/stderr
    │
    ├──→ storage.SaveLogChunk()        // Redis APPEND + 本地文件（持久化）
    └──→ storage.PublishLog()           // Redis Pub/Sub 通知（实时）
              │
              ↓
    Redis Pub/Sub channel: "cronicle:logs"
              │
              ↓
    Master LogSubscriber → BroadcastLog() → WebSocket → 前端

任务完成时:
    本地文件 → storage.SetLogComplete() 覆盖 Redis（保证完整）
    reportToMaster() 同步调用（保证 Master 收到结果）
```

## 实施步骤（按依赖顺序）

### Step 1: Redis Pub/Sub 基础设施

**文件**: `internal/storage/log_storage.go`

新增 3 个函数：

1. `PublishLog(ctx, eventID, content)` — 发布到 `"cronicle:logs"` 频道，消息格式 `"eventID\tcontent"`
2. `SubscribeLog(ctx) (<-chan string, func())` — 订阅 `"cronicle:logs"` 频道，返回消息 channel 和取消函数
3. `CloseLogHandle(eventID)` — 关闭指定 event 的缓存文件句柄

依赖 `go-redis/redis/v8`（已在 go.mod 中，该文件未 import）

### Step 2: Worker 改为直写 Redis

**文件**: `internal/worker/executor.go`

`executeShell()` 改造：
- **删除** gRPC logStream 创建代码（约 lines 309-320）
- **删除** logStream.Send() 发送逻辑（约 lines 341-353）
- **删除** logStream.CloseAndRecv() 关闭代码（约 line 380-384）
- **替换** output 处理 goroutine 中的 gRPC 发送为：
  ```go
  storage.SaveLogChunk(ctx, chunk.EventId, content)   // Redis + 文件
  storage.PublishLog(ctx, chunk.EventId, content)       // Pub/Sub 通知
  ```
- 保留 `fullOutput` bytes.Buffer（仍需在 recordTaskResult 中使用）

`recordTaskResult()` 改造：
- **改 `go reportToMaster(...)` 为同步调用** `e.reportToMaster(...)`
- `SetTaskResult()` 之后增加 `SetLogComplete()` + `SetLogExpiration()`

### Step 3: Master 新增日志订阅器

**新建文件**: `internal/master/log_subscriber.go`

- `LogSubscriber` 结构体，包含 `wsServer *WebSocketServer`
- `Start(ctx)` 方法：调用 `storage.SubscribeLog()`，循环接收消息，解析 eventID 和 content，调用 `wsServer.BroadcastLog()`
- 内置断线重连逻辑

**修改文件**: `internal/master/master.go`

- `Master` 结构体新增 `logSubscriber` 和 `logSubCancel` 字段
- `startServices()` 中启动 LogSubscriber（WebSocket 之后）
- `Stop()` 中取消 LogSubscriber

### Step 4: 移除 gRPC StreamLogs

**修改文件**: `internal/master/grpc_server.go`

- **删除** `StreamLogs()` 方法（lines 254-285）
- `ReportTaskResult()` 中的兜底逻辑保留（作为安全网），注释更新

**修改文件**: `pkg/grpc/proto/cronicle.proto`

- 移除 `rpc StreamLogs(stream LogChunk) returns (LogAck)`（line 22）
- 移除 `LogChunk` message（lines 102-108）
- `StreamType` 枚举和 `LogAck` message 暂时保留

**执行**: `make proto` 或 `protoc` 重新生成 pb 文件

### Step 5: 前端修复

**文件**: `frontend/src/views/LogsView.vue`

`handleTaskStatus` 中，当任务完成时（`data.status !== 'running'`）增加重新加载完整日志：
```typescript
if (data.status !== 'running') {
  exitCode.value = data.exit_code
  await loadLogs()  // 新增：补全 WebSocket 断连期间丢失的日志
}
```

（ShellView.vue 已有此逻辑，无需改动）

### Step 6: Worker 关闭时刷新文件

**文件**: `cmd/worker/main.go`

在 shutdown 路径中增加 `storage.CloseAllLogFiles()`

## 关键文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/storage/log_storage.go` | 修改 | 新增 PublishLog、SubscribeLog、CloseLogHandle |
| `internal/worker/executor.go` | 修改 | 移除 gRPC 流，改 Redis 直写，reportToMaster 改同步 |
| `internal/master/log_subscriber.go` | 新建 | Redis Pub/Sub 订阅器 |
| `internal/master/master.go` | 修改 | 集成 LogSubscriber 生命周期 |
| `internal/master/grpc_server.go` | 修改 | 删除 StreamLogs handler |
| `pkg/grpc/proto/cronicle.proto` | 修改 | 移除 StreamLogs RPC 和 LogChunk |
| `frontend/src/views/LogsView.vue` | 修改 | 任务完成时补全日志 |
| `cmd/worker/main.go` | 修改 | 关闭时 flush 文件 |

## 异常场景覆盖

| 场景 | 日志状态 | 恢复方式 |
|------|---------|---------|
| Redis 正常 | Redis 完整，本地完整 | 无需恢复 |
| Redis 执行中断了 N 秒 | Redis 缺 N 秒，本地完整 | 任务完成时 SetLogComplete 覆盖 |
| Redis 执行期间一直断 | Redis 为空，本地完整 | 任务完成时 SetLogComplete 覆盖 |
| Worker 崩溃（Redis 正常） | Redis 有部分，本地有部分 | Worker 重启后补传（后续实现） |
| Worker + Redis 同时挂 | 本地文件可能有部分 | Worker 恢复后补传（后续实现） |
| Pub/Sub 断连 | 实时显示中断 | 自动重连，任务完成时前端 reload 补全 |

## 验证方法

1. `go build ./...` 编译通过
2. 启动 Master + Worker，执行 shell 任务，前端实时显示日志
3. 执行大日志任务（10000 行循环），对比 Redis `task_logs:{eventID}` 与本地文件内容一致
4. 任务执行期间停止 Redis，任务继续执行，日志写入本地文件不中断，Redis 恢复后 Pub/Sub 自动重连
5. ShellView 和 LogsView 任务完成后均显示完整日志
