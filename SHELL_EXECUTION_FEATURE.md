# Shell 命令执行功能实现文档

## 📋 功能概述

基于 `test/shell_test_tools` 中的测试工具，在生产环境中实现了 **Master 向 Worker 发送 Shell 命令并实时流式返回执行结果**的功能。

**实现时间**: 2026-01-29
**版本**: v0.2.0

---

## 🎯 功能特性

### ✨ 核心功能
- ✅ **Ad-hoc Shell 执行**: 通过 Web 界面立即执行 Shell 命令
- ✅ **实时日志输出**: 轮询方式获取命令执行输出（500ms 间隔）
- ✅ **执行状态跟踪**: 实时显示命令执行状态（queued/running/success/failed）
- ✅ **退出码显示**: 显示命令执行完成后的退出码
- ✅ **快速命令模板**: 预置常用命令，一键填入
- ✅ **自动清理**: 临时任务标记为 disabled，避免被调度器重复执行

### 🔧 技术实现
- ✅ 利用现有的任务队列和分发机制
- ✅ 复用 Worker 的流式日志输出功能
- ✅ 集成到生产环境的 REST API
- ✅ 响应式前端界面，适配移动端

---

## 🏗️ 架构设计

### 工作流程

```
┌─────────────┐
│  前端界面   │ 用户输入 Shell 命令
└──────┬──────┘
       │ POST /api/v1/shell/execute
       ▼
┌─────────────┐
│ API Server  │ 1. 验证 Worker 在线状态
│             │ 2. 创建临时 Job 和 Event
└──────┬──────┘ 3. 添加到任务队列
       │
       ▼
┌─────────────┐
│Redis Queue  │ 任务队列
└──────┬──────┘
       │
       ▼
┌─────────────┐
│TaskConsumer │ 获取任务
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Dispatcher  │ 通过 gRPC 分发给 Worker
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Worker    │ 执行 Shell 命令
│  Executor   │ 实时流式输出到 Redis
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Redis     │ task_logs:{event_id}
└──────┬──────┘
       │
       │ GET /api/v1/shell/logs/{event_id} (轮询 500ms)
       ▼
┌─────────────┐
│  前端界面   │ 实时显示日志输出
└─────────────┘
```

---

## 📁 代码实现

### 1️⃣ 后端 API 实现

**文件**: `internal/master/api_server.go`

#### 新增路由

```go
// Shell 命令执行（ad-hoc）
shell := api.Group("/shell")
{
    shell.POST("/execute", s.executeShell)       // 执行 Shell 命令
    shell.GET("/logs/:event_id", s.getShellLogs) // 获取实时日志
}
```

#### executeShell 处理函数

**功能**：
1. 验证请求参数（command, node_id, timeout）
2. 检查 Worker 节点在线状态（60秒内有心跳）
3. 创建临时 Job 记录（enabled=false，避免调度）
4. 创建 Event 记录
5. 保存任务详情到 Redis
6. 添加到任务队列
7. 立即返回 event_id

**关键代码**：
```go
jobID := fmt.Sprintf("shell_adhoc_%d", time.Now().UnixNano())
eventID := fmt.Sprintf("shell_event_%d", time.Now().UnixNano())

job := &models.Job{
    ID:          jobID,
    Name:        "Ad-hoc Shell 命令",
    CronExpr:    "* * * * * *", // 6段式
    Enabled:     false,         // 禁用，避免调度器重复执行
    TaskType:    "shell",
    Command:     req.Command,
    Timeout:     req.Timeout,
}
```

#### getShellLogs 处理函数

**功能**：
1. 从 Redis 获取实时日志：`task_logs:{event_id}`
2. 查询 Event 状态
3. 返回日志、完成状态、退出码

---

### 2️⃣ 前端实现

#### API 客户端

**文件**: `frontend/src/api/index.ts`

```typescript
export interface ShellExecuteRequest {
    command: string
    node_id?: string
    timeout?: number
}

export interface ShellLogsResponse {
    event_id: string
    logs: string
    complete: boolean
    exit_code: number
    status: string
}

export const shellApi = {
    execute: (data: ShellExecuteRequest) =>
        request.post<ShellExecuteResponse>('/shell/execute', data),

    getLogs: (eventId: string) =>
        request.get<ShellLogsResponse>(`/shell/logs/${eventId}`),
}
```

#### Shell 执行页面

**文件**: `frontend/src/views/ShellView.vue`

**核心功能**：
1. 命令输入框（支持回车执行）
2. 快速命令按钮（8个常用命令）
3. 实时日志输出区域（黑色终端风格）
4. 执行状态指示
5. 退出码显示
6. 清空输出按钮

**轮询机制**：
```typescript
const startPolling = (eventId: string) => {
  pollTimer.value = window.setInterval(async () => {
    const result = await shellApi.getLogs(eventId)
    logs.value = result.logs

    if (result.complete) {
      stopPolling()
      // 显示执行结果
    }
  }, 500) // 每 500ms 轮询
}
```

#### 路由配置

**文件**: `frontend/src/router/index.ts`

```typescript
{
    path: 'shell',
    name: 'Shell',
    component: () => import('@/views/ShellView.vue'),
    meta: { title: 'Shell 执行' }
}
```

#### 导航菜单

**文件**: `frontend/src/views/LayoutView.vue`

```vue
<router-link to="/shell" class="nav-item">
  <el-icon class="nav-icon"><Monitor /></el-icon>
  <span v-if="!isCollapse" class="nav-text">Shell 执行</span>
</router-link>
```

---

## 🔌 API 接口文档

### 1. 执行 Shell 命令

**接口**: `POST /api/v1/shell/execute`

**请求体**:
```json
{
  "command": "ls -la",
  "node_id": "optional-node-id",  // 可选：指定节点
  "timeout": 30                    // 可选：超时时间（秒），默认30
}
```

**响应**:
```json
{
  "event_id": "shell_event_1234567890",
  "job_id": "shell_adhoc_1234567890",
  "command": "ls -la",
  "status": "queued",
  "message": "任务已提交，正在执行中",
  "node_id": "node-abc123"
}
```

**错误响应**:
```json
{
  "error": "没有可用的 Worker 节点"
}
```

---

### 2. 获取实时日志

**接口**: `GET /api/v1/shell/logs/{event_id}`

**响应**:
```json
{
  "event_id": "shell_event_1234567890",
  "logs": "total 48\ndrwxr-xr-x 12 linnan staff 384 Jan 29 16:00 .",
  "complete": false,
  "exit_code": 0,
  "status": "running"
}
```

**状态说明**:
- `queued`: 已排队，等待执行
- `running`: 正在执行
- `success`: 执行成功
- `failed`: 执行失败

---

## 🎨 前端界面

### 界面组成

#### 1. 命令输入区
- **输入框**: 大尺寸输入框，支持回车执行
- **执行按钮**: 主要操作按钮（带加载状态）
- **快速命令**: 8个预设命令按钮
  - 当前目录: `pwd`
  - 列出文件: `ls -la`
  - 当前用户: `whoami`
  - 系统时间: `date`
  - 系统信息: `uname -a`
  - 磁盘使用: `df -h`
  - 内存使用: `free -h`
  - CPU 信息: `cat /proc/cpuinfo | grep "model name" | head -1`

#### 2. 输出显示区
- **日志容器**: 黑色终端风格背景（#1e293b）
- **等宽字体**: Monaco/Menlo/Ubuntu Mono
- **实时更新**: 每 500ms 轮询更新
- **状态信息**: 显示 Event ID 和执行状态
- **退出码标签**: 成功（绿色）或失败（红色）

#### 3. UI/UX 优化
- ✅ 遵循 UI/UX Pro Max 设计系统
- ✅ 响应式布局（移动端适配）
- ✅ 加载状态指示
- ✅ 清空输出功能
- ✅ 快速命令模板
- ✅ 执行中禁用输入

---

## 📊 与测试工具的对比

| 特性 | 测试工具 | 生产环境 |
|------|---------|---------|
| **用途** | 开发测试 | 生产功能 |
| **集成方式** | 独立 Web 服务 | 集成到主 API |
| **路由前缀** | `/api/v1` | `/api/v1` (统一) |
| **日志获取** | 轮询 | 轮询 (500ms) |
| **前端框架** | 原生 HTML/JS | Vue 3 + Element Plus |
| **任务命名** | `shell_test_*` | `shell_adhoc_*` |
| **Cron 表达式** | 5段式 `* * * * *` | 6段式 `* * * * * *` |
| **节点选择** | 可选 | 可选（未来扩展） |
| **UI 风格** | 简洁实用 | 现代化设计系统 |

---

## 🚀 使用方法

### 1. 启动服务

```bash
# 启动 Master
go run cmd/master/main.go

# 启动 Worker
go run cmd/worker/main.go

# 启动前端（在另一个终端）
cd frontend
npm run dev
```

### 2. 访问页面

浏览器打开：`http://localhost:5173/shell`

### 3. 执行命令

1. 在输入框中输入命令（或点击快速命令）
2. 点击"执行"按钮或按回车
3. 查看实时日志输出
4. 等待命令完成，查看退出码

---

## 🔧 配置说明

### Worker 心跳配置

Worker 必须在 60 秒内有心跳才会被选中执行命令：

```yaml
# config.yaml
worker:
  heartbeat:
    interval: 10  # 心跳间隔（秒）
```

### 超时配置

默认超时 30 秒，可在请求中自定义：

```json
{
  "command": "sleep 60",
  "timeout": 120  // 120秒超时
}
```

### 并发限制

Worker 的最大并发任务数限制：

```yaml
worker:
  executor:
    max_concurrent_jobs: 10
```

---

## ⚠️ 注意事项

### 安全考虑
1. **权限控制**: 未来需要添加权限验证，避免未授权用户执行命令
2. **命令白名单**: 建议限制可执行的命令类型
3. **审计日志**: 记录所有执行的命令和用户
4. **超时保护**: 防止长时间运行的命令占用资源

### 性能考虑
1. **轮询频率**: 500ms 间隔平衡实时性和负载
2. **日志大小**: 大量输出可能影响性能
3. **并发执行**: 多个用户同时执行命令的负载均衡

### 已知限制
1. **WebSocket**: 当前使用轮询，未来可升级为 WebSocket
2. **节点选择**: 暂时自动选择，未来可手动指定
3. **命令历史**: 暂未保存执行历史
4. **文件上传**: 暂不支持上传脚本文件

---

## 🎯 未来优化方向

### 短期（1-2周）
- [ ] 添加 WebSocket 支持，实现真正的实时推送
- [ ] 命令历史记录功能
- [ ] 支持手动选择执行节点
- [ ] 添加命令执行时间统计

### 中期（1个月）
- [ ] 权限控制和用户认证
- [ ] 命令白名单/黑名单
- [ ] 支持多行命令和脚本文件
- [ ] 支持命令结果导出

### 长期（2-3个月）
- [ ] 终端复用（PTY支持，支持交互式命令）
- [ ] 命令编排（支持多个命令串行/并行执行）
- [ ] 定时任务支持（从 ad-hoc 创建定时任务）
- [ ] 命令执行审计和告警

---

## 📚 相关文档

- **测试工具文档**: [test/SHELL_TEST_README.md](test/SHELL_TEST_README.md)
- **API 设计文档**: [docs/api-design.md](docs/api-design.md) (待创建)
- **前端 UI/UX 优化**: [frontend/UI_UX_OPTIMIZATION.md](frontend/UI_UX_OPTIMIZATION.md)

---

## ✅ 测试检查清单

- [x] 后端 API 实现完成
- [x] 前端页面实现完成
- [x] 路由配置完成
- [x] 导航菜单添加完成
- [x] API 客户端封装完成
- [ ] 功能测试（启动服务验证）
- [ ] 文档完善
- [ ] 代码审查

---

**实现完成**: 2026-01-29
**功能状态**: ✅ 已实现，待测试验证
**下一阶段**: 测试功能并收集反馈
