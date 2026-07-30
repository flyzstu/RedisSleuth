package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Redis      Redis      `yaml:"redis"`
	Sampling   Sampling   `yaml:"sampling"`
	Thresholds Thresholds `yaml:"thresholds"`
	Output     Output     `yaml:"output"`
}

type Redis struct {
	Addr           string        `yaml:"addr"`
	Username       string        `yaml:"username"`
	PasswordEnv    string        `yaml:"password_env"`
	TLS            bool          `yaml:"tls"`
	InsecureTLS    bool          `yaml:"insecure_tls"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	CommandTimeout time.Duration `yaml:"command_timeout"`
}

type Sampling struct {
	Duration   time.Duration `yaml:"duration"`
	Interval   time.Duration `yaml:"interval"`
	ScanCount  int64         `yaml:"scan_count"`
	SampleSize int           `yaml:"sample_size"`
	ScanRate   int           `yaml:"scan_rate"`
	Top        int           `yaml:"top"`
}

type Thresholds struct {
	CPUPercent                   float64 `yaml:"cpu_percent"`
	MemoryPercent                float64 `yaml:"memory_percent"`
	FragmentationRatio           float64 `yaml:"fragmentation_ratio"`
	ClientConnectionsPerIP       int     `yaml:"client_connections_per_ip"`
	BigKeyBytes                  int64   `yaml:"big_key_bytes"`
	MasterMemoryDeviationPercent float64 `yaml:"master_memory_deviation_percent"`
	MasterCPUDeviationPercent    float64 `yaml:"master_cpu_deviation_percent"`
	ClientBufferBytes            int64   `yaml:"client_buffer_bytes"`
	ReplicationBacklogBytes      int64   `yaml:"replication_backlog_bytes"`
	NoTTLPercent                 float64 `yaml:"no_ttl_percent"`
}

type Output struct {
	Format      string `yaml:"format"`
	ShowFullKey bool   `yaml:"show_full_key"`
}

func Default() Config {
	return Config{
		Redis:    Redis{Addr: "127.0.0.1:6379", ConnectTimeout: 3 * time.Second, CommandTimeout: 2 * time.Second},
		Sampling: Sampling{Duration: 30 * time.Second, Interval: 5 * time.Second, ScanCount: 100, SampleSize: 1000, ScanRate: 500, Top: 20},
		Thresholds: Thresholds{
			CPUPercent: 80, MemoryPercent: 85, FragmentationRatio: 1.5,
			ClientConnectionsPerIP: 100, BigKeyBytes: 1 << 20,
			MasterMemoryDeviationPercent: 30, MasterCPUDeviationPercent: 50,
			ClientBufferBytes: 64 << 20, ReplicationBacklogBytes: 256 << 20, NoTTLPercent: 80,
		},
		Output: Output{Format: "table"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	// #nosec G304 -- the path is the operator-provided --config input. Reading
	// an arbitrary chosen YAML file is the intended CLI behavior; it is never
	// derived from Redis data or another untrusted remote source.
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("读取配置文件: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置文件: %w", err)
	}
	return cfg, cfg.Validate()
}

func (c Config) Password() (string, error) {
	if c.Redis.PasswordEnv == "" {
		return "", nil
	}
	value, ok := os.LookupEnv(c.Redis.PasswordEnv)
	if !ok {
		return "", fmt.Errorf("密码环境变量 %s 未设置", c.Redis.PasswordEnv)
	}
	return value, nil
}

func (c Config) Validate() error {
	if c.Redis.Addr == "" {
		return errors.New("redis.addr 不能为空")
	}
	if c.Redis.ConnectTimeout <= 0 || c.Redis.CommandTimeout <= 0 {
		return errors.New("Redis 超时必须大于 0")
	}
	if c.Sampling.Duration <= 0 || c.Sampling.Interval <= 0 {
		return errors.New("采样时长和间隔必须大于 0")
	}
	if c.Sampling.ScanCount <= 0 || c.Sampling.SampleSize < 0 || c.Sampling.ScanRate <= 0 {
		return errors.New("SCAN 参数无效")
	}
	if c.Output.Format != "table" && c.Output.Format != "json" {
		return errors.New("output 仅支持 table 或 json")
	}
	return nil
}
