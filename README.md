# Cronicle-Next 分布式任务调度平台

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat&logo=vue.js)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Status](https://img.shields.io/badge/Status-Beta-yellow.svg)

> 一个高性能、可扩展、可视化的分布式任务调度与执行平台，基于 Go + Vue 3 构建

**🎉 项目状态**: 核心功能已完成（75%），可用于开发测试

## 📚 快速链接

- 📖 [快速开始指南](docs/GETTING_STARTED.md) - **推荐首先阅读**
- 📊 [开发进度报告](docs/progress.md)
- 🎨 [前端完成报告](frontend/COMPLETION.md)
- 📋 [任务清单](/.gemini/antigravity/brain/cb448d4b-9346-4b7a-8602-3efabc31c29a/task.md)

## ✨ 特性

- 🚀 **高性能**：Go 语言实现，原生并发支持
- 🔄 **分布式架构**：Master-Worker 模式，支持水平扩展
- 🎯 **智能调度**：支持 Cron 表达式，灵活的任务调度
- 📊 **实时监控**：实时资源监控和任务执行状态
- 📝 **日志流式传输**：WebSocket 实时日志推送
- 🛡️ **高可用**：Master 自动故障转移，保证服务稳定
- 🎨 **现代化界面**：Vue 3 + TypeScript + Tailwind CSS
- 🔐 **安全可靠**：JWT 认证、密码加密、通信加密

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
- **语言**：Go 1.22+
- **Web 框架**：Gin
- **RPC**：gRPC
- **调度**：robfig/cron/v3
- **数据库**：PostgreSQL
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

- Go 1.22+
- Node.js 18+
- PostgreSQL 15+
- Redis 7+

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
# 编辑 config.yaml 配置数据库和 Redis 连接
```

4. **运行服务**
```bash
# 启动 PostgreSQL 和 Redis（使用 Docker）
docker-compose up -d postgres redis

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
# 一键启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

## 📚 文档

- [架构设计](docs/architecture.md)
- [API 文档](docs/api.md)
- [用户手册](docs/user-guide.md)
- [开发指南](docs/development.md)

## 🗺️ 路线图

- [x] 项目初始化和架构设计
- [ ] Master 核心功能实现
- [ ] Worker 核心功能实现
- [ ] 前端界面开发
- [ ] 实时日志系统
- [ ] 安全特性完善
- [ ] 性能优化
- [ ] 文档完善
- [ ] 生产环境部署

## 🤝 贡献

欢迎贡献代码、报告问题或提出新功能建议！

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 🙏 致谢

灵感来源于 [Cronicle](https://github.com/jhuckaby/Cronicle)。
