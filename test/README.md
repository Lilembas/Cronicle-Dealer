# Cronicle-Next 测试目录

本目录包含项目的测试脚本和测试工具。

## 📁 测试脚本

### Shell 脚本
- `run_integration_test.sh` - 集成测试脚本
- `run_worker_test.sh` - Worker 启动测试
- `run_e2e_test.sh` - 端到端测试
- `test_utils.sh` - 公共工具函数库

### Go 测试程序
- `integration_test.go` - 集成测试
- `worker_startup.go` - Worker 启动测试
- `master_worker_e2e.go` - E2E 测试

## 📖 文档

- [QUICK_START.md](QUICK_START.md) - 测试快速开始
- [TESTING_GUIDE.md](TESTING_GUIDE.md) - 详细测试指南
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - 故障排除

## 🛠️ Shell 测试工具

`shell_test_tools/` 目录包含用于测试 Shell 命令执行的工具：
- [SHELL_TEST_README.md](shell_test_tools/../../test/SHELL_TEST_README.md) - Shell 测试工具使用说明

## 🚀 快速开始

### 前置要求

- Go 1.22+
- Redis（可选）
- 配置文件 `config.yaml`

### 运行测试

```bash
# 1. 准备配置文件
cp config.example.yaml config.yaml

# 2. 运行集成测试
./test/run_integration_test.sh

# 3. 运行 E2E 测试
./test/run_e2e_test.sh

# 4. 运行 Worker 测试
./test/run_worker_test.sh
```

详细说明请查看 [QUICK_START.md](QUICK_START.md) 和 [TESTING_GUIDE.md](TESTING_GUIDE.md)。

## 📝 注意事项

- 测试会创建临时数据库文件
- 测试完成后会自动清理测试数据
- 如果 Redis 未运行，某些测试可能会跳过或失败
- 建议在运行测试前确保 Redis 服务可用

## 🔧 故障排除

如果遇到问题，请查看：
1. [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - 常见问题解决方案
2. 测试日志输出
3. 系统日志（如果有）

## 贡献

如果您发现测试问题或有改进建议，请提交 Issue 或 Pull Request。