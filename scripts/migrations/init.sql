-- Cronicle-Next 数据库初始化脚本
-- 注意：GORM 会自动创建这些表，此脚本仅供参考

-- 创建数据库（如果不存在）
-- CREATE DATABASE IF NOT EXISTS cronicle;

-- 创建索引以提升查询性能

-- jobs 表索引
CREATE INDEX IF NOT EXISTS idx_jobs_enabled ON jobs(enabled);
CREATE INDEX IF NOT EXISTS idx_jobs_category ON jobs(category);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);

-- events 表索引
CREATE INDEX IF NOT EXISTS idx_events_job_id ON events(job_id);
CREATE INDEX IF NOT EXISTS idx_events_node_id ON events(node_id);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_scheduled_time ON events(scheduled_time);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);

-- nodes 表索引
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_last_heartbeat ON nodes(last_heartbeat);

-- users 表索引已由 GORM uniqueIndex 创建

-- 创建默认管理员用户（密码：admin123，已用 bcrypt 加密）
-- 注意：生产环境请修改默认密码
INSERT INTO users (id, username, password, email, role, full_name, active, created_at, updated_at)
VALUES (
  'admin_1706268000_default',
  'admin',
  '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- admin123
  'admin@cronicle.local',
  'admin',
  'System Administrator',
  true,
  NOW(),
  NOW()
) ON CONFLICT (username) DO NOTHING;
