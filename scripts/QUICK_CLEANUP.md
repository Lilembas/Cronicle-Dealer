# 🚀 清除历史记录 - 快速指南

## ✅ 推荐方法

### 方法 1：交互式菜单（最安全）

```bash
make clean-events-selective
```

**功能**：
- ✅ 交互式选择删除条件
- ✅ 查看统计信息
- ✅ 确认后再删除
- ✅ 防止误操作

---

### 方法 2：命令行快捷操作

```bash
# 查看统计
make stats-events

# 清理所有记录
make clean-events
```

---

## 📊 当前状态

```
📊 历史记录统计:
  总记录数: 26
    成功: 13
    失败: 12
    运行中: 0
```

---

## 🎯 常用清理命令

### 按状态清理

```bash
# 只删除成功记录
sqlite3 cronicle.db "DELETE FROM events WHERE status='success';"

# 只删除失败记录
sqlite3 cronicle.db "DELETE FROM events WHERE status='failed';"
```

### 按时间清理

```bash
# 删除 7 天前的记录
sqlite3 cronicle.db "DELETE FROM events WHERE datetime(created_at) < datetime('now', '-7 days');"

# 删除 30 天前的记录
sqlite3 cronicle.db "DELETE FROM events WHERE datetime(created_at) < datetime('now', '-30 days');"
```

### 清理所有记录

```bash
sqlite3 cronicle.db "DELETE FROM events;"
```

---

## ⚠️ 安全提示

### 1. 删除前备份

```bash
cp cronicle.db cronicle.db.backup.$(date +%Y%m%d_%H%M%S)
```

### 2. 先查看统计

```bash
make stats-events
```

### 3. 验证删除结果

```bash
# 查看剩余记录数
sqlite3 cronicle.db "SELECT COUNT(*) FROM events;"
```

---

## 📖 详细文档

查看完整文档：
- **使用指南**: [scripts/CLEANUP_GUIDE.md](scripts/CLEANUP_GUIDE.md)
- **脚本目录**: [scripts/README.md](scripts/README.md)

---

## 🔧 可用脚本

| 脚本 | 功能 | 推荐度 |
|------|------|--------|
| `clear_events_selective.sh` | 交互式选择性清理 | ⭐⭐⭐⭐⭐ |
| `clear_events.sh` | 一键清理所有记录 | ⭐⭐⭐ |
| `clear_events.go` | Go 语言清理脚本 | ⭐⭐ |

---

## 📝 清理建议

### 开发环境
- **保留**: 7 天
- **频率**: 每天清理
- **命令**: 删除 7 天前的成功记录

### 测试环境
- **保留**: 30 天
- **频率**: 每周清理
- **命令**: 删除 30 天前的记录

### 生产环境
- **保留**: 90 天
- **频率**: 每月清理
- **命令**: 删除 90 天前的成功记录，保留失败记录用于分析

---

**🎉 现在就开始清理：`make clean-events-selective`**
