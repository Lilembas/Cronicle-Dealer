# Cronicle-Next 测试指南

本文档介绍如何使用测试脚本来验证 Cronicle-Next 的功能。

## 测试脚本概览

| 测试脚本 | 说明 | 用途 |
|---------|------|------|
| [integration_test.go](test/integration_test.go) | 集成测试 | 验证基本通讯链路 |
| [worker_startup.go](test/worker_startup.go) | Worker 启动测试 | 测试 Worker 节点启动和运行 |
| [master_worker_e2e.go](test/master_worker_e2e.go) | E2E 测试 | 端到端测试完整任务执行流程 |

---

## 1. Worker 启动测试

### 目的
测试 Worker 节点的启动、注册和心跳功能。

### 运行方式

#### 方法 1: 使用 Shell 脚本
```bash
./test/run_worker_test.sh
```

#### 方法 2: 直接运行 Go 程序
```bash
cd test
go run worker_startup.go -config ../config.yaml -duration 30s
```

#### 方法 3: 先编译后运行
```bash
cd test
go build -o worker_test worker_startup.go
./worker_test -config ../config.yaml -duration 30s
```

### 参数说明
- `-config`: 配置文件路径（默认: `../config.yaml`）
- `-duration`: 测试运行时长（默认: `30s`）

### 测试步骤
1. ✅ 连接 Redis
2. ✅ 连接 Master
3. ✅ 注册 Worker 节点
4. ✅ 启动执行器
5. ✅ 启动心跳机制
6. ✅ 验证心跳状态
7. ✅ 等待接收任务（通过 gRPC）
8. ✅ 清理测试环境

### 注意事项
- Worker 不会主动监听任务队列，而是通过 gRPC 接收 Master 分发的任务
- 测试期间，Master 可以向该 Worker 分发任务
- 测试完成后会自动清理 Worker 注册信息

---

## 2. Master + Worker E2E 测试

### 目的
端到端测试完整的任务调度和执行流程。

### 运行方式

#### 方法 1: 使用 Shell 脚本
```bash
./test/run_e2e_test.sh
```

#### 方法 2: 直接运行 Go 程序
```bash
cd test
go run master_worker_e2e.go -config ../config.yaml -jobs 3 -wait 60s
```

#### 方法 3: 先编译后运行
```bash
cd test
go build -o e2e_test master_worker_e2e.go
./e2e_test -config ../config.yaml -jobs 3 -wait 60s
```

### 参数说明
- `-config`: 配置文件路径（默认: `../config.yaml`）
- `-jobs`: 测试任务数量（默认: `3`）
- `-wait`: 等待任务执行完成的时长（默认: `60s`）

### 测试步骤

#### 阶段 1: 启动 Master 节点
1. 初始化数据库和 Redis
2. 启动 Master 服务（包括 gRPC、API、调度器、任务消费者）

#### 阶段 2: 启动 Worker 节点
1. 连接 Worker 到 Master
2. 注册 Worker 节点
3. 启动执行器和心跳

#### 阶段 3: 创建测试任务
1. 在数据库中创建任务定义
2. 配置任务参数（命令、超时等）

#### 阶段 4: 调度任务执行
1. 创建事件记录
2. 保存任务详情到 Redis
3. 添加任务到队列
4. Master 的 TaskConsumer 从队列获取任务
5. Dispatcher 将任务分发到 Worker
6. Worker 通过 gRPC 接收并执行任务

#### 阶段 5: 监控任务执行
1. 定期检查任务状态
2. 显示执行进度

#### 阶段 6: 查看任务结果
1. 获取任务详情
2. 显示退出码和输出
3. 统计成功/失败数量

#### 阶段 7: 清理测试数据
1. 删除测试任务和事件
2. 清理 Redis 数据
3. 移除 Worker 注册信息

### 测试输出示例

```
🚀 Cronicle-Next Master + Worker E2E 测试
=========================================

📋 阶段 1: 启动 Master 节点
-------------------------
1️⃣ 初始化数据库和 Redis...
✅ 存储初始化成功

2️⃣ 启动 Master 服务...
✅ Master 启动成功

📋 阶段 2: 启动 Worker 节点
-------------------------
3️⃣ 连接 Worker 到 Master...
✅ Worker 注册成功

4️⃣ 启动 Worker 执行器...
5️⃣ 启动心跳机制...
✅ Worker 就绪（通过 gRPC 接收任务）

📋 阶段 3: 创建测试任务
----------------------
✅ 任务创建成功 [job_id=test_job_001]
✅ 任务创建成功 [job_id=test_job_002]
✅ 任务创建成功 [job_id=test_job_003]

📋 阶段 4: 调度任务执行
----------------------
📤 任务已调度 [task_key=test_job_001:test_event_test_job_001]
📤 任务已调度 [task_key=test_job_002:test_event_test_job_002]
📤 任务已调度 [task_key=test_job_003:test_event_test_job_003]

✅ 成功调度 3 个任务

📋 阶段 5: 监控任务执行
----------------------
⏳ 等待任务执行完成 (最多 1m0s)...

🔄 [测试任务 #1] 任务执行中...
   进度: 0/3 完成
✅ [测试任务 #1] 任务完成
🔄 [测试任务 #2] 任务执行中...
   进度: 1/3 完成
✅ [测试任务 #2] 任务完成
🔄 [测试任务 #3] 任务执行中...
   进度: 2/3 完成
✅ [测试任务 #3] 任务完成

📋 阶段 6: 查看任务结果
----------------------

任务: 测试任务 #1
  状态: completed
  退出码: 0
  输出: 执行任务 #1

任务: 测试任务 #2
  状态: completed
  退出码: 0
  输出: 执行任务 #2

任务: 测试任务 #3
  状态: completed
  退出码: 0
  输出: 执行任务 #3

📋 阶段 7: 清理测试数据
----------------------
✅ 测试数据清理完成

========================================
🎉 E2E 测试完成
========================================

📊 测试结果统计:
   总任务数: 3
   成功: 3 ✅
   失败: 0 ❌
   完成率: 100.0%

✅ 验证项:
   ✅ Master 启动和运行
   ✅ Worker 注册和心跳
   ✅ 任务创建和持久化
   ✅ 任务调度和分发
   ✅ 任务队列监听
   ✅ 任务执行和状态更新
   ✅ 结果记录和查询

🎊 所有任务执行成功！
```

---

## 环境要求

### 必需
- **Go 1.22+** - 编译和运行测试程序
- **配置文件** - `config.yaml`（从 `config.example.yaml` 复制）

### 可选
- **Redis** - 如果本地未运行，测试会使用配置文件中的 Redis 地址
- **Docker** - 用于在容器中运行 Redis

---

## 故障排除

### 1. 配置文件不存在
```
❌ 配置文件 ../config.yaml 不存在
```
**解决方法:**
```bash
cp config.example.yaml config.yaml
```

### 2. Redis 连接失败
```
❌ Redis 连接失败: dial tcp: connection refused
```
**解决方法:**
```bash
# 启动本地 Redis
redis-server

# 或使用 Docker
docker-compose -f deployments/docker-compose.yml up -d redis
```

### 3. Master 连接失败
```
❌ Worker 连接 Master 失败
```
**解决方法:**
- 确保 Master 正在运行（对于 worker_startup.go）
- 检查配置文件中的 `master_address` 是否正确
- 确认防火墙未阻止 gRPC 端口（默认: 9090）

### 4. 编译错误
```
undefined: worker.TaskData
```
**解决方法:**
- 确保使用的是重命名后的文件（`worker_startup.go` 而非 `worker_startup_test.go`）

---

## 架构说明

### 任务分发流程

```
┌─────────┐      ┌──────────────┐      ┌─────────┐
│ Master  │ ───> │ Redis Queue  │ <─── │ E2E Test│
│ Task    │      └──────────────┘      │  Creates │
│Consumer │                              │  Tasks   │
└────┬────┘                              └─────────┘
     │
     │ DispatchTask()
     ▼
┌─────────┐      gRPC       ┌─────────┐
│Dispatcher│ ──────────────> │ Worker  │
└─────────┘                  │Executor │
                             └─────────┘
```

### Worker 不监听队列

**重要:** Worker **不直接监听** Redis 任务队列。任务分发流程如下：

1. **Master 的 TaskConsumer** 从 Redis 队列获取任务
2. **Dispatcher** 选择合适的 Worker 节点
3. **通过 gRPC** 将任务推送给 Worker
4. **Worker 的 Executor** 接收并执行任务

这种设计的好处：
- Worker 不需要访问 Redis
- Master 可以控制任务分发策略
- 支持更复杂的负载均衡和调度逻辑

---

## 测试最佳实践

### 1. 开发阶段
使用 **Worker 启动测试** 来验证 Worker 功能：
```bash
# 终端 1: 启动 Master
make master

# 终端 2: 启动 Worker 测试
./test/run_worker_test.sh
```

### 2. 集成测试
使用 **E2E 测试** 来验证完整流程：
```bash
./test/run_e2e_test.sh
```

### 3. CI/CD
在 CI/CD 流程中运行 E2E 测试：
```bash
#!/bin/bash
set -e

# 启动依赖服务
docker-compose -f deployments/docker-compose.yml up -d redis

# 等待服务就绪
sleep 5

# 运行 E2E 测试
./test/run_e2e_test.sh -jobs 5 -wait 120s

# 清理
docker-compose -f deployments/docker-compose.yml down
```

---

## 贡献指南

如果您发现测试问题或有改进建议：

1. **报告 Bug:** 在 GitHub Issues 中报告
2. **提交 PR:** 改进测试逻辑或添加新测试用例
3. **更新文档:** 保持测试文档与代码同步

---

## 相关文档

- [集成测试文档](test/README.md)
- [Getting Started](docs/GETTING_STARTED.md)
- [配置说明](config.example.yaml)
