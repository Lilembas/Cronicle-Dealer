package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Manager  ManagerConfig  `mapstructure:"manager"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// ManagerConfig Manager 配置
type ManagerConfig struct {
	Host          string              `mapstructure:"host"`
	HTTPPort      int                 `mapstructure:"http_port"`
	GRPCPort      int                 `mapstructure:"grpc_port"`
	Scheduler     SchedulerConfig     `mapstructure:"scheduler"`
	Heartbeat     HeartbeatConfig     `mapstructure:"heartbeat"`
	DispatchRetry DispatchRetryConfig `mapstructure:"dispatch_retry"`
	History       HistoryConfig       `mapstructure:"history"`
	Security      SecurityConfig      `mapstructure:"security"`
	Database      DatabaseConfig      `mapstructure:"database"`
}

// HistoryConfig 历史数据保留配置
type HistoryConfig struct {
	EventRetentionDays  int `mapstructure:"event_retention_days"`  // 任务记录保留天数，默认 30
	MetricRetentionDays int `mapstructure:"metric_retention_days"` // 节点负载保留天数，默认 7
}

// DispatchRetryConfig 分发重试配置
type DispatchRetryConfig struct {
	MaxRetries   int `mapstructure:"max_retries"`    // 最大重试次数，默认 3
	BaseDelaySec int `mapstructure:"base_delay_sec"` // 基础退避延迟（秒），默认 2
	MaxDelaySec  int `mapstructure:"max_delay_sec"`  // 最大退避上限（秒），默认 30
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Enabled      bool `mapstructure:"enabled"`
	TickInterval int  `mapstructure:"tick_interval"`
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Timeout        int `mapstructure:"timeout"`
	CheckInterval  int `mapstructure:"check_interval"`
	PendingTimeout int `mapstructure:"pending_timeout"` // pending 状态超时（秒），超时后标记为 failed
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	ManagerAddress string          `mapstructure:"manager_address"`
	Node           NodeConfig      `mapstructure:"node"`
	Heartbeat      WorkerHeartbeat `mapstructure:"heartbeat"`
	Executor       ExecutorConfig  `mapstructure:"executor"`
	AuthToken      string          `mapstructure:"auth_token"`
	NodeIDFile     string          `mapstructure:"node_id_file"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID   string   `mapstructure:"node_id"` // 节点唯一标识，可选
	Hostname string   `mapstructure:"hostname"`
	Tags     []string `mapstructure:"tags"`
}

// WorkerHeartbeat Worker 心跳配置
type WorkerHeartbeat struct {
	Interval int `mapstructure:"interval"`
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	GRPCPort       int `mapstructure:"grpc_port"`
	DefaultTimeout int `mapstructure:"default_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	// SQLite 特定配置
	Path string `mapstructure:"path"`
}

// DSN 返回数据库连接字符串
func (c *DatabaseConfig) DSN() string {
	if c.Driver == "sqlite" {
		return c.Path
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.Username, c.Password, c.Database)
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Address 返回 Redis 地址
func (c *RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWT       JWTConfig `mapstructure:"jwt"`
	AuthToken string    `mapstructure:"auth_token"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level           string `mapstructure:"level"`
	Format          string `mapstructure:"format"`
	Output          string `mapstructure:"output"`
	LogDir          string `mapstructure:"log_dir"`
	LogRetentionDays int   `mapstructure:"log_retention_days"`
	MaxLogSizeMB    int    `mapstructure:"max_log_size_mb"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	setDefaults()

	// 允许从环境变量读取
	viper.SetEnvPrefix("CRONICLE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("警告: 无法读取配置文件 (%s)，将尝试使用默认配置和环境变量\n", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	// Manager 默认值
	viper.SetDefault("manager.host", "0.0.0.0")
	viper.SetDefault("manager.http_port", 8080)
	viper.SetDefault("manager.grpc_port", 9090)
	viper.SetDefault("manager.scheduler.enabled", true)
	viper.SetDefault("manager.scheduler.tick_interval", 1)
	viper.SetDefault("manager.heartbeat.timeout", 60)
	viper.SetDefault("manager.heartbeat.check_interval", 30)
	viper.SetDefault("manager.heartbeat.pending_timeout", 10)
	viper.SetDefault("manager.dispatch_retry.max_retries", 1)
	viper.SetDefault("manager.dispatch_retry.base_delay_sec", 2)
	viper.SetDefault("manager.dispatch_retry.max_delay_sec", 30)
	viper.SetDefault("manager.history.event_retention_days", 30)
	viper.SetDefault("manager.history.metric_retention_days", 7)

	// Worker 默认值
	viper.SetDefault("worker.manager_address", "localhost:9090")
	viper.SetDefault("worker.auth_token", "default-token-change-me")
	viper.SetDefault("worker.node.hostname", "")
	viper.SetDefault("worker.node.node_id", "")
	viper.SetDefault("worker.node.tags", []string{"default"})
	viper.SetDefault("worker.executor.grpc_port", 50051)
	viper.SetDefault("worker.executor.default_timeout", 300)
	viper.SetDefault("worker.heartbeat.interval", 30)
	viper.SetDefault("worker.node_id_file", "./data/worker_nodes.json")

	// Database 默认值
	viper.SetDefault("manager.database.driver", "sqlite")
	viper.SetDefault("manager.database.path", "./cronicle.db")
	viper.SetDefault("manager.database.max_open_conns", 25)
	viper.SetDefault("manager.database.max_idle_conns", 10)
	viper.SetDefault("manager.database.conn_max_lifetime", 300)

	// Redis 默认值
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// Security 默认值
	viper.SetDefault("manager.security.jwt.secret", "default-secret-change-me")
	viper.SetDefault("manager.security.jwt.expire_hours", 24)
	viper.SetDefault("manager.security.auth_token", "default-token-change-me")

	// Logging 默认值
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.log_dir", "./logs")
	viper.SetDefault("logging.log_retention_days", 30)
	viper.SetDefault("logging.max_log_size_mb", 100)
}
