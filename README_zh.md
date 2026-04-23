# Cronicle-Dealer 分布式任务调度平台

[English](README.md) | [中文](README_zh.md)

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat&logo=vue.js)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Status](https://img.shields.io/badge/Status-Beta-yellow.svg)

> 一个高性能、可扩展、可视化的分布式任务调度与执行平台，基于 Go + Vue 3 构建

![alt text](docs/images/framework.png)

## 📦 快速开始

### 场景
- 你将服务器按 CPU/内存资源分割成了多个 Docker 容器
- 你有很多定时执行任务，你希望将这些任务动态地分配给容器执行
- 你希望自己编写负载均衡策略
- 你希望可视化查看和管理这些节点和任务
- 你希望随时在这些节点手动执行命令

### 方法
- `git clone` & `cd` 本仓库
- `cp config.example.yaml config.yaml`
- 编辑 `config.yaml`
- 执行 `make all` 
- 执行 `bin/manager`  (manager节点)
- 执行 `bin/worker`  (worker节点)
- 访问 `http://[manager_ip]:[http_port]`
- 默认账号: `admin` / `admin123`

## ✨ 特性

### 核心功能
- 🔄 **分布式架构**：Manager-Worker 模式，支持水平扩展
- 🎯 **定时调度**：支持 Cron 表达式，灵活的任务调度
- 📊 **实时监控**：WebSocket 实时推送任务状态和日志
- 📝 **日志流式传输**：实时日志推送，支持长任务
- ⚖️ **自定义负载均衡**：允许用户自定义负载均衡策略函数（根据节点 CPU、内存等负载）

## 🏗️ 架构

```
      ┌──────────────┐
      │     Web      │
      └──────┬───────┘
             │ HTTP/WS
      ┌──────▼──────┐
      │   Manager   │ (Scheduler/API)
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

## 📋 TODO
- [ ] Dockerfile

## 🙏 致谢

灵感来源于 [Cronicle](https://github.com/jhuckaby/Cronicle)。
