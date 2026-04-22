# Cronicle-Next 分布式任务调度平台

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat&logo=vue.js)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Status](https://img.shields.io/badge/Status-Beta-yellow.svg)

> 一个高性能、可扩展、可视化的分布式任务调度与执行平台，基于 Go + Vue 3 构建


## ✨ 特性

### 核心功能
- 🚀 **高性能**：Go 语言实现，原生并发支持
- 🔄 **分布式架构**：Manager-Worker 模式，支持水平扩展
- 🎯 **智能调度**：支持 Cron 表达式（6位，秒级精度），灵活的任务调度
- 📊 **实时监控**：WebSocket 实时推送任务状态和日志
- 📝 **日志流式传输**：实时日志推送，支持长任务
- ⚖️ **自定义负载均衡**：允许用户自定义负载均衡策略函数（根据节点cpu、内存等负载）


## 🏗️ 架构

```
      ┌──────────────┐
      │  Vue 3 前端   │
      └──────┬───────┘
             │ HTTP/WS
      ┌──────▼──────┐
      │   Manager   │ (Scheduler/API)
      └──────┬──────┘
             │ 
      ┌──────▼──────┐
      │    Redis    │ (Task Queue/State/HA)
      └──────┬──────┘
             │ gRPC Dispatch
   ┌─────────┴─────────┐
   ▼         ▼         ▼
┌─────┐   ┌─────┐   ┌─────┐
│ W-1 │   │ W-2 │   │ W-N │ Worker 节点 (Executor)
└─────┘   └─────┘   └─────┘
```

## 🛠️ 技术栈

### 后端
- **语言**：Go 1.25+
- **Web 框架**：Gin
- **RPC**：gRPC
- **调度**：robfig/cron/v3
- **数据库**：SQLite (默认) / PostgreSQL (待完善)
- **核心组件**：Redis
- **WebSocket**：Melody

### 前端
- **框架**：Vue 3 + TypeScript
- **构建工具**：Vite
- **UI**: PrimeVue + Tailwind CSS
- **状态管理**：Pinia
- **数据请求**：TanStack Query
- **日志终端**：xterm.js

## 📦 快速开始

### 前置要求

- Go 1.25+
- Node.js 18+
- Redis 7+

### 本地开发

1. **克隆项目**
```bash
git clone https://github.com/Lilembas/cronicle-next.git
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
# 编辑 config.yaml 配置 Redis 连接、manager、worker通讯地址和端口
```

4. **运行服务**
```bash
# 运行 Manager
make run-manager
或 go run cmd/manager/main.go

# 运行 Worker（新终端）
make run-worker
或 go run cmd/worker/main.go

# 运行前端（新终端）
cd frontend
npm run dev -- --host=0.0.0.0
```

5. **访问界面**

打开浏览器访问：http://manager_ip:5173

默认账号：`admin` / `admin123`


## 📚 文档

### 核心文档
- 🏗️ [架构设计](DESIGN.md) - 系统架构、数据模型、核心组件
- 🤖 [Claude 开发指南](CLAUDE.md) - AI 辅助开发规范

### 用户文档
- [快速开始指南](docs/GETTING_STARTED.md) - 安装和运行指南
- [开发进度报告](docs/progress.md) - 当前实现状态与缺口
- [前端完成报告](frontend/COMPLETION.md) - 前端页面与能力说明

### 开发文档
- [项目总结](docs/SUMMARY.md) - 高层状态与下一步

## 🗺️ 路线图

### 已完成 ✅
- [x] Manager 核心功能实现（Scheduler, Dispatcher, TaskConsumer）
- [x] Worker 核心功能实现（Client, Executor）
- [x] WebSocket 实时通信
- [x] Redis 任务队列集成
- [x] Shell 命令 Ad-hoc 执行
- [x] 日志实时推送
- [x] 自定义负载均衡策略


### 计划中 📋
- [ ] HTTP 任务执行器
- [ ] Dockerfile 与 Docker-compose 部署方案完善
- [ ] 通知系统（Webhook/邮件）
- [ ] 性能优化

## 🤝 贡献

欢迎贡献代码、报告问题或提出新功能建议！

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 🙏 致谢

灵感来源于 [Cronicle](https://github.com/jhuckaby/Cronicle)。
