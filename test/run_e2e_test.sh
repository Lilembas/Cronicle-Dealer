#!/bin/bash

# Cronicle-Next Master + Worker E2E 测试脚本

set -e

echo "🚀 Master + Worker E2E 测试"
echo "==========================="

# 导入测试工具函数
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test_utils.sh"

# 检查配置文件
CONFIG_FILE="$(check_config_file)"

# 检查依赖
echo "🔍 检查依赖..."
GO_BIN="$(find_go_binary)"
echo "✅ 依赖检查通过 (Go: $GO_BIN)"

# 构建测试程序
echo "🔨 构建 E2E 测试程序..."
build_test_program "$GO_BIN" "master_worker_e2e.go" "e2e_test"

# 运行测试
echo "🧪 运行 E2E 测试..."
run_test_program "e2e_test" "$@"

# 清理
echo "🧹 清理测试文件..."
cleanup_test_files "e2e_test"

echo "✅ E2E 测试完成！"
