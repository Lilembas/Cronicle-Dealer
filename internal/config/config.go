package config

import (
	"fmt"
	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Master   MasterConfig   `mapstructure:"master"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Storage  StorageConfig  `mapstructure:"storage"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Mode         string `mapstructure:"mode"` // master 或 worker
	Host         string `mapstructure:"host"`
	HTTPPort     int    `mapstructure:"http_port"`
	GRPCPort     int    `mapstructure:"grpc_port"`
	WebSocketPort int   `mapstructure:"websocket_port"`
}

// MasterConfig Master 配置
type MasterConfig struct {
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Heartbeat HeartbeatConfig `mapstructure:"heartbeat"`
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Enabled      bool `mapstructure:"enabled"`
	TickInterval int  `mapstructure:"tick_interval"`
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Timeout       int `mapstructure:"timeout"`
	CheckInterval int `mapstructure:"check_interval"`
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	MasterAddress string          `mapstructure:"master_address"`
	Node          NodeConfig      `mapstructure:"node"`
	Heartbeat     WorkerHeartbeat `mapstructure:"heartbeat"`
	Executor      ExecutorConfig  `mapstructure:"executor"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID   string   `mapstructure:"node_id"`   // 节点唯一标识，可选
	Hostname string   `mapstructure:"hostname"`
	Tags     []string `mapstructure:"tags"`
}

// WorkerHeartbeat Worker 心跳配置
type WorkerHeartbeat struct {
	Interval int `mapstructure:"interval"`
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	GRPCPort          int `mapstructure:"grpc_port"`
	MaxConcurrentJobs int `mapstructure:"max_concurrent_jobs"`
	DefaultTimeout    int `mapstructure:"default_timeout"`
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
	JWT         JWTConfig `mapstructure:"jwt"`
	WorkerToken string    `mapstructure:"worker_token"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	LogDir            string `mapstructure:"log_dir"`
	LogRetentionDays  int    `mapstructure:"log_retention_days"`
	MaxLogSizeMB      int    `mapstructure:"max_log_size_mb"`
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	// 服务器默认值
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.http_port", 8080)
	viper.SetDefault("server.grpc_port", 9090)

	// 数据库默认值
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.path", "./cronicle.db")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", 300)

	// Redis 默认值
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// 日志默认值
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")

	// Master 默认值
	viper.SetDefault("master.scheduler.enabled", true)
	viper.SetDefault("master.scheduler.tick_interval", 1)
	viper.SetDefault("master.heartbeat.timeout", 60)
	viper.SetDefault("master.heartbeat.check_interval", 30)

	// Worker 默认值
	viper.SetDefault("worker.executor.grpc_port", 9090)
	viper.SetDefault("worker.executor.max_concurrent_jobs", 10)
	viper.SetDefault("worker.executor.default_timeout", 300)
	viper.SetDefault("worker.heartbeat.interval", 30)
}
