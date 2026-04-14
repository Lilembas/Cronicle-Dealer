#!/bin/bash

# Cronicle-Next 选择性清除历史记录
# 可以按时间、状态等条件删除

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="$PROJECT_ROOT/cronicle.db"

# 显示菜单
show_menu() {
    echo ""
    echo "========================================"
    echo "  Cronicle-Next 清理历史记录工具"
    echo "========================================"
    echo "1. 删除所有成功记录"
    echo "2. 删除所有失败记录"
    echo "3. 删除 7 天前的记录"
    echo "4. 删除 30 天前的记录"
    echo "5. 删除所有记录（包括日志）"
    echo "6. 查看记录统计"
    echo "0. 退出"
    echo "========================================"
    echo -n "请选择操作 [0-6]: "
}

# 查看统计
show_stats() {
    echo ""
    echo "📊 历史记录统计:"
    echo "-----------------------------------"

    TOTAL=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events;" 2>/dev/null || echo "0")
    SUCCESS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE status='success';" 2>/dev/null || echo "0")
    FAILED=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE status='failed';" 2>/dev/null || echo "0")
    RUNNING=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE status='running';" 2>/dev/null || echo "0")

    echo "总记录数: $TOTAL"
    echo "  - 成功: $SUCCESS"
    echo "  - 失败: $FAILED"
    echo "  - 运行中: $RUNNING"

    # 最早的记录
    OLDEST=$(sqlite3 "$DB_PATH" "SELECT MIN(created_at) FROM events;" 2>/dev/null)
    if [ -n "$OLDEST" ]; then
        echo "最早记录: $OLDEST"
    fi

    # 最新的记录
    NEWEST=$(sqlite3 "$DB_PATH" "SELECT MAX(created_at) FROM events;" 2>/dev/null)
    if [ -n "$NEWEST" ]; then
        echo "最新记录: $NEWEST"
    fi
    echo "-----------------------------------"
}

# 删除成功记录
delete_success() {
    COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE status='success';" 2>/dev/null || echo "0")
    if [ "$COUNT" -eq 0 ]; then
        echo "✅ 没有成功记录需要删除"
        return
    fi

    echo -e "\n⚠️  将删除 $COUNT 条成功记录"
    confirm || return

    sqlite3 "$DB_PATH" "DELETE FROM events WHERE status='success';"
    echo "✅ 已删除 $COUNT 条成功记录"
}

# 删除失败记录
delete_failed() {
    COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE status='failed';" 2>/dev/null || echo "0")
    if [ "$COUNT" -eq 0 ]; then
        echo "✅ 没有失败记录需要删除"
        return
    fi

    echo -e "\n⚠️  将删除 $COUNT 条失败记录"
    confirm || return

    sqlite3 "$DB_PATH" "DELETE FROM events WHERE status='failed';"
    echo "✅ 已删除 $COUNT 条失败记录"
}

# 删除指定天数前的记录
delete_old() {
    DAYS=$1
    COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events WHERE datetime(created_at) < datetime('now', '-$DAYS days');" 2>/dev/null || echo "0")

    if [ "$COUNT" -eq 0 ]; then
        echo "✅ 没有 $DAYS 天前的记录需要删除"
        return
    fi

    echo -e "\n⚠️  将删除 $COUNT 条 $DAYS 天前的记录"
    confirm || return

    sqlite3 "$DB_PATH" "DELETE FROM events WHERE datetime(created_at) < datetime('now', '-$DAYS days');"
    echo "✅ 已删除 $COUNT 条 $DAYS 天前的记录"
}

# 删除所有记录
delete_all() {
    COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events;" 2>/dev/null || echo "0")
    if [ "$COUNT" -eq 0 ]; then
        echo "✅ 没有记录需要删除"
        return
    fi

    echo -e "\n⚠️  将删除所有 $COUNT 条记录（包括日志文件）"
    confirm || return

    sqlite3 "$DB_PATH" "DELETE FROM events;"
    echo "✅ 已删除所有数据库记录"

    # 删除日志文件
    LOG_DIR="/var/log/cronicle/events"
    if [ -d "$LOG_DIR" ]; then
        LOG_COUNT=$(find "$LOG_DIR" -name "*.log" -type f 2>/dev/null | wc -l)
        if [ "$LOG_COUNT" -gt 0 ]; then
            rm -f "$LOG_DIR"/*.log
            echo "✅ 已删除 $LOG_COUNT 个日志文件"
        fi
    fi
}

# 确认操作
confirm() {
    echo -n "确认删除？(yes/no): "
    read -r ANSWER
    [ "$ANSWER" = "yes" ]
}

# 主循环
while true; do
    show_menu
    read -r CHOICE

    case $CHOICE in
        1)
            show_stats
            delete_success
            ;;
        2)
            show_stats
            delete_failed
            ;;
        3)
            show_stats
            delete_old 7
            ;;
        4)
            show_stats
            delete_old 30
            ;;
        5)
            show_stats
            delete_all
            ;;
        6)
            show_stats
            ;;
        0)
            echo "👋 再见！"
            exit 0
            ;;
        *)
            echo "❌ 无效选择，请重试"
            ;;
    esac

    echo ""
    echo "按 Enter 继续..."
    read
done
