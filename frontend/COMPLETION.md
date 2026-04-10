# Cronicle-Next 前端完成报告（代码实况）

> 最后更新: 2026-04-11

## 当前状态

- 前端整体完成度: 约 80%
- 可用页面: `Login`、`Dashboard`、`Jobs`、`JobEdit`、`Events`、`Shell`
- 占位页面: `JobDetail`、`Nodes`、`Logs`

## 已实现能力

### 基础设施

- Vue 3 + TypeScript + Vite 项目结构
- Vue Router 路由与基础守卫
- Pinia 认证状态管理
- Axios 请求封装与统一错误处理
- TanStack Query 数据请求管理

### 功能页面

1. Login
   - 基础登录表单
   - 当前为模拟登录（未对接后端 JWT）
2. Dashboard
   - 统计信息展示
   - 节点状态列表
   - WebSocket 任务/节点状态刷新
3. Jobs
   - 任务列表、创建、编辑、删除、触发入口
4. JobEdit
   - 任务表单与 Cron 构建输入
5. Events
   - 执行记录列表与筛选
   - 查看日志入口
6. Shell
   - Ad-hoc 命令执行
   - WebSocket 实时日志输出

## 当前缺口

1. 认证未闭环
   - 登录仍为 mock
   - 未接入真实 `/api/v1/auth/*`
2. 任务相关缺口
   - 手动触发 API 后端流程未闭环，前端仅有入口
   - 任务中止按钮与交互未实现
3. 页面缺口
   - `JobDetailView.vue` 占位
   - `NodesView.vue` 占位
   - `LogsView.vue` 占位
4. 协议一致性
   - `Job.Env` 字段提交格式与后端模型存在不一致风险

## 下一步建议

1. 优先对接 JWT 登录与鉴权
2. 实现任务中止交互（Events 列表）
3. 完成 3 个占位页面
4. 修复 `env` 字段前后端格式一致性
