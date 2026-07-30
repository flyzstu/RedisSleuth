package redisclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/config"
	"github.com/redis/go-redis/v9"
)

// NodeAPI is deliberately small and read-only so collectors can be unit tested.
type NodeAPI interface {
	Addr() string
	Info(ctx context.Context, sections ...string) (string, error)
	ClusterInfo(ctx context.Context) (string, error)
	ClusterNodes(ctx context.Context) (string, error)
	ClusterSlots(ctx context.Context) ([]redis.ClusterSlot, error)
	ClientList(ctx context.Context) (string, error)
	SlowLog(ctx context.Context, count int64) ([]any, error)
	Scan(ctx context.Context, cursor uint64, count int64) ([]string, uint64, error)
	Type(ctx context.Context, key string) (string, error)
	PTTL(ctx context.Context, key string) (time.Duration, error)
	MemoryUsage(ctx context.Context, key string) (int64, error)
	Length(ctx context.Context, typ, key string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

type Client struct {
	addr    string
	timeout time.Duration
	rdb     *redis.Client
}

func New(cfg config.Redis, addr, password string) *Client {
	username := cfg.Username
	// Redis 5 has no ACL and only accepts AUTH <password>. Treating "default"
	// as an empty username keeps the same configuration compatible with 5.x.
	if username == "default" {
		username = ""
	}
	opts := &redis.Options{
		Addr: addr, Username: username, Password: password,
		DialTimeout: cfg.ConnectTimeout, ReadTimeout: cfg.CommandTimeout,
		WriteTimeout: cfg.CommandTimeout, MaxRetries: 1, Protocol: 2,
		DisableIdentity: true,
	}
	if cfg.TLS {
		// #nosec G402 -- certificate verification can only be disabled through
		// the explicit --tls-insecure operator opt-in; secure verification is
		// the default and the README warns against production use.
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureTLS}
	}
	return &Client{addr: addr, timeout: cfg.CommandTimeout, rdb: redis.NewClient(opts)}
}

func (c *Client) Addr() string { return c.addr }

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.Ping(ctx).Err()
}
func (c *Client) Info(ctx context.Context, sections ...string) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.Info(ctx, sections...).Result()
}
func (c *Client) ClusterInfo(ctx context.Context) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.ClusterInfo(ctx).Result()
}
func (c *Client) ClusterNodes(ctx context.Context) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.ClusterNodes(ctx).Result()
}
func (c *Client) ClusterSlots(ctx context.Context) ([]redis.ClusterSlot, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.ClusterSlots(ctx).Result()
}
func (c *Client) ClientList(ctx context.Context) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.ClientList(ctx).Result()
}
func (c *Client) SlowLog(ctx context.Context, count int64) ([]any, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	value, err := c.rdb.Do(ctx, "SLOWLOG", "GET", count).Result()
	if err != nil {
		return nil, err
	}
	rows, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("未知 SLOWLOG 返回类型 %T", value)
	}
	return rows, nil
}
func (c *Client) Scan(ctx context.Context, cursor uint64, count int64) ([]string, uint64, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.Scan(ctx, cursor, "*", count).Result()
}
func (c *Client) Type(ctx context.Context, key string) (string, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.Type(ctx, key).Result()
}
func (c *Client) PTTL(ctx context.Context, key string) (time.Duration, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.PTTL(ctx, key).Result()
}
func (c *Client) MemoryUsage(ctx context.Context, key string) (int64, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.rdb.MemoryUsage(ctx, key, 0).Result()
}
func (c *Client) Length(ctx context.Context, typ, key string) (int64, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	var cmd *redis.IntCmd
	switch typ {
	case "string":
		cmd = c.rdb.StrLen(ctx, key)
	case "hash":
		cmd = c.rdb.HLen(ctx, key)
	case "list":
		cmd = c.rdb.LLen(ctx, key)
	case "set":
		cmd = c.rdb.SCard(ctx, key)
	case "zset":
		cmd = c.rdb.ZCard(ctx, key)
	case "stream":
		cmd = c.rdb.XLen(ctx, key)
	default:
		return 0, nil
	}
	return cmd.Result()
}
func (c *Client) Close() error { return c.rdb.Close() }
