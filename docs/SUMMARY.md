# Cronicle-Next 项目总结（代码实况）

> 最后更新: 2026-04-11

## 一句话总结

Cronicle-Next 已具备“可开发测试”的核心能力，但仍有少量高优先级缺口，暂不建议直接用于生产。

## 当前能力边界

### 已可用

- 分布式调度链路: Scheduler -> Redis 队列 -> Dispatcher -> Worker 执行
- Shell 任务执行与实时日志推送（gRPC Stream + WebSocket）
- 常用后端 API: Jobs / Events / Nodes / Stats / Shell
- 前端主流程页面: Dashboard、Jobs、JobEdit、Events、Shell

### 未闭环

- JWT 认证与鉴权中间件
- 任务中止（Master API 与 Worker AbortTask 均未完成）
- HTTP / Docker 任务执行器
- 3 个占位页面（JobDetail / Nodes / Logs）

## 适用场景

- 适合: 本地开发、联调、功能验证、演示环境
- 不建议直接用于: 无额外加固的生产环境

## 建议执行顺序

1. 先补齐认证与任务中止
2. 修复 `Job.Env` 前后端数据格式不一致
3. 完成占位页面，形成前端可视化闭环
4. 再扩展 HTTP/Docker 执行器与重试策略
