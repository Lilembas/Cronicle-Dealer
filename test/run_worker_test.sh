#!/bin/bash

# Cronicle-Next Worker 启动测试脚本

set -e

echo "🔧 Worker 启动测试"
echo "=================="

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 检查配置文件（在项目根目录）
if [ ! -f "$PROJECT_ROOT/config.yaml" ]; then
    echo "❌ 配置文件 $PROJECT_ROOT/config.yaml 不存在"
    echo "请先复制 config.example.yaml 到 config.yaml"
    exit 1
fi

# 检查依赖
echo "🔍 检查依赖..."

# 尝试找到 Go 可执行文件
GO_BIN=""
if command -v go &> /dev/null; then
    GO_BIN="go"
elif [ -x "/usr/local/go/bin/go" ]; then
    GO_BIN="/usr/local/go/bin/go"
elif [ -x "/usr/bin/go" ]; then
    GO_BIN="/usr/bin/go"
elif [ -x "$HOME/go/bin/go" ]; then
    GO_BIN="$HOME/go/bin/go"
else
    echo "❌ Go 未安装或不在 PATH 中"
    echo "尝试的路径: /usr/local/go/bin/go, /usr/bin/go, \$HOME/go/bin/go"
    echo "或设置 PATH: export PATH=\$PATH:/usr/local/go/bin"
    exit 1
fi

echo "✅ 依赖检查通过 (Go: $GO_BIN)"

# 构建测试程序
echo "🔨 构建 Worker 测试程序..."
cd "$SCRIPT_DIR"
$GO_BIN build -o worker_test worker_startup.go

echo "🧪 运行 Worker 测试..."
./worker_test "$@"

# 清理
echo "🧹 清理测试文件..."
rm -f worker_test

echo "✅ 测试完成！"
