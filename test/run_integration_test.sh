#!/bin/bash

# Cronicle-Next 集成测试脚本

set -e

echo "🚀 开始 Cronicle-Next 后端通讯链路测试"

# 导入测试工具函数
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test_utils.sh"

# 检查配置文件
CONFIG_FILE="$(check_config_file)"

# 检查依赖
echo "🔍 检查依赖..."
GO_BIN="$(find_go_binary)"
echo "✅ 依赖检查通过 (Go: $GO_BIN)"

# 检查 Redis 是否运行
PROJECT_ROOT="$(get_project_root)"
if ! nc -z localhost 6379 2>/dev/null; then
    echo "⚠️  Redis 未运行，启动 Redis 容器..."
    docker-compose -f "$PROJECT_ROOT/deployments/docker-compose.yml" up -d redis
    sleep 5
fi

# 构建测试程序
echo "🔨 构建测试程序..."
build_test_program "$GO_BIN" "integration_test.go" "integration_test"

# 运行测试
echo "🧪 运行集成测试..."
run_test_program "integration_test"

# 清理
echo "🧹 清理测试文件..."
cleanup_test_files "integration_test"

echo "✅ 测试完成！"
