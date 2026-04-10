# Claude AI 开发指导文档

> **Cronicle-Next** - 分布式任务调度平台
> **版本**: v0.1.0 Beta
> **最后更新**: 2026-04-10

本文档为 Claude AI 提供 Cronicle-Next 项目的开发指导，包括项目架构、开发规范、最佳实践等。

---

## 📋 目录

- [1. 项目概览](#1-项目概览)
- [2. 架构理解](#2-架构理解)
- [3. 开发规范](#3-开发规范)
- [4. 代码组织](#4-代码组织)
- [5. 常见任务](#5-常见任务)
- [6. 调试指南](#6-调试指南)
- [7. 测试指南](#7-测试指南)
- [8. 部署指南](#8-部署指南)
- [9. 故障排查](#9-故障排查)
- [10. 最佳实践](#10-最佳实践)

---

## 1. 项目概览

### 1.1 项目简介

Cronicle-Next 是一个基于 Go + Vue 3 的分布式任务调度平台，采用 Master-Worker 架构。

**核心特性**：
- 🚀 高性能：Go 原生并发
- 🔄 分布式：支持水平扩展
- 📊 实时监控：WebSocket 推送
- 🎯 智能调度：Cron 表达式
- 🛡️ 高可用：Master 故障转移

### 1.2 技术栈

**后端**：
- Go 1.24+
- Gin (REST API)
- gRPC (节点通信)
- GORM (ORM)
- Redis (队列/缓存)
- Zap (日志)

**前端**：
- Vue 3.4+
- TypeScript
- Vite
- Element Plus
- Tailwind CSS

### 1.3 项目状态

- **完成度**: ~85%
- **后端**: 核心功能已完成
- **前端**: 主要页面已完成
- **待完成**: JWT 认证、任务中止、Cron 可视化编辑器

---

## 2. 架构理解

### 2.1 架构模式

**Master-Worker 架构**：
```
前端 (Vue 3)
    ↓ HTTP/WebSocket
Master (Go)
    ↓ gRPC
Worker 节点
```

### 2.2 核心组件

#### Master 组件

**Scheduler（调度器）**：
- 位置：`internal/master/scheduler.go`
- 功能：管理 Cron 任务，触发执行
- 依赖：`robfig/cron/v3`

**Dispatcher（分发器）**：
- 位置：`internal/master/dispatcher.go`
- 功能：选择合适的 Worker 节点
- 策略：any/node_id/tags/least_loaded

**TaskConsumer（任务消费者）**：
- 位置：`internal/master/task_consumer.go`
- 功能：从 Redis 队列消费任务
- 调用：Dispatcher 分发任务

**gRPC Server**：
- 位置：`internal/master/grpc_server.go`
- 功能：处理 Worker 注册、心跳、日志流

**WebSocket Server**：
- 位置：`internal/master/websocket_server.go`
- 功能：实时推送日志和状态

#### Worker 组件

**Client（客户端）**：
- 位置：`internal/worker/client.go`
- 功能：连接 Master、注册、心跳

**Executor（执行器）**：
- 位置：`internal/worker/executor.go`
- 功能：接收任务、执行命令、上报结果

### 2.3 数据流

```
1. Scheduler 触发任务
   ↓
2. 创建 Event 记录
   ↓
3. 推送到 Redis 队列
   ↓
4. TaskConsumer 消费任务
   ↓
5. Dispatcher 选择 Worker
   ↓
6. gRPC 发送到 Worker
   ↓
7. Executor 执行任务
   ↓
8. 实时推送日志 (WebSocket)
   ↓
9. 上报执行结果
   ↓
10. 更新 Event 状态
```

---

## 3. 开发规范

### 3.1 Go 代码规范

#### 命名约定

**文件命名**：
- 使用 `snake_case`
- 示例：`task_consumer.go`, `grpc_server.go`

**包命名**：
- 使用小写单词
- 简洁明了
- 示例：`master`, `worker`, `models`

**函数命名**：
- 导出函数：`PascalCase`
- 内部函数：`camelCase`
- 示例：`NewMaster()`, `startServices()`

**变量命名**：
- `camelCase`
- 缩写全大写
- 示例：`jobID`, `httpPort`, `grpcConn`

#### 错误处理

```go
// ✅ 好的做法
event, err := s.eventService.CreateEvent(job)
if err != nil {
    logger.Error("创建事件失败", zap.Error(err))
    return fmt.Errorf("创建事件失败: %w", err)
}

// ❌ 避免
event, err := s.eventService.CreateEvent(job)
if err != nil {
    panic(err)  // 不要使用 panic
}
```

#### 日志记录

```go
// ✅ 结构化日志
logger.Info("任务开始执行",
    zap.String("job_id", job.ID),
    zap.String("event_id", event.ID),
    zap.Time("scheduled_time", event.ScheduledTime),
)

// ❌ 避免字符串拼接
logger.Info(fmt.Sprintf("任务 %s 开始执行", job.ID))
```

#### 上下文使用

```go
// ✅ 传递 Context
func (s *Service) CreateJob(ctx context.Context, job *Job) error {
    // ...
}

// ✅ 设置超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### 3.2 Vue 代码规范

#### 组件结构

```vue
<script setup lang="ts">
// 1. 导入
import { ref, computed, onMounted } from 'vue'

// 2. Props 定义
interface Props {
  jobId: string
}
const props = defineProps<Props>()

// 3. Emits 定义
const emit = defineEmits<{
  (e: 'update', value: string): void
}>()

// 4. 响应式状态
const loading = ref(false)
const job = ref<Job | null>(null)

// 5. 计算属性
const title = computed(() => job.value?.name || '')

// 6. 方法
async function fetchJob() {
  loading.value = true
  try {
    const { data } = await api.getJob(props.jobId)
    job.value = data
  } finally {
    loading.value = false
  }
}

// 7. 生命周期
onMounted(() => {
  fetchJob()
})
</script>

<template>
  <div v-if="loading">加载中...</div>
  <div v-else-if="job">{{ job.name }}</div>
</template>
```

#### TypeScript 类型

```typescript
// ✅ 定义接口
interface Job {
  id: string
  name: string
  cron_expr: string
  enabled: boolean
}

// ✅ 使用类型
const jobs = ref<Job[]>([])

// ✅ API 返回类型
async function getJobs(): Promise<Job[]> {
  const { data } = await axios.get<Job[]>('/api/v1/jobs')
  return data
}
```

### 3.3 Git 提交规范

#### 提交信息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

#### Type 类型

- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式（不影响功能）
- `refactor`: 重构
- `perf`: 性能优化
- `test`: 测试相关
- `chore`: 构建/工具相关

#### 示例

```bash
feat(master): 添加任务重试逻辑

实现了任务失败时的自动重试机制：
- 支持配置重试次数和延迟
- 使用指数退避策略
- 记录重试历史

Closes #123
```

---

## 4. 代码组织

### 4.1 目录结构

```
cronicle-next/
├── cmd/                    # 主程序
│   ├── master/            # Master 主程序
│   └── worker/            # Worker 主程序
├── internal/              # 内部包
│   ├── master/           # Master 相关
│   ├── worker/           # Worker 相关
│   ├── models/           # 数据模型
│   ├── storage/          # 存储层
│   └── config/           # 配置
├── pkg/                   # 公共包
│   ├── logger/           # 日志
│   ├── utils/            # 工具
│   └── grpc/             # gRPC
├── frontend/              # 前端
│   ├── src/
│   │   ├── views/        # 页面
│   │   ├── components/   # 组件
│   │   ├── api/          # API
│   │   ├── stores/       # 状态
│   │   └── composables/  # 组合式函数
├── deployments/           # 部署
│   └── docker/           # Docker 配置
├── docs/                  # 文档
├── test/                  # 测试
└── config.example.yaml    # 配置模板
```

### 4.2 文件职责

**models/**：数据模型定义
- `job.go` - 任务模型
- `event.go` - 执行记录模型
- `node.go` - 节点模型
- `user.go` - 用户模型

**storage/**：数据访问层
- `database.go` - 数据库初始化
- `redis.go` - Redis 客户端
- `log_storage.go` - 日志存储

**master/**：Master 相关
- `master.go` - Master 管理器
- `scheduler.go` - 调度器
- `dispatcher.go` - 分发器
- `task_consumer.go` - 任务消费者
- `api_server.go` - REST API
- `grpc_server.go` - gRPC 服务
- `websocket_server.go` - WebSocket 服务

**worker/**：Worker 相关
- `client.go` - Master 客户端
- `executor.go` - 任务执行器

---

## 5. 常见任务

### 5.1 添加新的 API 端点

**步骤**：

1. 在 `internal/master/api_server.go` 添加路由：
```go
func (s *APIServer) setupRoutes() {
    s.router.POST("/api/v1/custom", s.handleCustom)
}
```

2. 实现处理函数：
```go
func (s *APIServer) handleCustom(c *gin.Context) {
    var req CustomRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    result, err := s.service.DoSomething(req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, result)
}
```

3. 在前端添加 API 调用：
```typescript
// frontend/src/api/custom.ts
export async function customAPI(params: CustomParams) {
  return axios.post('/api/v1/custom', params)
}
```

### 5.2 添加新的数据模型

**步骤**：

1. 在 `internal/models/` 创建模型文件：
```go
package models

import "time"

type CustomModel struct {
    ID        string    `gorm:"primaryKey" json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

func (CustomModel) TableName() string {
    return "custom_models"
}
```

2. 在 `internal/storage/database.go` 添加迁移：
```go
func InitializeDatabase(driver, dsn string) error {
    db, err := gorm.Open(...)
    // ...
    db.AutoMigrate(&models.CustomModel{})
    return nil
}
```

3. 创建 Service 层：
```go
package services

type CustomService struct {
    db *gorm.DB
}

func (s *CustomService) Create(model *models.CustomModel) error {
    return s.db.Create(model).Error
}
```

### 5.3 添加前端页面

**步骤**：

1. 在 `frontend/src/views/` 创建页面：
```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'

const items = ref([])

async function fetchItems() {
  const { data } = await api.getItems()
  items.value = data
}

onMounted(() => {
  fetchItems()
})
</script>

<template>
  <div>
    <h1>Custom Page</h1>
    <div v-for="item in items" :key="item.id">
      {{ item.name }}
    </div>
  </div>
</template>
```

2. 在 `frontend/src/router/index.ts` 添加路由：
```typescript
{
  path: '/custom',
  name: 'Custom',
  component: () => import('@/views/CustomView.vue')
}
```

3. 在侧边栏添加菜单（如需要）：
```vue
<el-menu-item index="/custom">
  <el-icon><Document /></el-icon>
  <span>Custom</span>
</el-menu-item>
```

### 5.4 修改 gRPC 协议

**步骤**：

1. 编辑 `pkg/grpc/proto/cronicle.proto`：
```protobuf
service CronicleService {
    rpc NewMethod(Request) returns (Response);
}

message Request {
    string data = 1;
}

message Response {
    bool success = 1;
}
```

2. 生成代码：
```bash
make proto
```

3. 在服务端实现：
```go
func (s *GRPCServer) NewMethod(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    return &pb.Response{Success: true}, nil
}
```

4. 在客户端调用：
```go
resp, err := client.NewMethod(ctx, &pb.Request{Data: "test"})
```

---

## 6. 调试指南

### 6.1 本地运行

**启动 Master**：
```bash
# 方式 1: Makefile
make run-master

# 方式 2: 直接运行
go run cmd/master/main.go

# 方式 3: 先编译后运行
go build -o bin/master cmd/master/main.go
./bin/master
```

**启动 Worker**：
```bash
# 新终端
make run-worker
```

**启动前端**：
```bash
cd frontend
npm run dev
```

### 6.2 日志调试

**查看 Master 日志**：
```bash
# 使用 debug 级别
LOG_LEVEL=debug go run cmd/master/main.go
```

**查看 Worker 日志**：
```bash
# Worker 日志会输出到终端
# 可以重定向到文件
go run cmd/worker/main.go > worker.log 2>&1
```

**查看日志文件**：
```bash
# 日志存储在 logs/ 目录
tail -f logs/master.log
tail -f logs/worker.log
tail -f logs/events/evt_xxx.log
```

### 6.3 断点调试

**使用 VS Code**：

创建 `.vscode/launch.json`：
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Master",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/master",
      "env": {
        "LOG_LEVEL": "debug"
      }
    },
    {
      "name": "Debug Worker",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/worker",
      "env": {
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

**使用 Delve**：
```bash
dlv debug cmd/master/main.go
```

### 6.4 API 测试

**使用 curl**：
```bash
# 健康检查
curl http://localhost:8080/health

# 获取任务列表
curl http://localhost:8080/api/v1/jobs

# 创建任务
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"name":"test","cron_expr":"* * * * *","command":"echo hello"}'
```

**使用 Postman**：
导入 API 集合（如果有的话）

---

## 7. 测试指南

### 7.1 运行测试

**集成测试**：
```bash
./test/run_integration_test.sh
```

**E2E 测试**：
```bash
./test/run_e2e_test.sh
```

**Worker 测试**：
```bash
./test/run_worker_test.sh
```

### 7.2 单元测试

**创建测试文件**：
```go
package scheduler

import "testing"

func TestParseCron(t *testing.T) {
    expr := "* * * * *"
    schedule, err := cron.ParseStandard(expr)
    if err != nil {
        t.Fatalf("解析失败: %v", err)
    }
    
    if schedule == nil {
        t.Error("schedule 不应为 nil")
    }
}
```

**运行单元测试**：
```bash
go test ./internal/master/...
go test -v ./internal/worker/...
```

### 7.3 前端测试

**运行测试**：
```bash
cd frontend
npm run test
```

---

## 8. 部署指南

### 8.1 Docker 部署

**构建镜像**：
```bash
# Master
docker build -f deployments/docker/master.Dockerfile -t cronicle-master:latest .

# Worker
docker build -f deployments/docker/worker.Dockerfile -t cronicle-worker:latest .
```

**使用 Docker Compose**：
```bash
docker-compose -f deployments/docker-compose.yml up -d
```

### 8.2 生产部署

**配置检查清单**：
- [ ] 修改 JWT secret
- [ ] 修改 Worker token
- [ ] 配置 PostgreSQL
- [ ] 配置 Redis
- [ ] 设置日志级别
- [ ] 配置 HTTPS
- [ ] 设置资源限制

**环境变量**：
```bash
export JWT_SECRET="your-secret-key"
export WORKER_TOKEN="your-worker-token"
export DATABASE_URL="postgres://user:pass@host:5432/db"
export REDIS_URL="redis://host:6379/0"
```

---

## 9. 故障排查

### 9.1 常见问题

#### Master 无法启动

**症状**：端口被占用
```bash
Error: listen tcp :8080: bind: address already in use
```

**解决**：
```bash
# 查看占用进程
lsof -i :8080

# 杀死进程
kill -9 <PID>

# 或修改配置文件中的端口
```

#### Worker 无法连接 Master

**症状**：连接失败
```bash
❌ Worker 连接 Master 失败
```

**解决**：
1. 检查 Master 是否运行
2. 检查 `config.yaml` 中的 `master_address`
3. 检查防火墙规则
4. 查看网络连通性

#### 任务不执行

**症状**：任务调度但不执行

**排查步骤**：
1. 检查 Job 是否启用
2. 检查 Cron 表达式是否正确
3. 检查是否有在线 Worker
4. 查看 Master 日志
5. 查看 Worker 日志

### 9.2 日志分析

**Master 日志**：
```bash
# 查看 Scheduler 日志
grep "Scheduler" logs/master.log

# 查看分发错误
grep "Dispatch" logs/master.log | grep -i error

# 查看 gRPC 日志
grep "gRPC" logs/master.log
```

**Worker 日志**：
```bash
# 查看执行错误
grep "error" logs/worker.log

# 查看心跳日志
grep "Heartbeat" logs/worker.log
```

### 9.3 性能问题

**高 CPU 使用率**：
```bash
# 查看 CPU 使用
top

# 查看 Goroutine 数量
# 在代码中添加
import "runtime"
runtime.NumGoroutine()
```

**内存泄漏**：
```bash
# 查看内存使用
free -h

# 使用 pprof
import _ "net/http/pprof"
# 访问 http://localhost:8080/debug/pprof/
```

---

## 10. 最佳实践

### 10.1 性能优化

**使用连接池**：
```go
// 数据库连接池
db.DB().SetMaxOpenConns(25)
db.DB().SetMaxIdleConns(10)

// Redis 连接池
redis.NewClient(&redis.Options{
    PoolSize: 10,
})
```

**并发控制**：
```go
// 使用 Worker Pool
type WorkerPool struct {
    sem chan struct{}
}

func NewWorkerPool(size int) *WorkerPool {
    return &WorkerPool{
        sem: make(chan struct{}, size),
    }
}

func (p *WorkerPool) Go(f func()) {
    p.sem <- struct{}{}
    go func() {
        defer func() { <-p.sem }()
        f()
    }()
}
```

### 10.2 安全最佳实践

**验证输入**：
```go
func (s *Service) CreateJob(job *Job) error {
    if job.Name == "" {
        return errors.New("名称不能为空")
    }
    if _, err := cron.ParseStandard(job.CronExpr); err != nil {
        return fmt.Errorf("Cron 表达式无效: %w", err)
    }
    // ...
}
```

**加密敏感数据**：
```go
import "golang.org/x/crypto/bcrypt"

// 加密密码
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// 验证密码
err = bcrypt.CompareHashAndPassword(hash, []byte(password))
```

### 10.3 可维护性

**添加注释**：
```go
// ExecuteTask 执行任务并返回结果
// 它会：
// 1. 验证任务参数
// 2. 选择合适的 Worker
// 3. 发送任务到 Worker
// 4. 等待执行结果
// 5. 处理超时和错误
func (s *Service) ExecuteTask(task *Task) error {
    // ...
}
```

**编写文档**：
- 更新 README.md
- 更新 DESIGN.md
- 添加 API 文档
- 添加使用示例

### 10.4 错误处理

**包装错误**：
```go
if err != nil {
    return fmt.Errorf("创建任务失败: %w", err)
}
```

**定义错误类型**：
```go
var (
    ErrJobNotFound = errors.New("任务不存在")
    ErrInvalidCron = errors.New("Cron 表达式无效")
)

if err != nil {
    if errors.Is(err, ErrJobNotFound) {
        // 处理任务不存在
    }
}
```

---

## 附录

### A. 快速参考

**常用命令**：
```bash
# 运行 Master
make run-master

# 运行 Worker
make run-worker

# 运行前端
cd frontend && npm run dev

# 运行测试
./test/run_e2e_test.sh

# 构建
make build

# Docker
docker-compose up -d
```

**重要端口**：
- Master HTTP: 8080
- Master gRPC: 9090
- WebSocket: 8081
- Frontend: 5173
- PostgreSQL: 5432
- Redis: 6379

### B. 相关文档

- [DESIGN.md](DESIGN.md) - 架构设计文档
- [README.md](README.md) - 项目说明
- [docs/api.md](docs/api.md) - API 文档
- [docs/development.md](docs/development.md) - 开发指南
- [TODO.md](TODO.md) - 待办事项

### C. 获取帮助

- 查看 [DESIGN.md](DESIGN.md) 了解架构
- 查看 [docs/](docs/) 目录下的文档
- 查看测试代码示例
- 查看现有代码实现

---

**文档版本**: v0.1.0
**最后更新**: 2026-04-10
**维护者**: Cronicle-Next Team
