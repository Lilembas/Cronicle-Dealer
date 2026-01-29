# Shell命令测试工具使用说明

## 功能介绍

这是一个简单的Web测试工具，用于测试Master向Worker发送Shell命令并实时返回执行结果。

### 主要特性

- ✨ 简洁的Web界面，上下两个文本框
- 🚀 实时执行Shell命令
- 📊 显示执行结果、退出码、执行时间
- 🎨 美观的界面设计
- 📈 统计信息面板
- 🔧 多个快速命令示例
- 🔄 Master、Worker、Web服务器分离启动

## 快速开始

### 方式一：一键启动（推荐）

使用 tmux 或 screen 在多窗口中同时启动所有组件：

```bash
cd test/shell_test_tools
bash start_all.sh
```

这将自动启动：
- **Master 节点**（窗口0）
- **Worker 节点**（窗口1）
- **Web 服务器**（窗口2）

### 方式二：分别启动

如果需要分别启动各个组件，可以打开三个终端窗口：

**终端1 - 启动 Master：**
```bash
cd test/shell_test_tools
bash run_master.sh
```

**终端2 - 启动 Worker：**
```bash
cd test/shell_test_tools
bash run_worker.sh
```

**终端3 - 启动 Web 服务器：**
```bash
cd test/shell_test_tools
bash run_web_server.sh
```

### 访问测试页面

服务启动后，在浏览器中访问：

```
http://localhost:8888
```

## 使用方法

### 基本使用

1. **输入命令**：在上方文本框中输入要执行的Shell命令
2. **执行命令**：点击"执行命令"按钮或按回车键
3. **查看结果**：下方文本框会实时显示命令执行结果

### 快速示例

页面提供了多个常用命令示例按钮，点击即可快速填入：

- `pwd` - 显示当前目录
- `ls -la` - 列出文件详情
- `whoami` - 显示当前用户
- `date` - 显示当前时间
- `uname -a` - 显示系统信息
- `df -h` - 显示磁盘使用情况
- `free -h` - 显示内存使用情况
- `uptime` - 显示系统运行时间
- `echo $HOME` - 显示环境变量
- `ps aux | head -20` - 显示进程列表

### 功能按钮

- **执行命令**：执行输入的Shell命令
- **清空输出**：清空下方的输出显示区域

## API接口

### 执行Shell命令

**请求：**

```bash
POST /api/v1/shell/execute
Content-Type: application/json

{
  "command": "ls -la"
}
```

**响应：**

```json
{
  "command": "ls -la",
  "output": "命令输出内容",
  "exit_code": 0,
  "status": "completed"
}
```

### 健康检查

**请求：**

```bash
GET /api/v1/health
```

**响应：**

```json
{
  "status": "ok",
  "online_nodes": 1,
  "timestamp": 1706497200
}
```

### 获取统计信息

**请求：**

```bash
GET /api/v1/stats
```

**响应：**

```json
{
  "online_nodes": 1,
  "running_jobs": 0
}
```

## 架构说明

### 工作流程

```
用户输入命令
    ↓
Web服务器接收
    ↓
创建临时Job和Event
    ↓
添加到Redis任务队列
    ↓
Master TaskConsumer获取任务
    ↓
Dispatcher通过gRPC发送给Worker
    ↓
Worker Executor执行命令
    ↓
保存结果到Redis
    ↓
Web服务器轮询获取结果
    ↓
返回给用户界面
```

### 技术栈

- **后端**：Go + Gin框架
- **前端**：原生HTML + CSS + JavaScript
- **通信**：REST API
- **数据库**：SQLite（存储Job和Event）
- **缓存**：Redis（任务队列和结果缓存）

## 注意事项

1. **权限限制**：只能执行当前用户有权限的命令
2. **超时设置**：命令执行超时时间为30秒
3. **并发限制**：每次只能执行一个命令（串行执行）
4. **安全警告**：这是测试工具，不要在生产环境使用
5. **资源清理**：每次执行后会自动清理测试数据

## 端口说明

- **Web服务**：8888端口
- **Master gRPC**：8081端口（由config.yaml配置）
- **Worker gRPC**：9090端口（由config.yaml配置）

## 故障排查

### 服务无法启动

1. 检查端口8888是否被占用
2. 检查配置文件 `../config.yaml` 是否存在
3. 检查Redis是否正常运行
4. 查看日志输出

### 命令执行失败

1. 检查Worker节点是否在线（访问 /api/v1/health）
2. 检查命令语法是否正确
3. 检查当前用户是否有执行权限
4. 查看浏览器控制台的错误信息

### 无法访问页面

1. 确认服务已成功启动
2. 检查防火墙设置
3. 确认访问地址为 http://localhost:8888
4. 查看服务端日志

## 开发建议

如需扩展功能，可以参考以下方向：

1. **多命令并发**：支持同时执行多个命令
2. **命令历史**：保存执行历史记录
4. **文件上传**：支持上传脚本文件
5. **定时执行**：支持定时执行命令
6. **多Worker支持**：选择特定的Worker执行
7. **实时流式输出**：使用WebSocket实现真正的实时输出

## 许可证

MIT License
