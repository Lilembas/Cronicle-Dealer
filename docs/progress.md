# Cronicle-Next 开发进度报告

> 最后更新: 2026-04-11  
> 统计口径: 基于当前仓库代码的静态检查（非历史计划）

## 当前结论

- 总体状态: 可用于开发测试
- 后端: 核心链路已打通（调度 -> 队列 -> 分发 -> Worker 执行 -> 结果回传）
- 前端: 主流程可用（Dashboard/Jobs/JobEdit/Events/Shell），3 个页面仍为占位
- 关键缺口: JWT 认证、任务中止、HTTP/Docker 执行器、前后端 `env` 字段格式不一致

## 完成度（估算）

| 模块 | 完成度 | 说明 |
|------|--------|------|
| 后端核心调度与分发 | 90% | 可创建/调度/分发并执行 Shell 任务 |
| 实时通信与日志 | 85% | gRPC 日志流 + WebSocket 推送已接通 |
| 前端功能页面 | 80% | 5 个页面可用，3 个页面占位 |
| 认证与权限 | 10% | 仅配置与前端本地 token，未接入真实 JWT |
| 执行器扩展能力 | 35% | 仅 Shell 可用，HTTP/Docker 未实现 |
| 整体 | 80%-85% | 适合开发测试，不建议直接生产 |

## 已实现能力

### 后端

- Job CRUD、Events 查询、Nodes 查询、Stats 统计 API 可用
- Scheduler 支持 Cron（秒级）并写入 Redis 队列
- TaskConsumer 从 Redis 队列消费并调用 Dispatcher 分发
- Dispatcher 按节点状态与负载选择 Worker 并发起 gRPC `SubmitTask`
- Worker 可执行 Shell 命令，支持超时、实时日志流式上报
- Master 可接收任务结果并更新 Event 状态
- WebSocket 支持日志房间订阅与状态广播

### 前端

- 登录页（当前为模拟登录）
- 仪表盘（统计 + 节点状态）
- 任务列表与编辑页（增删改查）
- 执行记录页（查询、筛选、查看日志入口）
- Shell 执行页（提交命令、实时输出）

## 未完成或部分完成

### P0（高优先级）

1. JWT 认证闭环未实现
   - 后端无 `/api/v1/auth/login` 与 JWT 校验中间件
   - 前端仍使用 mock 登录流程
2. 任务中止未实现
   - `POST /api/v1/events/:id/abort` 仅返回“功能开发中”
   - Worker 侧 `AbortTask` 仅保留接口
3. 占位页面
   - `JobDetailView.vue`
   - `NodesView.vue`
   - `LogsView.vue`

### P1（中优先级）

1. `triggerJob` 手动触发流程未完成（仅查询 job 后直接返回）
2. HTTP/Docker 执行器未实现
3. 资源监控为占位数据（CPU/内存等为模拟值）
4. 任务重试逻辑未落地

## 已识别风险

1. `createJob` 依赖 `server_time` context 值并做强制类型断言，存在潜在 panic 风险
2. `Job.Env` 在后端为字符串（JSON），前端编辑页提交为对象，存在兼容性风险
3. 文档与代码曾存在不同步，已在本次更新中修正主要差异

## 下一步建议

1. 先补齐认证和任务中止（上线阻塞项）
2. 修复 `env` 字段前后端协议一致性
3. 完成 `Nodes/Logs/JobDetail` 三个页面，形成可视化闭环
4. 再推进 HTTP/Docker 执行器和重试机制
