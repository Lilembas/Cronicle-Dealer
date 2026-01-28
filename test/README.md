# Cronicle-Next 集成测试

## 测试目的

这个测试脚本用于验证整个后端通讯链路的完整性，包括：

- ✅ SQLite 数据库连接和迁移
- ✅ Redis 连接和队列功能
- ✅ Master 节点启动和运行
- ✅ Worker 节点注册和心跳
- ✅ gRPC 通讯（Master ↔ Worker）
- ✅ 任务调度和状态缓存

## 运行测试

### 方法一：使用 Makefile（推荐）

```bash
# 运行集成测试
make integration-test
```

### 方法二：直接运行 Go 程序

```bash
# 进入 test 目录
cd test

# 运行测试
go run integration_test.go

# 或指定配置文件
go run integration_test.go -config ../config.yaml
```

### 方法三：使用 Shell 脚本

```bash
# 运行集成测试脚本
./test/run_integration_test.sh
```

## 测试要求

- **Go 1.22+** 已安装
- **Redis** 服务运行（可选，但推荐）
  - 如果未运行，脚本会自动启动 Redis Docker 容器
- **配置文件** `../config.yaml` 存在
  - 如果不存在，请复制 `config.example.yaml` 到 `config.yaml`

## 测试输出

成功运行时，您将看到类似以下的输出：

```
🧪 Cronicle-Next 后端通讯链路测试
================================
1️⃣ 测试 SQLite 数据库连接...
✅ SQLite 数据库连接成功
2️⃣ 测试 Redis 连接...
✅ Redis 连接成功
3️⃣ 启动 Master 节点...
✅ Master 节点启动成功
4️⃣ 启动 Worker 节点...
✅ Worker 节点启动成功
5️⃣ 测试任务创建和调度...
✅ 任务已添加到队列
6️⃣ 验证 Worker 心跳...
✅ Worker 心跳正常，节点在线
7️⃣ 验证任务状态...
✅ 任务状态: pending
8️⃣ 清理测试数据...
✅ 测试数据清理完成

🎉 所有测试完成！
✅ SQLite 数据库: 正常
✅ Redis: 正常
✅ Master 节点: 正常
✅ Worker 节点: 正常
✅ gRPC 通讯: 正常
✅ 任务调度: 正常
✅ 心跳机制: 正常
✅ 状态缓存: 正常
```

## 注意事项

- 这个测试脚本**不包含完整的任务执行流程**，因为它需要 Worker 实际执行任务
- 如需测试完整任务执行，请运行实际的 Master 和 Worker 服务
- 测试会在当前目录创建 `cronicle.db` 文件（SQLite 数据库），测试完成后会清理相关数据
- 如果 Redis 未运行，测试可能会失败或跳过某些 Redis 相关的检查

## 故障排除

### 1. 配置文件不存在
```
❌ 配置文件 ../config.yaml 不存在
```
**解决方法**:
```bash
cp config.example.yaml config.yaml
```

### 2. Redis 连接失败
```
❌ Redis 连接失败: ...
```
**解决方法**:
- 启动本地 Redis 服务，或
- 运行 `docker-compose -f deployments/docker-compose.yml up -d redis`

### 3. 数据库迁移失败
```
❌ 数据库迁移失败: ...
```
**解决方法**:
- 删除现有的 `cronicle.db` 文件并重试
- 检查文件权限

## 贡献

如果您发现测试脚本中的问题或有改进建议，请提交 Issue 或 Pull Request。