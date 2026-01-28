# Cronicle-Next 测试快速开始

## 快速命令

### 1️⃣ Worker 启动测试
```bash
# 使用 Shell 脚本（推荐）
./test/run_worker_test.sh

# 或直接运行
cd test && go run worker_startup.go -duration 30s
```

**测试内容:**
- ✅ Worker 连接 Master
- ✅ 节点注册
- ✅ 心跳机制
- ✅ 执行器启动

---

### 2️⃣ 完整 E2E 测试（推荐）
```bash
# 使用 Shell 脚本（推荐）
./test/run_e2e_test.sh

# 或自定义参数
cd test && go run master_worker_e2e.go -jobs 5 -wait 120s
```

**测试内容:**
- ✅ Master + Worker 启动
- ✅ 任务创建和调度
- ✅ 任务执行（通过 gRPC）
- ✅ 结果验证

---

## 前置条件

```bash
# 1. 复制配置文件
cp config.example.yaml config.yaml

# 2. 确保 Redis 运行（如果需要）
docker-compose -f deployments/docker-compose.yml up -d redis

# 3. 验证配置
cat config.yaml
```

---

## 测试输出

成功时你会看到：
- ✅ 所有步骤的绿色对勾
- 📊 任务执行统计
- 🎉 测试完成提示

---

## 详细文档

📖 查看 [TESTING_GUIDE.md](TESTING_GUIDE.md) 获取完整文档
