package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/analyzer"
	"github.com/flyzstu/RedisSleuth/internal/cluster"
	"github.com/flyzstu/RedisSleuth/internal/collector"
	"github.com/flyzstu/RedisSleuth/internal/config"
	"github.com/flyzstu/RedisSleuth/internal/model"
	"github.com/flyzstu/RedisSleuth/internal/report"
	"github.com/flyzstu/RedisSleuth/internal/sampler"
	"github.com/spf13/cobra"
)

var version = "dev"

type options struct {
	configFile, addr, username, passwordEnv, output    string
	tls, insecureTLS, showFullKey                      bool
	connectTimeout, commandTimeout, duration, interval time.Duration
	scanCount                                          int64
	sampleSize, scanRate, top                          int
}

var opts options

var rootCmd = &cobra.Command{
	Use: "redissleuth", Short: "只读、低侵入的 Redis Cluster 性能诊断工具",
	SilenceUsage: true, SilenceErrors: true,
}

func Execute() error { return rootCmd.Execute() }

func init() {
	d := config.Default()
	f := rootCmd.PersistentFlags()
	f.StringVar(&opts.configFile, "config", "", "YAML 配置文件")
	f.StringVar(&opts.addr, "addr", d.Redis.Addr, "Redis Cluster 入口地址")
	f.StringVar(&opts.username, "username", d.Redis.Username, "用户名（Redis 5 的 default 会使用单参数 AUTH）")
	f.StringVar(&opts.passwordEnv, "password-env", d.Redis.PasswordEnv, "密码环境变量名")
	f.BoolVar(&opts.tls, "tls", d.Redis.TLS, "启用 TLS")
	f.BoolVar(&opts.insecureTLS, "tls-insecure", d.Redis.InsecureTLS, "跳过 TLS 证书校验")
	f.DurationVar(&opts.connectTimeout, "connect-timeout", d.Redis.ConnectTimeout, "连接超时")
	f.DurationVar(&opts.commandTimeout, "command-timeout", d.Redis.CommandTimeout, "单个 Redis 命令超时")
	f.DurationVar(&opts.duration, "duration", d.Sampling.Duration, "采样时长")
	f.DurationVar(&opts.interval, "interval", d.Sampling.Interval, "多点采样间隔")
	f.Int64Var(&opts.scanCount, "scan-count", d.Sampling.ScanCount, "每次 SCAN COUNT 提示值")
	f.IntVar(&opts.sampleSize, "sample-size", d.Sampling.SampleSize, "最大 Key 样本数")
	f.IntVar(&opts.scanRate, "scan-rate", d.Sampling.ScanRate, "每秒最大扫描 Key 数")
	f.IntVar(&opts.top, "top", d.Sampling.Top, "Top 结果数量")
	f.BoolVar(&opts.showFullKey, "show-full-key", d.Output.ShowFullKey, "显示完整 Key（默认脱敏）")
	f.StringVarP(&opts.output, "output", "o", d.Output.Format, "输出格式：table|json")
	rootCmd.Version = version
	rootCmd.AddCommand(topologyCmd(), cpuCmd(), memoryCmd(), slotsCmd(), clientsCmd(), diagnoseCmd())
}

func loadConfig(cmd *cobra.Command) (config.Config, error) {
	cfg, err := config.Load(opts.configFile)
	if err != nil {
		return cfg, err
	}
	set := func(name string) bool { flag := cmd.Flags().Lookup(name); return flag != nil && flag.Changed }
	if set("addr") {
		cfg.Redis.Addr = opts.addr
	}
	if set("username") {
		cfg.Redis.Username = opts.username
	}
	if set("password-env") {
		cfg.Redis.PasswordEnv = opts.passwordEnv
	}
	if set("tls") {
		cfg.Redis.TLS = opts.tls
	}
	if set("tls-insecure") {
		cfg.Redis.InsecureTLS = opts.insecureTLS
	}
	if set("connect-timeout") {
		cfg.Redis.ConnectTimeout = opts.connectTimeout
	}
	if set("command-timeout") {
		cfg.Redis.CommandTimeout = opts.commandTimeout
	}
	if set("duration") {
		cfg.Sampling.Duration = opts.duration
	}
	if set("interval") {
		cfg.Sampling.Interval = opts.interval
	}
	if set("scan-count") {
		cfg.Sampling.ScanCount = opts.scanCount
	}
	if set("sample-size") {
		cfg.Sampling.SampleSize = opts.sampleSize
	}
	if set("scan-rate") {
		cfg.Sampling.ScanRate = opts.scanRate
	}
	if set("top") {
		cfg.Sampling.Top = opts.top
	}
	if set("show-full-key") {
		cfg.Output.ShowFullKey = opts.showFullKey
	}
	if set("output") {
		cfg.Output.Format = opts.output
	}
	return cfg, cfg.Validate()
}

func run(cmd *cobra.Command, kind string) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelWarn}))
	env, err := cluster.Discover(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer env.Close()
	start := time.Now()
	r := model.Report{Metadata: model.Metadata{Start: start}, Cluster: env.Cluster, Nodes: env.Nodes, Findings: []model.Finding{}, Recommendations: []string{}}
	switch kind {
	case "topology":
	case "clients":
		r.Clients, r.ClientDetails, err = collector.Clients(ctx, env)
	case "cpu", "memory":
		var a, b collector.Snapshot
		a, b, err = collector.Sample(ctx, env, cfg.Sampling.Duration)
		if err == nil {
			if kind == "cpu" {
				r.CPU = collector.CPU(a, b, env.Nodes)
				r.Metadata.Calculation = "CPU=采样周期累计 CPU 秒差/实际采样秒数，表示 Redis 进程单核 CPU 百分比"
			} else {
				r.Memory = collector.Memory(a, b, env.Nodes)
			}
		}
	case "slots":
		var snap collector.Snapshot
		snap, err = collector.TakeSnapshot(ctx, env, 0)
		if err == nil {
			r.Keys = sampler.Keys(ctx, env)
			r.Slots = collector.Slots(env, snap, r.Keys)
			r.Keys = topKeys(r.Keys, cfg.Sampling.Top)
		}
	case "diagnose":
		err = collectDiagnose(ctx, env, &r)
	}
	if err != nil {
		return err
	}
	if kind != "topology" {
		r.Findings, r.Recommendations = analyzer.Analyze(cfg, r.CPU, r.Memory, r.Clients, r.Keys, r.Slots)
	}
	r.Metadata.End = time.Now()
	r.Metadata.Duration = r.Metadata.End.Sub(r.Metadata.Start).String()
	return report.Write(cmd.OutOrStdout(), cfg.Output.Format, r)
}

func collectDiagnose(ctx context.Context, env *cluster.Environment, r *model.Report) error {
	first, err := collector.TakeSnapshot(ctx, env, 128)
	if err != nil {
		return err
	}
	// Key/client collection happens inside the CPU sampling window to avoid extending
	// a 30-second diagnosis by another full scan phase.
	baselineClients := make(chan []model.ClientAggregate, 1)
	keyCh := make(chan []model.KeySample, 1)
	go func() {
		v, _, _ := collector.Clients(ctx, env)
		baselineClients <- v
	}()
	go func() { keyCh <- sampler.Keys(ctx, env) }()
	timer := time.NewTimer(env.Config.Sampling.Duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	second, err := collector.TakeSnapshot(ctx, env, 128)
	if err != nil {
		return err
	}
	baseline := <-baselineClients
	r.Clients, r.ClientDetails, _ = collector.Clients(ctx, env)
	applyClientDeltas(r.Clients, baseline, env.Config.Thresholds.ClientConnectionsPerIP)
	r.Keys = <-keyCh
	r.CPU = collector.CPU(first, second, env.Nodes)
	r.Memory = collector.Memory(first, second, env.Nodes)
	r.Slots = collector.Slots(env, second, r.Keys)
	r.Keys = topKeys(r.Keys, env.Config.Sampling.Top)
	r.Metadata.Calculation = "CPU 为累计 CPU 秒差/实际采样秒数（单核百分比）；内存优先 used_memory/maxmemory，maxmemory=0 时使用 total_system_memory"
	return nil
}

func applyClientDeltas(current, baseline []model.ClientAggregate, threshold int) {
	before := make(map[string]int, len(baseline))
	for _, item := range baseline {
		before[item.IP] = item.Connections
	}
	for i := range current {
		current[i].ConnectionDelta = current[i].Connections - before[current[i].IP]
		if current[i].ConnectionDelta >= threshold {
			current[i].Storm = true
		}
	}
}

func topKeys(keys []model.KeySample, top int) []model.KeySample {
	if top > 0 && len(keys) > top {
		return keys[:top]
	}
	return keys
}

func command(use, short, kind string) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return run(cmd, kind) }}
}
