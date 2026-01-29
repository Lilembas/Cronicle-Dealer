#!/bin/bash

# Worker启动脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 启动 Worker 节点..."
echo "========================================"

# 检查Go
GO_BIN=""
if command -v go &> /dev/null; then
    GO_BIN="go"
elif [ -x "/usr/local/go/bin/go" ]; then
    GO_BIN="/usr/local/go/bin/go"
else
    echo "❌ Go 未安装"
    exit 1
fi

echo "✅ 找到 Go: $GO_BIN"
echo ""

# 运行Worker
$GO_BIN run start_worker.go
