# Cronicle-Next 待办事项

> 基于当前代码实况整理  
> 最后更新: 2026-04-13

---

## 📊 当前进度

```
后端完成度: ~92%
前端完成度: ~88%
总体完成度: ~88%

✅ 已完成: 核心调度链路、JWT基础认证、任务中止、triggerJob闭环、WebSocket、主要页面、Linux资源采集、分发重试配置化
⚠️  部分完成: 分发重试（已实现且参数已配置化）
❌ 未完成: HTTP/Docker执行器、统一队列治理、测试体系
```

> 最后更新: 2026-04-13

---

## 🔴 P0 - 高优先级（近期必须完成）

### 1. 手动触发任务闭环
**状态**: ✅ 已完成 (2026-04-13)  
**文件**: `internal/master/api_server.go`

- [x] 完成 `triggerJob` 全流程
  - [x] 创建 Event 记录
  - [x] 写入 Redis 任务详情
  - [x] 推入队列
  - [x] 返回统一执行状态（queued / running）
- [x] 前端任务详情页展示触发结果与最新状态

---

### 2. 分发重试配置化与可观测性
**状态**: ✅ 已完成 (2026-04-13)  
**文件**: `internal/master/task_consumer.go`, `internal/config/config.go`

- [x] 将重试参数配置化
  - [x] `max_dispatch_retries`
  - [x] `dispatch_retry_base_delay`
  - [x] `dispatch_retry_max_delay`
- [x] 增加重试日志字段（job_id/event_id/retry/delay/error）
- [ ] 增加重试失败计数指标（后续接 Prometheus）

---

## 🟡 P1 - 中优先级（重要功能）

### 3. HTTP 任务执行器
**文件**: `internal/worker/executor.go`

- [ ] 支持 GET/POST/PUT/DELETE
- [ ] 支持 headers/body/timeout
- [ ] 响应状态码判定和输出落库

---

### 4. Docker 任务执行器
**文件**: `internal/worker/executor.go`

- [ ] 设计最小执行参数（image/cmd/env/timeout）
- [ ] 容器生命周期管理（创建/执行/清理）
- [ ] 执行日志采集与退出码映射

---

### 5. 队列治理能力（借鉴 Cronicle）
**文件**: `internal/storage/redis.go`, `internal/master/api_server.go`, `frontend/src/views/JobDetailView.vue`

- [ ] 每任务队列上限（`queue_max`）检查
- [ ] 队列长度查询 API
- [ ] 队列清理 API（按 job flush）
- [ ] 前端展示排队长度与清理入口

---

### 6. 插件协议能力（借鉴 Cronicle）
**文件**: `internal/worker/executor.go`, `internal/models/event.go`

- [ ] 定义插件输入输出 JSON 协议
- [ ] 支持 progress/metrics 上报
- [ ] 支持链式触发（chain/chain_error）基础语义

---

## 🟢 P2 - 低优先级（增强项）

### 7. 通知系统
- [ ] job 级 webhook
- [ ] universal webhook（全局事件流）

### 8. 调度纠偏
- [ ] reset cursor
- [ ] 补跑策略（指定窗口补调度）

### 9. API Key 与权限矩阵
- [ ] API Key 模型与鉴权
- [ ] 按操作粒度授权（run/abort/edit 等）

---

## 🧪 测试与质量保障

### 10. 测试体系建设

- [ ] `triggerJob` 集成测试
- [ ] `abortEvent` 端到端测试（含 running/pending 分支）
- [ ] TaskConsumer 重试流程测试
- [ ] Linux 资源采集单元测试（解析 `/proc`）
- [ ] 前端关键页面冒烟测试（Login/Events/Logs/Nodes）

---

## ⚠️ 已知风险

- [ ] Dispatcher 连接池并发安全性需要进一步加固
- [ ] 执行失败重试尚未打通（当前主要是分发失败重试）
- [ ] 缺少生产级告警与可观测看板
