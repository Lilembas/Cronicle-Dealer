# 快速清除历史记录

## 🚀 快速开始

### 选项 1：交互式清理（推荐）

```bash
cd /codespace/developers/linnan/claudeProjects/cronicle-dealer

# 方式 A：选择性清理（最安全）
./scripts/clear_events_selective.sh

# 方式 B：一键清理所有
./scripts/clear_events.sh
```

### 选项 2：命令行直接清理

```bash
# 清理所有记录
sqlite3 cronicle.db "DELETE FROM events;"

# 清理 7 天前的记录
sqlite3 cronicle.db "DELETE FROM events WHERE datetime(created_at) < datetime('now', '-7 days');"

# 只清理成功的记录
sqlite3 cronicle.db "DELETE FROM events WHERE status = 'success';"
```

---

## 📊 当前状态

```
总记录数: 26
  - 成功: 13
  - 失败: 12
  - 运行中: 0
```

---

## ⚠️ 重要提示

1. **删除前先备份**：
   ```bash
   cp cronicle.db cronicle.db.backup
   ```

2. **查看当前统计**：
   ```bash
   ./scripts/clear_events_selective.sh
   # 选择 6 查看记录统计
   ```

3. **推荐清理策略**：
   - 开发环境：保留 7 天
   - 测试环境：保留 30 天
   - 生产环境：保留 90 天

---

## 📖 详细文档

查看 [CLEANUP_GUIDE.md](./CLEANUP_GUIDE.md) 了解更多用法。
