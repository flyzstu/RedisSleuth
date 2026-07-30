package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/flyzstu/RedisSleuth/internal/config"
	"github.com/flyzstu/RedisSleuth/internal/model"
	"github.com/flyzstu/RedisSleuth/internal/redisclient"
)

type Environment struct {
	Config  config.Config
	Nodes   []model.Node
	Clients map[string]redisclient.NodeAPI
	Cluster model.Cluster
}

func Discover(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Environment, error) {
	password, err := cfg.Password()
	if err != nil {
		return nil, err
	}
	entry := redisclient.New(cfg.Redis, cfg.Redis.Addr, password)
	rawNodes, err := entry.ClusterNodes(ctx)
	if err != nil {
		_ = entry.Close()
		return nil, fmt.Errorf("CLUSTER NODES（请确认目标为 Redis Cluster）: %w", err)
	}
	nodes, err := ParseNodes(rawNodes)
	if err != nil {
		_ = entry.Close()
		return nil, err
	}
	rawCluster, err := entry.ClusterInfo(ctx)
	if err != nil {
		_ = entry.Close()
		return nil, fmt.Errorf("CLUSTER INFO: %w", err)
	}
	ci := ParseClusterInfo(rawCluster)
	// Redis 5 supports CLUSTER SLOTS. Use it as the authoritative slot view,
	// while CLUSTER NODES remains the source for roles and link state.
	slots, slotsErr := entry.ClusterSlots(ctx)
	if slotsErr != nil {
		logger.Warn("CLUSTER SLOTS 读取失败", "error", slotsErr)
	} else {
		byID := make(map[string][]string)
		for _, s := range slots {
			if len(s.Nodes) == 0 {
				continue
			}
			byID[s.Nodes[0].ID] = append(byID[s.Nodes[0].ID], fmt.Sprintf("%d-%d", s.Start, s.End))
		}
		for i := range nodes {
			if ranges := byID[nodes[i].ID]; len(ranges) > 0 {
				nodes[i].Slots = ranges
			}
		}
	}
	env := &Environment{Config: cfg, Nodes: nodes, Clients: make(map[string]redisclient.NodeAPI)}
	for _, n := range nodes {
		if contains(n.Flags, "fail") || contains(n.Flags, "noaddr") || n.Addr == "" {
			continue
		}
		var client redisclient.NodeAPI
		if n.Addr == cfg.Redis.Addr {
			client = entry
		} else {
			client = redisclient.New(cfg.Redis, n.Addr, password)
		}
		if err := client.Ping(ctx); err != nil {
			logger.Warn("节点不可访问", "node", n.Addr, "error", err)
			_ = client.Close()
			continue
		}
		env.Clients[n.Addr] = client
	}
	if _, ok := env.Clients[cfg.Redis.Addr]; !ok {
		_ = entry.Close()
	}
	env.Cluster.State = ci["cluster_state"]
	env.Cluster.KnownNodes, _ = strconv.Atoi(ci["cluster_known_nodes"])
	for i := range env.Nodes {
		env.Nodes[i].ClusterState = env.Cluster.State
		if env.Nodes[i].Role == "master" {
			env.Cluster.Masters++
		} else {
			env.Cluster.Replicas++
		}
		if client := env.Clients[env.Nodes[i].Addr]; client != nil {
			if raw, e := client.Info(ctx, "replication"); e == nil {
				info := ParseInfo(raw)
				if env.Nodes[i].Role == "master" {
					env.Nodes[i].ReplicationState = "connected_replicas=" + info["connected_slaves"]
				} else {
					env.Nodes[i].ReplicationState = info["master_link_status"]
				}
			}
		}
	}
	if env.Cluster.KnownNodes == 0 {
		env.Cluster.KnownNodes = len(nodes)
	}
	return env, nil
}

func (e *Environment) Close() {
	for _, client := range e.Clients {
		_ = client.Close()
	}
}
