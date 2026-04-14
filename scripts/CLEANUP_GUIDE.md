# Cronicle-Next 清除历史记录指南

## 📋 可用方法

### 方法 1：Shell 脚本（推荐）⭐

**最简单的方式，提供交互式确认**

```bash
# 运行清理脚本
./scripts/clear_events.sh
```

**特点**：
- ✅ 交互式确认，防止误操作
- ✅ 显示当前记录数
- ✅ 同时清理数据库和日志文件
- ✅ 验证删除结果

---

### 方法 2：选择性清理（最安全）⭐⭐

**按条件删除，更灵活**

```bash
# 运行选择性清理脚本
./scripts/clear_events_selective.sh
```

**选项**：
1. 删除所有成功记录
2. 删除所有失败记录
3. 删除 7 天前的记录
4. 删除 30 天前的记录
5. 删除所有记录（包括日志）
6. 查看记录统计

**特点**：
- ✅ 交互式菜单
- ✅ 显示统计信息
- ✅ 按条件删除
- ✅ 最安全的方式

---

### 方法 3：Go 脚本

**编程方式，可扩展**

```bash
# 运行 Go 脚本
go run scripts/clear_events.go
```

**特点**：
- ✅ 使用 GORM ORM
- ✅ 事务支持
- ✅ 可自定义逻辑
- ✅ 适合二次开发

---

### 方法 4：直接 SQL（最快）

**适合自动化脚本**

```bash
# SQLite
sqlite3 /path/to/cronicle.db "DELETE FROM events;"

# PostgreSQL
psql -U cronicle -d cronicle -c "DELETE FROM events;"
```

**验证删除**：
```bash
# SQLite
sqlite3 /path/to/cronicle.db "SELECT COUNT(*) FROM events;"

# PostgreSQL
psql -U cronicle -d cronicle -c "SELECT COUNT(*) FROM events;"
```

---

## 🔧 高级用法

### 按时间范围删除

```bash
# 删除 2024 年的所有记录
sqlite3 cronicle.db "DELETE FROM events WHERE strftime('%Y', created_at) = '2024';"

# 删除指定日期之前的记录
sqlite3 cronicle.db "DELETE FROM events WHERE datetime(created_at) < datetime('2024-01-01');"

# 删除最近 7 天的记录
sqlite3 cronicle.db "DELETE FROM events WHERE datetime(created_at) >= datetime('now', '-7 days');"
```

### 按状态删除

```bash
# 只删除成功的记录
sqlite3 cronicle.db "DELETE FROM events WHERE status = 'success';"

# 只删除失败的记录
sqlite3 cronicle.db "DELETE FROM events WHERE status = 'failed';"

# 删除超时的记录
sqlite3 cronicle.db "DELETE FROM events WHERE status = 'timeout';"
```

### 按作业删除

```bash
# 删除特定作业的所有记录
sqlite3 cronicle.db "DELETE FROM events WHERE job_id = 'job_xxx';"
```

### 组合条件

```bash
# 删除 30 天前的失败记录
sqlite3 cronicle.db "DELETE FROM events WHERE status = 'failed' AND datetime(created_at) < datetime('now', '-30 days');"

# 删除特定作业的成功记录（保留失败记录用于调试）
sqlite3 cronicle.db "DELETE FROM events WHERE job_id = 'job_xxx' AND status = 'success';"
```

---

## 📊 查询统计

### 查看记录总数

```bash
sqlite3 cronicle.db "SELECT COUNT(*) FROM events;"
```

### 查看状态分布

```bash
sqlite3 cronicle.db "
SELECT
    status,
    COUNT(*) as count
FROM events
GROUP BY status
ORDER BY count DESC;
"
```

### 查看最旧的记录

```bash
sqlite3 cronicle.db "SELECT * FROM events ORDER BY created_at ASC LIMIT 1;"
```

### 查看最新的记录

```bash
sqlite3 cronicle.db "SELECT * FROM events ORDER BY created_at DESC LIMIT 10;"
```

### 查看每个作业的执行次数

```bash
sqlite3 cronicle.db "
SELECT
    job_id,
    job_name,
    COUNT(*) as total,
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
FROM events
GROUP BY job_id, job_name
ORDER BY total DESC;
"
```

---

## 🗑️ 清理日志文件

### 删除所有日志文件

```bash
rm -f /var/log/cronicle/events/*.log
```

### 删除指定日期前的日志文件

```bash
# 删除 7 天前修改的日志文件
find /var/log/cronicle/events/ -name "*.log" -mtime +7 -delete
```

### 按大小删除

```bash
# 删除大于 100MB 的日志文件
find /var/log/cronicle/events/ -name "*.log" -size +100M -delete
```

---

## ⚠️ 安全建议

### 1. 备份

**删除前先备份数据库**

```bash
# SQLite
cp cronicle.db cronicle.db.backup.$(date +%Y%m%d_%H%M%S)

# PostgreSQL
pg_dump -U cronicle cronicle > cronicle_backup_$(date +%Y%m%d_%H%M%S).sql
```

### 2. 测试

**先在测试环境验证**

```bash
# 查询要删除的记录数（不执行删除）
sqlite3 cronicle.db "SELECT COUNT(*) FROM events WHERE datetime(created_at) < datetime('now', '-30 days');"
```

### 3. 分批删除

**大量数据分批删除，避免锁表**

```bash
# 删除前 1000 条
sqlite3 cronicle.db "DELETE FROM events WHERE id IN (SELECT id FROM events ORDER BY created_at ASC LIMIT 1000);"

# 重复执行直到删除完毕
```

### 4. 定期清理

**设置定时任务自动清理**

```bash
# 添加到 crontab
crontab -e

# 每周日凌晨 2 点删除 30 天前的记录
0 2 * * 0 /path/to/cronicle-next/scripts/clear_events_selective.sh
```

---

## 📝 自动化脚本示例

### 每日清理脚本

```bash
#!/bin/bash
# daily_cleanup.sh

# 保留 30 天的成功记录
sqlite3 /path/to/cronicle.db "DELETE FROM events WHERE status = 'success' AND datetime(created_at) < datetime('now', '-30 days');"

# 保留 90 天的失败记录（用于调试）
sqlite3 /path/to/cronicle.db "DELETE FROM events WHERE status = 'failed' AND datetime(created_at) < datetime('now', '-90 days');"

# 删除 7 天前的日志文件
find /var/log/cronicle/events/ -name "*.log" -mtime +7 -delete

# 记录日志
echo "$(date): 清理完成" >> /var/log/cronicle/cleanup.log
```

### 压缩旧日志

```bash
#!/bin/bash
# compress_old_logs.sh

# 压缩 30 天前的日志文件
find /var/log/cronicle/events/ -name "*.log" -mtime +30 -exec gzip {} \;

echo "$(date): 压缩完成" >> /var/log/cronicle/compress.log
```

---

## 🚨 故障恢复

### 恢复误删的数据

```bash
# SQLite - 从备份恢复
cp cronicle.db.backup.20240414 cronicle.db

# PostgreSQL - 从备份恢复
psql -U cronicle cronicle < cronicle_backup_20240414.sql
```

---

## ✅ 推荐策略

### 开发环境

- **保留**：7 天
- **清理频率**：每天
- **优先清理**：成功记录

### 测试环境

- **保留**：30 天
- **清理频率**：每周
- **优先清理**：成功记录

### 生产环境

- **保留**：90 天
- **清理频率**：每月
- **优先清理**：成功记录
- **归档**：失败记录用于分析

---

## 🎯 快速开始

**第一次使用？推荐这个流程：**

1. **查看统计**：
   ```bash
   ./scripts/clear_events_selective.sh
   # 选择 6 查看记录统计
   ```

2. **备份数据**：
   ```bash
   cp cronicle.db cronicle.db.backup
   ```

3. **选择性删除**：
   ```bash
   ./scripts/clear_events_selective.sh
   # 选择 3 删除 7 天前的记录
   ```

4. **验证结果**：
   ```bash
   ./scripts/clear_events_selective.sh
   # 选择 6 查看记录统计
   ```

---

**📞 需要帮助？**

查看文档：[CLAUDE.md](../CLAUDE.md)
提交问题：[GitHub Issues](https://github.com/cronicle/cronicle-next/issues)
