# Cronicle-Next 测试快速开始

本指南帮助你快速运行 Cronicle-Next 的测试。

## 🚀 快速开始

### 前置条件

```bash
# 1. 复制配置文件
cp config.example.yaml config.yaml

# 2. 确保 Redis 运行（可选但推荐）
docker-compose -f deployments/docker-compose.yml up -d redis
```

### 运行测试

#### 1️⃣ 集成测试（最快速）

```bash
# 使用 Shell 脚本
./test/run_integration_test.sh

# 或直接运行 Go 程序
cd test && go run integration_test.go
```

**测试内容：**
- ✅ 数据库连接
- ✅ Redis 连接
- ✅ Master 节点启动
- ✅ Worker 节点注册和心跳
- ✅ gRPC 通讯

---

#### 2️⃣ Worker 启动测试

```bash
# 使用 Shell 脚本
./test/run_worker_test.sh

# 或自定义运行时长
cd test && go run worker_startup.go -duration 30s
```

**测试内容：**
- ✅ Worker 连接 Master
- ✅ 节点注册
- ✅ 心跳机制
- ✅ 执行器启动

---

#### 3️⃣ E2E 测试（完整流程）

```bash
# 使用 Shell 脚本（推荐）
./test/run_e2e_test.sh

# 或自定义参数
cd test && go run master_worker_e2e.go -jobs 5 -wait 120s
```

**测试内容：**
- ✅ Master + Worker 启动
- ✅ 任务创建和调度
- ✅ 任务执行（通过 gRPC）
- ✅ 结果验证

---

## 📊 测试输出

成功时你会看到：
- ✅ 所有步骤的绿色对勾
- 📊 任务执行统计
- 🎉 测试完成提示

示例输出：
```
🧪 Cronicle-Next E2E 测试
================================
1️⃣ 初始化数据库和 Redis... ✅
2️⃣ 启动 Master 服务... ✅
3️⃣ 连接 Worker 到 Master... ✅
4️⃣ 创建测试任务... ✅
5️⃣ 调度任务执行... ✅
6️⃣ 监控任务执行... ✅
7️⃣ 清理测试数据... ✅

🎉 所有测试完成！
📊 成功: 5 | 失败: 0
```

---

## 🔧 参数说明

### Worker 启动测试
- `-config` - 配置文件路径（默认: `../config.yaml`）
- `-duration` - 测试运行时长（默认: `30s`）

### E2E 测试
- `-config` - 配置文件路径（默认: `../config.yaml`）
- `-jobs` - 测试任务数量（默认: `3`）
- `-wait` - 等待任务完成的时长（默认: `60s`）

---

## ⚠️ 常见问题

### 1. 配置文件不存在
```bash
❌ 配置文件 ../config.yaml 不存在
```
**解决方法：**
```bash
cp config.example.yaml config.yaml
```

### 2. Redis 连接失败
```bash
❌ Redis 连接失败: dial tcp: connection refused
```
**解决方法：**
```bash
# 启动本地 Redis
redis-server

# 或使用 Docker
docker-compose -f deployments/docker-compose.yml up -d redis
```

### 3. Go 未找到
```bash
❌ Go 未安装或不在 PATH 中
```
**解决方法：**
```bash
# 检查 Go 版本
go version

# 临时添加到 PATH
export PATH=$PATH:/usr/local/go/bin
```

---

## 📖 详细文档

- [TESTING_GUIDE.md](TESTING_GUIDE.md) - 详细测试指南
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - 故障排除
- [README.md](README.md) - 测试目录说明

---

## 📝 注意事项

- 测试会创建临时数据库文件
- 测试完成后会自动清理测试数据
- 如果 Redis 未运行，某些测试可能会跳过
- 建议在开发环境中运行测试

---

## 🎯 测试最佳实践

### 开发阶段
使用 Worker 启动测试快速验证功能：
```bash
./test/run_worker_test.sh
```

### 集成测试
使用 E2E 测试验证完整流程：
```bash
./test/run_e2e_test.sh
```

### CI/CD
在 CI/CD 流程中运行完整测试：
```bash
./test/run_e2e_test.sh -jobs 10 -wait 180s
```
