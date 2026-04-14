#!/bin/bash

# Cronicle-Next 清除历史执行记录脚本
# 用法: ./scripts/clear_events.sh

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Cronicle-Next 清除历史记录工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DB_PATH="$PROJECT_ROOT/cronicle.db"
LOG_DIR="/var/log/cronicle/events"

# 检查数据库文件
if [ ! -f "$DB_PATH" ]; then
    echo -e "${RED}❌ 数据库文件不存在: $DB_PATH${NC}"
    exit 1
fi

# 查询当前记录数
COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events;" 2>/dev/null || echo "0")

echo -e "📊 当前历史记录数: ${GREEN}$COUNT${NC}"
echo ""

if [ "$COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅ 没有需要删除的记录${NC}"
    exit 0
fi

# 确认操作
echo -e "${RED}⚠️  警告：此操作将删除所有历史执行记录！${NC}"
echo -ne "是否继续？(输入 'yes' 确认): "
read -r CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo -e "${YELLOW}❌ 操作已取消${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}🗑️  开始删除历史记录...${NC}"

# 删除数据库记录
sqlite3 "$DB_PATH" "DELETE FROM events;" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 成功删除 $COUNT 条数据库记录${NC}"
else
    echo -e "${RED}❌ 删除数据库记录失败${NC}"
    exit 1
fi

# 清理日志文件
echo ""
echo -e "${YELLOW}🧹 清理日志文件...${NC}"

if [ -d "$LOG_DIR" ]; then
    LOG_COUNT=$(find "$LOG_DIR" -name "*.log" -type f 2>/dev/null | wc -l)

    if [ "$LOG_COUNT" -gt 0 ]; then
        rm -f "$LOG_DIR"/*.log 2>/dev/null
        echo -e "${GREEN}✅ 成功删除 $LOG_COUNT 个日志文件${NC}"
    else
        echo -e "${GREEN}✅ 日志目录为空${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  日志目录不存在: $LOG_DIR${NC}"
fi

# 验证删除结果
NEW_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM events;" 2>/dev/null || echo "0")

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 清理完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "📊 剩余记录数: ${GREEN}$NEW_COUNT${NC}"
echo ""
