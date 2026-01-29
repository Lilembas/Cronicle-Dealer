#!/bin/bash

# Web服务器启动脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🌐 启动 Shell测试 Web 服务器..."
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

# 运行Web服务器
$GO_BIN run shell_web_server.go
