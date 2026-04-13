# Cronicle-Next 分布式任务调度平台

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat&logo=vue.js)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Status](https://img.shields.io/badge/Status-Beta-yellow.svg)

> 一个高性能、可扩展、可视化的分布式任务调度与执行平台，基于 Go + Vue 3 构建

**🎉 项目状态（2026-04-13）**: 核心链路可用（调度、分发、执行、日志推送、JWT 认证、任务中止、triggerJob 手动触发、前端主页面），可用于开发测试；待完成 HTTP/Docker 执行器、统一队列治理、测试体系。

## 📚 快速链接

- 📖 [快速开始指南](docs/GETTING_STARTED.md) - **推荐首先阅读**
- 🏗️ [设计文档](DESIGN.md) - 架构设计和技术细节
- 🤖 [Claude 开发指南](CLAUDE.md) - AI 辅助开发指导
- 📊 [开发进度报告](docs/progress.md)
- 🎨 [前端完成报告](frontend/COMPLETION.md)
- 📋 [待办事项](TODO.md)

## ✨ 特性

### 核心功能
- 🚀 **高性能**：Go 语言实现，原生并发支持
- 🔄 **分布式架构**：Master-Worker 模式，支持水平扩展
- 🎯 **智能调度**：支持 Cron 表达式（6位，秒级精度），灵活的任务调度
- 📊 **实时监控**：WebSocket 实时推送任务状态和日志
- 📝 **日志流式传输**：实时日志推送，支持长任务
- 🛡️ **高可用**：Redis 队列缓冲，任务不丢失

### 前端界面
- 🎨 **现代化界面**：Vue 3 + TypeScript + Tailwind CSS
- 📱 **响应式设计**：适配各种屏幕尺寸
- ⚡ **实时更新**：WebSocket 自动刷新数据
- 🔧 **任务管理**：完整的 CRUD 操作
- 💻 **Shell 执行**：Ad-hoc 命令执行和实时输出

### 后端服务
- 🌐 **REST API**：15+ API 端点
- 📡 **gRPC 通信**：7 个 RPC 接口
- 🔐 **安全配置**：JWT 和 Worker Token 配置（待实现）
- 📦 **多数据库**：支持 SQLite 和 PostgreSQL
- 🗄️ **Redis 集成**：队列、缓存、分布式锁

## 🏗️ 架构

```
┌─────────────┐
│  Vue 3 前端  │
└──────┬──────┘
       │ HTTP/WebSocket
┌──────▼──────────────────┐
│    Master 节点           │
│  - REST API (Gin)       │
│  - 调度引擎 (Cron)       │
│  - gRPC Server          │
│  - WebSocket (Melody)   │
└──────┬──────────────────┘
       │ gRPC
   ┌───┴───┬───────┐
   ▼       ▼       ▼
┌─────┐ ┌─────┐ ┌─────┐
│ W-1 │ │ W-2 │ │ W-N │ Worker 节点
└─────┘ └─────┘ └─────┘
```

## 🛠️ 技术栈

### 后端
- **语言**：Go 1.25+
- **Web 框架**：Gin
- **RPC**：gRPC
- **调度**：robfig/cron/v3
- **数据库**：SQLite (默认) / PostgreSQL (可选)
- **缓存**：Redis
- **WebSocket**：Melody

### 前端
- **框架**：Vue 3 + TypeScript
- **构建工具**：Vite
- **UI 库**：Element Plus + Tailwind CSS
- **状态管理**：Pinia
- **数据请求**：TanStack Query
- **日志终端**：xterm.js

## 📦 快速开始

### 前置要求

- Go 1.25+
- Node.js 18+
- Redis 7+ (可选，用于分布式缓存和队列)

### 本地开发

1. **克隆项目**
```bash
git clone https://github.com/cronicle/cronicle-next.git
cd cronicle-next
```

2. **安装依赖**
```bash
# 后端依赖
go mod download

# 前端依赖
cd frontend
npm install
```

3. **配置环境**
```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 配置 Redis 连接（可选）
```

4. **运行服务**
```bash
# 启动 Redis（使用 Docker，可选）
docker-compose up -d redis

# 运行 Master
make run-master

# 运行 Worker（新终端）
make run-worker

# 运行前端（新终端）
cd frontend
npm run dev
```

5. **访问界面**

打开浏览器访问：http://localhost:5173

默认账号：`admin` / `admin123`

### Docker 部署

```bash
# 启动 Redis（如果需要）
docker-compose up -d redis

# 或者启动所有服务（包括前端）
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

## 📚 文档

### 核心文档
- [架构设计](DESIGN.md) - 系统架构、数据模型、核心组件
- [Claude 开发指南](CLAUDE.md) - 开发规范、代码组织、最佳实践

### 用户文档
- [快速开始指南](docs/GETTING_STARTED.md) - 安装和运行指南
- [开发进度报告](docs/progress.md) - 当前实现状态与缺口
- [前端完成报告](frontend/COMPLETION.md) - 前端页面与能力说明

### 开发文档
- [项目总结](docs/SUMMARY.md) - 高层状态与下一步
- [测试指南](test/TESTING_GUIDE.md) - 测试相关说明
- [故障排查](test/TROUBLESHOOTING.md) - 常见问题解决

## 🗺️ 路线图

### 已完成 ✅
- [x] 项目初始化和架构设计
- [x] Master 核心功能实现（Scheduler, Dispatcher, TaskConsumer）
- [x] Worker 核心功能实现（Client, Executor）
- [x] WebSocket 实时通信
- [x] Redis 任务队列集成
- [x] 前端主要页面（Dashboard, Jobs, Events, Shell）
- [x] Shell 命令 Ad-hoc 执行
- [x] 日志实时推送
- [x] 认证系统实现（JWT）
- [x] 任务中止功能
- [x] 前端剩余页面（JobDetail, Nodes, Logs）
- [x] triggerJob 手动触发闭环
- [x] 分发重试参数配置化

### 进行中 🚧
- [ ] 分发重试可观测性（Prometheus 指标）
- [ ] 统一队列治理能力

### 计划中 📋
- [ ] HTTP 任务执行器
- [ ] Docker 任务执行器
- [ ] Cron 可视化编辑器增强
- [ ] 用户管理界面
- [ ] 通知系统（Webhook/邮件）
- [ ] 性能优化
- [ ] 单元测试和集成测试

## 🤝 贡献

欢迎贡献代码、报告问题或提出新功能建议！

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 🙏 致谢

灵感来源于 [Cronicle](https://github.com/jhuckaby/Cronicle)。
