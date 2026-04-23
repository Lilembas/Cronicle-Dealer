# Cronicle-Dealer Distributed Task Scheduling Platform

[English](README.md) | [中文](README_zh.md)

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-4FC08D?style=flat&logo=vue.js)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Status](https://img.shields.io/badge/Status-Beta-yellow.svg)

> A high-performance, scalable, and visual distributed task scheduling and execution platform built with Go + Vue 3

## 📦 Quick Start

### Scenarios
- You have partitioned your server into multiple Docker containers by CPU/memory resources
- You have many scheduled tasks and want to dynamically distribute them to containers for execution
- You want to write your own load balancing strategy
- You want to visually manage these nodes and tasks
- You want to manually execute commands on these nodes at any time

### Steps
- `git clone` & `cd` this repo
- `cp config.example.yaml config.yaml`
- edit `config.yaml`
- run `make all` to build
- run `bin/manager` for manager node
- run `bin/worker` for worker node
- visit `http://[manager_ip]:[http_port]`
- default account: `admin` / `admin123`

## ✨ Features

### Core Features
- 🔄 **Distributed Architecture**: Manager-Worker mode with horizontal scaling support
- 🎯 **Scheduled Execution**: Cron expression support for flexible task scheduling
- 📊 **Real-time Monitoring**: WebSocket real-time push for task status and logs
- 📝 **Log Streaming**: Real-time log push with support for long-running tasks
- ⚖️ **Custom Load Balancing**: User-defined load balancing strategy functions (based on node CPU, memory, etc.)

## 🏗️ Architecture

```
      ┌──────────────┐
      │  Vue 3 Frontend │
      └──────┬───────┘
             │ HTTP/WS
      ┌──────▼──────┐
      │   Manager   │ (Scheduler/API)
      └──────┬──────┘
             │ gRPC Dispatch
   ┌─────────┴─────────┐
   ▼         ▼         ▼
┌─────┐   ┌─────┐   ┌─────┐
│ W-1 │   │ W-2 │   │ W-N │ Worker Nodes (Executor)
└─────┘   └─────┘   └─────┘
```

## 🛠️ Tech Stack

### Backend
- **Language**: Go 1.25+
- **Web Framework**: Gin
- **RPC**: gRPC
- **Scheduling**: robfig/cron/v3
- **Database**: SQLite (default) / PostgreSQL (to be improved)
- **Core Component**: Redis
- **WebSocket**: Melody

### Frontend
- **Framework**: Vue 3 + TypeScript
- **Build Tool**: Vite
- **UI**: PrimeVue + Tailwind CSS
- **State Management**: Pinia
- **Data Fetching**: TanStack Query
- **Log Terminal**: xterm.js

## 📋 TODO
- [ ] Dockerfile
- [ ] English Version

## 🙏 Acknowledgements

Inspired by [Cronicle](https://github.com/jhuckaby/Cronicle).
