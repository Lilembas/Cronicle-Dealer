# Cronicle-Next 设计文档

> **版本**: v0.5.0
> **最后更新**: 2026-04-21
> **状态**: Beta

## 📋 目录

- [1. 项目概述](#1-项目概述)
- [2. 架构设计](#2-架构设计)
- [3. 数据模型](#3-数据模型)
- [4. 核心组件](#4-核心组件)
- [5. 通信协议](#5-通信协议)
- [6. 技术栈](#6-技术栈)
- [7. 部署架构](#7-部署架构)
- [8. 安全设计](#8-安全设计)
- [9. 性能优化](#9-性能优化)
- [10. 监控和可观测性](#10-监控和可观测性)

---

## 1. 项目概述

### 1.1 项目简介

Cronicle-Next 是一个高性能、可扩展、可视化的分布式任务调度与执行平台。采用 Manager-Worker 架构，支持水平扩展、高可用性和实时监控。

### 1.2 核心特性

- **分布式架构**: Manager-Worker 模式，支持水平扩展
- **单 Manager 运行模式**: 聚焦功能完整性与稳定性，支持多 Worker 负载均衡
- **智能调度**: 支持 Cron 表达式（6位，秒级精度），灵活的任务调度策略
- **实时监控**: WebSocket 实时推送任务状态和日志流
- **任务执行能力**: 稳定支持 Shell 执行，支持严格模式（Exit on Error）
- **现代化界面**: Vue 3 + TypeScript + Tailwind CSS + PrimeVue (全页面闭环)
- **自定义负载均衡**: 支持 Any, NodeID, Tags, LeastLoaded 等多种分发策略

### 1.3 设计原则

1. **简单性**: 模块化设计，职责单一
2. **可靠性**: 故障自动恢复，数据不丢失
3. **可扩展性**: 支持水平扩展，轻松应对增长
4. **可观测性**: 完善的日志、监控和追踪
5. **安全性**: JWT 认证，加密通信

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        前端层 (Frontend)                      │
│  Vue 3 + TypeScript + Tailwind CSS + PrimeVue                │
│  - 仪表盘、任务管理、节点管理、日志查看器                      │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP/WebSocket
┌─────────────────────────────────────────────────────────────┐
│                      Manager 节点                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ REST API    │  │  Scheduler  │  │ Dispatcher  │          │
│  │  (Gin)      │  │  (Cron)     │  │  (Strategy)  │          │
│  └──────┬──────┘  └──────┬──────┘  └──────▲──────┘          │
└─────────┼────────────────┼────────────────┼─────────────────┘
          │                │                │
          │         ┌──────▼──────┐         │
          └────────►│    Redis    │─────────┘
                    │ (Queue/Lock)│
          ┌────────►└──────┬──────┘
          │                │
┌─────────┴────────────────▼──────────────────────────────────┐
│                      Worker 节点                            │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   gRPC      │  │  Executor   │  │   Monitor   │          │
│  │   Client    │  │  (Shell)    │  │  (Resource) │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 组件交互流程

#### 2.2.1 任务调度流程

```
1. Scheduler (Cron) 触发
   ↓
2. 创建 Event 记录
   ↓
3. 保存任务详情到 Redis
   ↓
4. 添加到 Redis 队列
   ↓
5. TaskConsumer 从队列获取任务
   ↓
6. Dispatcher 选择合适的 Worker
   ↓
7. 通过 gRPC 发送任务到 Worker
   ↓
8. Worker Executor 执行任务
   ↓
9. 实时推送日志 (WebSocket + gRPC Stream)
   ↓
10. 上报执行结果
    ↓
11. 更新 Event 状态
```

#### 2.2.2 日志流传输

```
Worker Executor
    │
    ├── stdout ──┐
    └── stderr ──┤
                 │
            gRPC StreamLogs
                 │
          Manager gRPC Server
                 │
          WebSocket Hub
                 │
         ┌───────┴────────┐
         ▼                ▼
    前端终端         日志存储
     (xterm.js)       (文件系统)
```

### 2.3 数据流

```
┌──────────┐
│   Job    │ (配置)
└─────┬────┘
      │ Cron 触发
      ▼
┌──────────┐
│  Event   │ (执行记录)
└─────┬────┘
      │ 分发
      ▼
┌──────────┐
│  Queue   │ (Redis 队列)
└─────┬────┘
      │ 消费
      ▼
┌──────────┐
│ Executor │ (Worker 执行)
└─────┬────┘
      │ 输出
      ▼
┌──────────┐
│   Logs   │ (日志存储)
└──────────┘
```

---

## 3. 数据模型

### 3.1 核心模型

#### Job（任务配置）

```go
type Job struct {
    ID          string    // 任务ID
    Name        string    // 任务名称
    Description string    // 任务描述
    Category    string    // 任务分类
    
    // 调度配置
    CronExpr    string    // Cron 表达式
    Timezone    string    // 时区
    Enabled     bool      // 是否启用
    
    // 执行配置
    TaskType    string    // 任务类型: shell/http/docker
    Command     string    // 执行命令
    WorkingDir  string    // 工作目录
    Env         string    // 环境变量 (JSON)
    
    // 目标节点
    TargetType  string    // 目标类型: any/node_id/tags
    TargetValue string    // 目标值
    
    // 超时和重试
    Timeout     int       // 超时时间(秒)
    MaxRetries  int       // 最大重试次数
    RetryDelay  int       // 重试延迟(秒)
    
    // 并发控制
    Concurrent  bool      // 是否允许并发
    
    // 通知配置
    NotifyOnSuccess bool
    NotifyOnFailure bool
    NotifyWebhook   string
    
    // 统计信息
    TotalRuns   int64
    SuccessRuns int64
    FailedRuns  int64
}
```

#### Event（执行记录）

```go
type Event struct {
    ID        string    // 执行记录ID
    JobID     string    // 关联任务ID
    JobName   string    // 任务名称
    
    // 执行信息
    NodeID    string    // 执行节点ID
    NodeName  string    // 执行节点名称
    
    // 状态
    Status    string    // pending/running/success/failed/aborted/timeout
    
    // 时间
    ScheduledTime time.Time  // 计划执行时间
    StartTime     *time.Time // 实际开始时间
    EndTime       *time.Time // 结束时间
    Duration      int64      // 执行时长(秒)
    
    // 执行结果
    ExitCode     int    // 退出码
    ErrorMessage string // 错误信息
    
    // 日志
    LogPath      string // 日志文件路径
    LogSize      int64  // 日志大小(字节)
    
    // 资源使用
    CPUPercent   float64 // CPU使用率
    MemoryBytes  int64   // 内存使用量
    
    // 重试信息
    RetryCount   int    // 重试次数
    IsRetry      bool   // 是否为重试
    ParentEventID string // 父执行记录ID
}
```

#### Node（节点信息）

```go
type Node struct {
    ID          string    // 节点ID
    Hostname    string    // 主机名
    IP          string    // IP地址
    Tags        []string  // 节点标签
    Version     string    // 版本号
    
    // 状态
    Status      string    // online/busy/offline
    LastSeen    time.Time // 最后心跳时间
    
    // 资源
    CPUCores    int     // CPU核心数
    CPUUsage    float64 // CPU使用率
    MemoryTotal float64 // 总内存
    MemoryUsage float64 // 内存使用率
    DiskTotal   float64 // 总磁盘
    DiskUsage   float64 // 磁盘使用率
    
    // 统计
    RunningJobs int      // 运行中任务数
    TotalRuns   int64    // 总执行次数
    SuccessRuns int64    // 成功次数
    FailedRuns  int64    // 失败次数
}
```

### 3.2 数据库设计

#### 表结构

**jobs 表**
```sql
CREATE TABLE jobs (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    cron_expr VARCHAR(100) NOT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    enabled BOOLEAN DEFAULT true,
    task_type VARCHAR(20) DEFAULT 'shell',
    command TEXT NOT NULL,
    working_dir VARCHAR(500),
    env TEXT,
    target_type VARCHAR(20) DEFAULT 'any',
    target_value VARCHAR(255),
    timeout INT DEFAULT 3600,
    max_retries INT DEFAULT 0,
    retry_delay INT DEFAULT 60,
    concurrent BOOLEAN DEFAULT false,
    queue_max_size INT DEFAULT 0,
    notify_on_success BOOLEAN DEFAULT false,
    notify_on_failure BOOLEAN DEFAULT true,
    notify_webhook VARCHAR(500),
    created_by VARCHAR(100),
    updated_by VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_run_time TIMESTAMP,
    next_run_time TIMESTAMP,
    total_runs BIGINT DEFAULT 0,
    success_runs BIGINT DEFAULT 0,
    failed_runs BIGINT DEFAULT 0
);

CREATE INDEX idx_jobs_enabled ON jobs(enabled);
CREATE INDEX idx_jobs_next_run ON jobs(next_run_time);
```

**events 表**
```sql
CREATE TABLE events (
    id VARCHAR(64) PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    job_name VARCHAR(255),
    node_id VARCHAR(64),
    node_name VARCHAR(255),
    status VARCHAR(20) DEFAULT 'pending',
    scheduled_time TIMESTAMP NOT NULL,
    start_time TIMESTAMP,
    end_time TIMESTAMP,
    duration BIGINT,
    exit_code INT,
    error_message TEXT,
    log_path VARCHAR(500),
    log_size BIGINT,
    cpu_percent DOUBLE,
    memory_bytes BIGINT,
    retry_count INT DEFAULT 0,
    is_retry BOOLEAN DEFAULT false,
    parent_event_id VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_events_job_id ON events(job_id);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_scheduled ON events(scheduled_time);
```

**nodes 表**
```sql
CREATE TABLE nodes (
    id VARCHAR(64) PRIMARY KEY,
    hostname VARCHAR(255) NOT NULL,
    ip VARCHAR(50),
    tags TEXT,
    version VARCHAR(20),
    status VARCHAR(20) DEFAULT 'online',
    last_seen TIMESTAMP,
    cpu_cores INT,
    cpu_usage DOUBLE,
    memory_total DOUBLE,
    memory_usage DOUBLE,
    disk_total DOUBLE,
    disk_usage DOUBLE,
    running_jobs INT DEFAULT 0,
    total_runs BIGINT DEFAULT 0,
    success_runs BIGINT DEFAULT 0,
    failed_runs BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_nodes_status ON nodes(status);
```

### 3.3 Redis 数据结构

```
# 任务队列
cronicle:queue:tasks → List<EventID>

# 节点注册
cronicle:nodes:{node_id} → Hash<NodeInfo>

# 任务详情（缓存）
cronicle:task:{event_id} → Hash<TaskDetails>

# 心跳超时
cronicle:heartbeat:{node_id} → String<timestamp>

# 日志流
cronicle:logs:{event_id} → List<LogChunk>

# 分布式锁
cronicle:lock:manager → String<manager_id>
cronicle:lock:job:{job_id} → String<event_id>
```

---

## 4. 核心组件

### 4.1 Manager 组件

#### 4.1.1 Scheduler（调度器）

**职责**:
- 解析和管理 Cron 表达式
- 触发任务执行
- 计算下次执行时间

**实现**:
```go
type Scheduler struct {
    cron       *cron.Cron
    jobService *JobService
}
```

**工作流程**:
1. 从数据库加载启用的 Job
2. 解析 Cron 表达式
3. 注册到 Cron 调度器
4. 触发时创建 Event 并推送到队列

#### 4.1.2 Dispatcher（分发器）

**职责**:
- 选择合适的 Worker 节点
- 负载均衡
- 任务分发

**分发策略**:
- **Any**: 随机选择在线节点
- **Node ID**: 指定节点
- **Tags**: 按标签匹配
- **Least Loaded**: 选择负载最低的节点

**实现**:
```go
type Dispatcher struct {
    nodeService *NodeService
}

func (d *Dispatcher) Dispatch(job *Job, event *Event) (*Node, error) {
    // 根据目标类型选择节点
    switch job.TargetType {
    case "any":
        return d.selectAnyNode()
    case "node_id":
        return d.selectNodeByID(job.TargetValue)
    case "tags":
        return d.selectNodeByTags(job.TargetValue)
    }
}
```

#### 4.1.3 TaskConsumer（任务消费者）

**职责**:
- 从 Redis 队列消费任务
- 调用 Dispatcher 分发
- 处理分发失败

**实现**:
```go
type TaskConsumer struct {
    dispatcher *Dispatcher
    queue      *redis.Client
}

func (c *TaskConsumer) Start() {
    for {
        task := c.popTask()
        if task != nil {
            c.dispatcher.Dispatch(task.Job, task.Event)
        }
    }
}
```

#### 4.1.4 gRPC Server

**职责**:
- 处理 Worker 注册
- 接收心跳
- 接收日志流
- 处理任务结果

**接口实现**:
```go
type GRPCServer struct {
    nodeService    *NodeService
    eventService   *EventService
    wsServer       *WebSocketServer
}

func (s *GRPCServer) RegisterNode(ctx, req) (*RegisterNodeResponse, error) {
    // 1. 验证节点信息
    // 2. 生成节点ID
    // 3. 保存到数据库
    // 4. 返回安全令牌
}

func (s *GRPCServer) StreamLogs(stream CronicleService_StreamLogsServer) error {
    // 接收日志流
    for {
        chunk, err := stream.Recv()
        if err != nil {
            break
        }
        // 转发到 WebSocket
        s.wsServer.BroadcastLog(chunk)
    }
    return nil
}
```

#### 4.1.5 WebSocket Server

**职责**:
- 管理前端 WebSocket 连接
- 实时推送日志
- 实时推送任务状态
- 实时推送节点状态

**Hub 模式**:
```go
type WebSocketHub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

func (h *WebSocketHub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
        case client := <-h.unregister:
            delete(h.clients, client)
        case message := <-h.broadcast:
            for client := range h.clients {
                client.send <- message
            }
        }
    }
}
```

### 4.2 Worker 组件

#### 4.2.1 Client（客户端）

**职责**:
- 连接到 Manager
- 节点注册
- 发送心跳
- 资源上报

**实现**:
```go
type Client struct {
    managerConn  *grpc.ClientConn
    nodeID      string
    config      *Config
}

func (c *Client) Start() {
    // 1. 注册节点
    c.register()
    
    // 2. 启动心跳
    go c.heartbeatLoop()
}
```

#### 4.2.2 Executor（执行器）

**职责**:
- 接收任务
- 执行任务
- 捕获输出
- 上报结果

**执行流程**:
```go
type Executor struct {
    nodeID      string
    runningJobs map[string]*JobContext
    maxConcurrent int
}

func (e *Executor) Execute(task *TaskRequest) error {
    // 1. 创建执行上下文
    ctx := NewJobContext(task)
    
    // 2. 启动命令
    cmd := exec.CommandContext(ctx.Context, task.Command)
    
    // 3. 捕获输出
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    
    // 4. 流式传输日志
    go e.streamOutput(stdout, task, STDOUT)
    go e.streamOutput(stderr, task, STDERR)
    
    // 5. 等待完成
    err := cmd.Wait()
    
    // 6. 上报结果
    e.reportResult(task, err)
    
    return nil
}
```

### 4.3 前端组件

#### 4.3.1 页面结构

```
src/views/
├── LoginView.vue          # 登录页
├── LayoutView.vue         # 布局
├── DashboardView.vue      # 仪表盘
├── JobsView.vue           # 任务列表
├── JobDetailView.vue      # 任务详情
├── JobEditView.vue        # 任务编辑
├── EventsView.vue         # 执行记录
├── NodesView.vue          # 节点管理
├── LogsView.vue           # 日志查看
└── ShellView.vue          # Shell 执行
```

#### 4.3.2 状态管理

```typescript
// stores/jobStore.ts
export const useJobStore = defineStore('job', () => {
  const jobs = ref<Job[]>([])
  const loading = ref(false)
  
  async function fetchJobs() {
    loading.value = true
    const { data } = await api.getJobs()
    jobs.value = data
    loading.value = false
  }
  
  return { jobs, loading, fetchJobs }
})
```

#### 4.3.3 WebSocket 集成

```typescript
// composables/useWebSocket.ts
export function useWebSocket() {
  const ws = ref<WebSocket | null>(null)
  const logs = ref<LogEntry[]>([])
  
  function connect() {
    ws.value = new WebSocket('ws://localhost:8081/ws')
    
    ws.value.onmessage = (event) => {
      const data = JSON.parse(event.data)
      if (data.type === 'log') {
        logs.value.push(data)
      }
    }
  }
  
  return { connect, logs }
}
```

---

## 5. 通信协议

### 5.1 gRPC 接口

#### 5.1.1 服务定义

```protobuf
service CronicleService {
    // 节点管理
    rpc RegisterNode(RegisterNodeRequest) returns (RegisterNodeResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    rpc UnregisterNode(UnregisterNodeRequest) returns (UnregisterNodeResponse);
    
    // 任务执行
    rpc SubmitTask(TaskRequest) returns (TaskResponse);
    rpc AbortTask(AbortTaskRequest) returns (AbortTaskResponse);
    
    // 日志流
    rpc StreamLogs(stream LogChunk) returns (LogAck);
    
    // 结果上报
    rpc ReportTaskResult(TaskResult) returns (TaskResultAck);
}
```

#### 5.1.2 消息格式

**任务提交**:
```protobuf
message TaskRequest {
    string job_id = 1;
    string event_id = 2;
    TaskType type = 3;
    string command = 4;
    map<string, string> env = 5;
    int32 timeout = 6;
    string working_dir = 7;
    int64 scheduled_time = 8;
}
```

**日志流**:
```protobuf
message LogChunk {
    string job_id = 1;
    string event_id = 2;
    bytes content = 3;
    int64 timestamp = 4;
    StreamType stream_type = 5;  // STDOUT/STDERR/SYSTEM
}
```

### 5.2 REST API

#### 5.2.1 任务管理

```
GET    /api/v1/jobs           # 获取任务列表
POST   /api/v1/jobs           # 创建任务
GET    /api/v1/jobs/:id       # 获取任务详情
PUT    /api/v1/jobs/:id       # 更新任务
DELETE /api/v1/jobs/:id       # 删除任务
POST   /api/v1/jobs/:id/trigger  # 手动触发
```

#### 5.2.2 执行记录

```
GET /api/v1/events           # 获取执行记录
GET /api/v1/events/:id       # 获取执行详情
GET /api/v1/events/:id/logs  # 获取执行日志
POST /api/v1/events/:id/abort  # 中止执行
```

#### 5.2.3 节点管理

```
GET    /api/v1/nodes          # 获取节点列表
GET    /api/v1/nodes/:id      # 获取节点详情
DELETE /api/v1/nodes/:id      # 删除节点
```

### 5.3 WebSocket 协议

#### 5.3.1 连接

```
WS ws://localhost:8081/ws
```

#### 5.3.2 消息格式

**日志推送**:
```json
{
  "type": "log",
  "event_id": "evt_xxx",
  "content": "Hello, World!",
  "stream_type": "stdout",
  "timestamp": 1706497200
}
```

**状态更新**:
```json
{
  "type": "event_status",
  "event_id": "evt_xxx",
  "status": "running",
  "progress": 50
}
```

**节点状态**:
```json
{
  "type": "node_status",
  "node_id": "node_xxx",
  "status": "online",
  "cpu_usage": 45.2,
  "memory_usage": 62.8
}
```

---

## 6. 技术栈

### 6.1 后端技术

| 组件 | 技术 | 版本 | 说明 |
|------|------|------|------|
| 语言 | Go | 1.24+ | 高性能并发 |
| Web框架 | Gin | 1.10.0 | REST API |
| RPC | gRPC | 1.78.0 | 节点通信 |
| 调度 | robfig/cron | 3.0.1 | Cron调度 |
| 数据库 | SQLite / PostgreSQL (中) | - | 持久化存储 |
| 消息队列 | Redis (必须) | 7+ | 任务分发/状态管理 |
| WebSocket | Melody | 1.4.0 | 实时推送 |
| 日志 | Zap | 1.27.1 | 结构化日志 |
| 配置 | Viper | 1.18.2 | 配置管理 |
| ORM | GORM | 1.25.5 | 数据库操作 |

### 6.2 前端技术

| 组件 | 技术 | 版本 | 说明 |
|------|------|------|------|
| 框架 | Vue | 3.4+ | 渐进式框架 |
| 语言 | TypeScript | 5+ | 类型系统 |
| 构建 | Vite | 5+ | 构建工具 |
| UI库 | PrimeVue | - | 组件库 |
| 样式 | Tailwind CSS | - | CSS框架 |
| 状态 | Pinia | - | 状态管理 |
| 请求 | TanStack Query | - | 数据请求 |
| 终端 | xterm.js | - | 终端模拟 |

### 6.3 部署技术

| 组件 | 技术 | 说明 |
|------|------|------|
| 容器 | Docker | 应用容器化 |
| 编排 | Docker Compose | 本地开发 |
| 部署 | Kubernetes | 生产部署(可选) |

---

## 7. 部署架构

### 7.1 开发环境

```
┌─────────────────────────────────────┐
│         开发机                      │
│  ┌────────────┐  ┌────────────┐    │
│  │  Manager    │  │  Worker    │    │
│  │  :8080     │  │  :9090     │    │
│  └────────────┘  └────────────┘    │
│  ┌────────────┐                    │
│  │  Frontend  │                    │
│  │  :5173     │                    │
│  └────────────┘                    │
└─────────────────────────────────────┘
        │                   │
        ▼                   ▼
┌──────────────┐   ┌──────────────┐
│  PostgreSQL  │   │    Redis     │
│  :5432       │   │    :6379     │
└──────────────┘   └──────────────┘
```

### 7.2 生产环境

```
                    ┌─────────────┐
                    │   LB / Nginx│
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌─────▼──────┐ ┌─────▼──────┐
    │  Manager-1   │ │  Manager-2  │ │  Manager-3  │
    │  (Primary)  │ │  (Standby) │ │  (Standby) │
    └──────┬──────┘ └─────┬──────┘ └─────┬──────┘
           │               │               │
           └───────────────┼───────────────┘
                           │
    ┌──────────────────────┼──────────────────────┐
    │                      │                      │
┌───▼────┐  ┌───▼────┐  ┌─▼────┐  ┌───▼────┐
│Worker-1│  │Worker-2│  │...   │  │Worker-N│
└────────┘  └────────┘  └──────┘  └────────┘
```

### 7.3 容器化部署

**Docker Compose**:
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: cronicle
      POSTGRES_USER: cronicle
      POSTGRES_PASSWORD: password

  redis:
    image: redis:7-alpine

  manager:
    build:
      context: .
      dockerfile: deployments/docker/manager.Dockerfile
    ports:
      - "8080:8080"
      - "9090:9090"
    depends_on:
      - postgres
      - redis

  worker:
    build:
      context: .
      dockerfile: deployments/docker/worker.Dockerfile
    depends_on:
      - manager
```

---

## 8. 安全设计

### 8.1 认证机制

#### JWT 认证
```go
type Claims struct {
    UserID string
    Role   string
    jwt.RegisteredClaims
}

func GenerateToken(user *User) (string, error) {
    claims := Claims{
        UserID: user.ID,
        Role:   user.Role,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}
```

### 8.2 通信安全

#### gRPC TLS
```go
creds, err := credentials.NewServerTLSFromFile(
    "server.crt",
    "server.key",
)
server := grpc.NewServer(grpc.Creds(creds))
```

#### Worker Token
```go
type RegisterNodeResponse struct {
    NodeId       string
    Success      bool
    Message      string
    SecurityToken string  // 用于后续通信验证
}
```

### 8.3 权限控制 (RBAC)

系统采用基于角色的权限控制体系，预设三种核心角色，并在前端实现“权限感知”交互。

#### 角色权限矩阵

| 功能模块 | 权限点 | 只读用户 (viewer) | 普通用户 (user) | 管理员 (admin) |
| :--- | :--- | :---: | :---: | :---: |
| **任务监控** | 查看 Dashboard/列表/详情 | ✅ | ✅ | ✅ |
| **任务操作** | 手动触发/中止任务 | 🚫 (Tooltip提示) | ✅ | ✅ |
| **代码/配置** | 编辑任务/删除任务 | 🚫 (Tooltip提示) | ✅ | ✅ |
| **Shell执行** | 临时执行代码 (Ad-hoc) | 🚫 (不可见) | ✅ | ✅ |
| **基础数据** | 搜索分组/查看节点 | ✅ | ✅ | ✅ |
| **系统管理** | 负载均衡策略配置 | 🚫 (禁用提示) | 🚫 (禁用提示) | ✅ |
| **节点治理** | 节点上线/下线/删除 | 🚫 (禁用提示) | 🚫 (禁用提示) | ✅ |
| **后台配置** | 用户管理/分组管理/日志 | 🚫 (隐藏导航) | 🚫 (隐藏导航) | ✅ |

#### 交互规范
*   **Disabled + Tooltip**: 对于用户可见但无权操作的按钮，采用灰色禁用状态，并悬浮显示“需管理员权限”或“无操作权限”提示。
*   **Defensive Logic**: 前端逻辑层（API调用前）进行二次角色校验，确保安全性。

### 8.4 任务分组同步逻辑 (Category Sync)
*   **后端驱动**: `listCategories` 接口在初始化时确保“默认分组”存在，并自动聚合各 Job 中定义的分类。
*   **前端一致性**: `JobEditView` 与 `CategoriesView` 共享同一套分类 API，取消硬编码，通过 `adminApi.listCategories` 实现全角色下的分类实时同步。

- **Admin**: 完全权限
- **Operator**: 操作权限
- **Viewer**: 只读权限

#### 权限检查
```go
func (h *Handler) CreateJob(c *gin.Context) {
    user := getUser(c)
    if !hasPermission(user, "job.create") {
        c.JSON(403, gin.H{"error": "Forbidden"})
        return
    }
    // ...
}
```

---

## 9. 性能优化

### 9.1 并发控制

#### Goroutine 池
```go
type WorkerPool struct {
    tasks   chan Task
    workers int
}

func NewWorkerPool(workers int) *WorkerPool {
    pool := &WorkerPool{
        tasks:   make(chan Task, 1000),
        workers: workers,
    }
    for i := 0; i < workers; i++ {
        go pool.worker()
    }
    return pool
}
```

### 9.2 缓存策略

#### Redis 缓存
```go
func (s *JobService) GetJob(id string) (*Job, error) {
    // 先查缓存
    cached, err := s.redis.Get("job:" + id)
    if err == nil {
        return json.Unmarshal(cached)
    }
    
    // 查数据库
    job, err := s.db.GetJob(id)
    if err != nil {
        return nil, err
    }
    
    // 写缓存
    s.redis.Set("job:"+id, json.Marshal(job), 5*time.Minute)
    return job, nil
}
```

### 9.3 数据库优化

#### 索引优化
```sql
-- 常用查询索引
CREATE INDEX idx_jobs_enabled ON jobs(enabled);
CREATE INDEX idx_jobs_next_run ON jobs(next_run_time);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_scheduled ON events(scheduled_time);
```

#### 连接池配置
```go
db.DB().SetMaxOpenConns(25)
db.DB().SetMaxIdleConns(10)
db.DB().SetConnMaxLifetime(5 * time.Minute)
```

---

## 10. 监控和可观测性

### 10.1 日志系统

#### 结构化日志
```go
logger.Info("任务执行",
    zap.String("job_id", job.ID),
    zap.String("event_id", event.ID),
    zap.Duration("duration", duration),
)
```

#### 日志级别
- **Debug**: 详细调试信息
- **Info**: 一般信息
- **Warn**: 警告信息
- **Error**: 错误信息
- **Fatal**: 致命错误

### 10.2 指标监控

#### Prometheus 指标
```go
var (
    jobsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cronicle_jobs_total",
            Help: "Total number of jobs",
        },
        []string{"status"},
    )
    
    jobDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "cronicle_job_duration_seconds",
            Help: "Job execution duration",
        },
    )
)
```

### 10.3 健康检查

#### Health Check 端点
```go
func (s *Server) HealthCheck(c *gin.Context) {
    status := map[string]string{
        "status": "ok",
        "timestamp": time.Now().Format(time.RFC3339),
    }
    
    // 检查数据库
    if err := s.db.Ping(); err != nil {
        status["database"] = "error"
    } else {
        status["database"] = "ok"
    }
    
    // 检查 Redis
    if err := s.redis.Ping().Err(); err != nil {
        status["redis"] = "error"
    } else {
        status["redis"] = "ok"
    }
    
    c.JSON(200, status)
}
```

---

## 附录

### A. 配置参考

参见 [config.example.yaml](config.example.yaml)

### B. API 文档

参见 [README.md](README.md) 与 [docs/progress.md](docs/progress.md)

### C. 开发指南

参见 [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)

### D. 部署指南

参见 [deployments/docker-compose.yml](deployments/docker-compose.yml)

---

**文档版本**: v0.5.0
**最后更新**: 2026-04-21
**维护者**: Cronicle-Next Team

---

## 11. 实现状态（2026-04-21）

### 11.1 总体进度

| 模块 | 完成度 | 状态 |
|------|--------|------|
| **后端核心** | 95% | ✅ 调度与分发稳定 |
| **前端界面** | 94% | ✅ 交互优化与权限感知 |
| **认证与权限** | 98% | ✅ RBAC 完整闭环 |
| **总体** | **95%** | 🟢 接近准生产状态 |

### 11.2 当前已完成（新增项）

#### 后端
- ✅ **RBAC 全链路支持**: API 层角色鉴权中间件与 Claims 扩展
- ✅ **Category 增强**: 自动化初始化与同步逻辑
- ✅ **API 路由优化**: 开放 `/categories` 給所有用户查询

#### 前端
- ✅ **全系统权限感知**: 核心操作（触发/中止/编辑/管理）基于角色的禁用提示
- ✅ **UI/UX 体系化升级**: 统一主卡片布局、优化次级导航视觉
- ✅ **动态数据联动**: 任务分组在各页面间完美同步
#### 后端

- ✅ Scheduler -> Redis Queue -> TaskConsumer -> Dispatcher -> Worker 主链路打通
- ✅ TaskConsumer 可控生命周期（支持停止等待）与分发失败重试（指数退避）
- ✅ `abortEvent` / `Dispatcher.AbortTask` / Worker `AbortTask` 任务中止闭环
- ✅ `triggerJob` 手动触发闭环（创建 Event + 入队 + 状态返回）
- ✅ 分发重试参数配置化（`max_dispatch_retries`、`dispatch_retry_base_delay`、`dispatch_retry_max_delay`）
- ✅ JWT 基础登录、刷新、鉴权中间件
- ✅ Worker 真实资源采集（Linux `/proc` + `statfs`）
- ✅ WebSocket 实时日志和状态推送

#### 前端

- ✅ Login（真实登录 API）
- ✅ Dashboard / Jobs / JobEdit / Events / Shell
- ✅ JobDetail / Nodes / Logs 页面已从占位改为可用版本
- ✅ 任务触发按钮与事件高亮展示

### 11.3 当前未完成（按优先级）

#### P0

1. **任务执行可靠性增强**
   - 重试可观测性（指标/告警）

#### P1

2. **HTTP 任务执行器**
3. **Docker 任务执行器**
4. **统一队列治理能力**
   - `queue_max`、队列清理、排队策略

5. **插件协议能力（可参考 Cronicle）**
   - STDIN/STDOUT JSON 协议
   - progress / metrics / chain 等扩展

#### P2

6. **通知体系**
   - job webhook + universal webhook
7. **调度纠偏能力**
   - reset cursor / 补跑策略

### 11.4 技术债务

- ⚠️ 任务重试仍偏”分发层”，执行失败重试链路未完成
- ⚠️ 缺少系统化单元测试/集成测试
- ⚠️ Dispatcher gRPC 客户端连接池并发安全性需进一步加固

---

**文档版本**: v0.3.1
**最后更新**: 2026-04-13
**维护者**: Cronicle-Next Team
