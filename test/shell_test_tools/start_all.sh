#!/bin/bash

# Shell测试工具一键启动脚本
# 使用 tmux 或 screen 在多个窗口中启动 Master、Worker 和 Web 服务器

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "╔════════════════════════════════════════════════════╗"
echo "║  🚀 Cronicle Shell测试工具 - 一键启动               ║"
echo "╚════════════════════════════════════════════════════╝"

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

# 检查是否使用 tmux 或 screen
USE_TMUX=false
USE_SCREEN=false

if command -v tmux &> /dev/null; then
    USE_TMUX=true
elif command -v screen &> /dev/null; then
    USE_SCREEN=true
fi

if [ "$USE_TMUX" = false ] && [ "$USE_SCREEN" = false ]; then
    echo ""
    echo "⚠️  未检测到 tmux 或 screen"
    echo "请手动在三个终端窗口中分别运行："
    echo ""
    echo "终端1 - Master:"
    echo "  cd $SCRIPT_DIR && bash run_master.sh"
    echo ""
    echo "终端2 - Worker:"
    echo "  cd $SCRIPT_DIR && bash run_worker.sh"
    echo ""
    echo "终端3 - Web服务器:"
    echo "  cd $SCRIPT_DIR && bash run_web_server.sh"
    echo ""
    exit 0
fi

SESSION_NAME="cronicle-shell-test"

# 清理已存在的会话
if [ "$USE_TMUX" = true ]; then
    tmux has-session -t $SESSION_NAME 2>/dev/null
    if [ $? -eq 0 ]; then
        echo "⚠️  检测到已存在的会话，正在关闭..."
        tmux kill-session -t $SESSION_NAME
        sleep 1
    fi
fi

echo ""
echo "📦 启动方式: $( [ "$USE_TMUX" = true ] && echo 'tmux' || echo 'screen' )"
echo ""

# 使用 tmux 启动
if [ "$USE_TMUX" = true ]; then
    echo "🔧 创建 tmux 会话: $SESSION_NAME"

    # 创建新会话（分离模式）
    tmux new-session -d -s $SESSION_NAME

    # 窗口0: Master
    tmux rename-window -t $SESSION_NAME:0 "Master"
    tmux send-keys -t $SESSION_NAME:0 "cd $SCRIPT_DIR && bash run_master.sh" C-m

    # 窗口1: Worker
    tmux new-window -t $SESSION_NAME -n "Worker"
    tmux send-keys -t $SESSION_NAME:1 "sleep 2 && cd $SCRIPT_DIR && bash run_worker.sh" C-m

    # 窗口2: Web服务器
    tmux new-window -t $SESSION_NAME -n "Web"
    tmux send-keys -t $SESSION_NAME:2 "sleep 4 && cd $SCRIPT_DIR && bash run_web_server.sh" C-m

    # 返回Master窗口
    tmux select-window -t $SESSION_NAME:0

    echo "✅ tmux 会话创建成功！"
    echo ""
    echo "📋 会话信息:"
    echo "   窗口0: Master 节点"
    echo "   窗口1: Worker 节点"
    echo "   窗口2: Web 服务器"
    echo ""
    echo "🎯 使用说明:"
    echo "   - Ctrl+B 0: 切换到 Master 窗口"
    echo "   - Ctrl+B 1: 切换到 Worker 窗口"
    echo "   - Ctrl+B 2: 切换到 Web 窗口"
    echo "   - Ctrl+B [: 前一个窗口"
    echo "   - Ctrl+B ]: 后一个窗口"
    echo "   - Ctrl+B D: 分离会话（服务继续运行）"
    echo "   - Ctrl+B C: 关闭窗口"
    echo ""
    echo "📝 重新连接会话:"
    echo "   tmux attach -t $SESSION_NAME"
    echo ""
    echo "🛑 停止服务:"
    echo "   tmux kill-session -t $SESSION_NAME"
    echo ""
    echo "🚀 正在连接到会话..."

    # 连接到会话
    exec tmux attach-session -t $SESSION_NAME

# 使用 screen 启动
else
    echo "🔧 创建 screen 会话: $SESSION_NAME"

    # 创建新会话
    screen -dmS $SESSION_NAME

    # 启动Master
    screen -S $SESSION_NAME -X screen -t Master bash run_master.sh

    # 启动Worker
    screen -S $SESSION_NAME -X screen -t Worker bash -c 'sleep 2 && bash run_worker.sh'

    # 启动Web服务器
    screen -S $SESSION_NAME -X screen -t Web bash -c 'sleep 4 && bash run_web_server.sh'

    echo "✅ screen 会话创建成功！"
    echo ""
    echo "📋 会话信息:"
    echo "   窗口Master: Master 节点"
    echo "   窗口Worker: Worker 节点"
    echo "   窗口Web: Web 服务器"
    echo ""
    echo "🎯 使用说明:"
    echo "   - Ctrl+A c: 创建新窗口"
    echo "   - Ctrl+A n: 下一个窗口"
    echo "   - Ctrl+A p: 上一个窗口"
    echo "   - Ctrl+A 0-9: 切换到指定窗口"
    echo "   - Ctrl+A D: 分离会话（服务继续运行）"
    echo "   - Ctrl+A k: 关闭窗口"
    echo ""
    echo "📝 重新连接会话:"
    echo "   screen -r $SESSION_NAME"
    echo ""
    echo "🛑 停止服务:"
    echo "   screen -S $SESSION_NAME -X quit"
    echo ""
    echo "🚀 正在连接到会话..."

    # 连接到会话
    exec screen -r $SESSION_NAME
fi
