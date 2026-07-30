package collector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/cluster"
	"github.com/flyzstu/RedisSleuth/internal/model"
	"github.com/flyzstu/RedisSleuth/internal/redisclient"
)

type Snapshot struct {
	Time  time.Time
	Nodes map[string]NodeSnapshot
}

type NodeSnapshot struct {
	Info     map[string]string
	Commands map[string]CommandCounter
	Slowlog  []model.SlowEntry
}

type CommandCounter struct{ Calls, Usec int64 }

func TakeSnapshot(ctx context.Context, env *cluster.Environment, slowCount int64) (Snapshot, error) {
	out := Snapshot{Time: time.Now(), Nodes: make(map[string]NodeSnapshot)}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, len(env.Clients))
	for addr, client := range env.Clients {
		wg.Add(1)
		go func(addr string, client redisclient.NodeAPI) {
			defer wg.Done()
			raw, err := client.Info(ctx)
			if err != nil {
				errs <- fmt.Errorf("%s INFO: %w", addr, err)
				return
			}
			info := cluster.ParseInfo(raw)
			slow, err := client.SlowLog(ctx, slowCount)
			if err != nil {
				slow = nil
			}
			node := NodeSnapshot{Info: info, Commands: ParseCommandStats(info), Slowlog: ParseSlowlog(slow)}
			mu.Lock()
			out.Nodes[addr] = node
			mu.Unlock()
		}(addr, client)
	}
	wg.Wait()
	close(errs)
	if len(out.Nodes) == 0 {
		for err := range errs {
			return out, err
		}
		return out, fmt.Errorf("没有成功采集任何节点")
	}
	return out, nil
}

func Sample(ctx context.Context, env *cluster.Environment, duration time.Duration) (Snapshot, Snapshot, error) {
	first, err := TakeSnapshot(ctx, env, 128)
	if err != nil {
		return first, Snapshot{}, err
	}
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	interval := env.Config.Sampling.Interval
	if interval > duration {
		interval = duration
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return first, Snapshot{}, ctx.Err()
		case <-ticker.C:
			// Intermediate snapshots make sampling continuous. SLOWLOG is only
			// needed at the boundaries, so request zero entries here.
			if _, err := TakeSnapshot(ctx, env, 0); err != nil {
				return first, Snapshot{}, err
			}
		case <-deadline.C:
			second, err := TakeSnapshot(ctx, env, 128)
			return first, second, err
		}
	}
}

func ParseCommandStats(info map[string]string) map[string]CommandCounter {
	out := make(map[string]CommandCounter)
	for key, value := range info {
		if !strings.HasPrefix(key, "cmdstat_") {
			continue
		}
		fields := parseCSV(value)
		calls, _ := strconv.ParseInt(fields["calls"], 10, 64)
		usec, _ := strconv.ParseInt(fields["usec"], 10, 64)
		out[strings.TrimPrefix(key, "cmdstat_")] = CommandCounter{Calls: calls, Usec: usec}
	}
	return out
}

func ParseSlowlog(rows []any) []model.SlowEntry {
	out := make([]model.SlowEntry, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.([]any)
		if !ok || len(row) < 4 {
			continue
		}
		id := anyInt64(row[0])
		unix := anyInt64(row[1])
		duration := anyInt64(row[2])
		command := ""
		if args, ok := row[3].([]any); ok {
			parts := make([]string, 0, len(args))
			for _, arg := range args {
				parts = append(parts, anyString(arg))
			}
			command = strings.Join(parts, " ")
		}
		entry := model.SlowEntry{ID: id, Time: time.Unix(unix, 0), Duration: time.Duration(duration) * time.Microsecond, Command: command}
		if len(row) > 4 {
			entry.Client = anyString(row[4])
		}
		if len(row) > 5 {
			entry.Name = anyString(row[5])
		}
		out = append(out, entry)
	}
	return out
}

func CPU(first, second Snapshot, nodes []model.Node) []model.CPUStats {
	seconds := second.Time.Sub(first.Time).Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	var out []model.CPUStats
	for _, node := range nodes {
		a, aok := first.Nodes[node.Addr]
		b, bok := second.Nodes[node.Addr]
		if !aok || !bok {
			continue
		}
		cpu := (floatValue(b.Info, "used_cpu_sys") + floatValue(b.Info, "used_cpu_user") -
			floatValue(a.Info, "used_cpu_sys") - floatValue(a.Info, "used_cpu_user")) / seconds * 100
		children := (floatValue(b.Info, "used_cpu_sys_children") + floatValue(b.Info, "used_cpu_user_children") -
			floatValue(a.Info, "used_cpu_sys_children") - floatValue(a.Info, "used_cpu_user_children")) / seconds * 100
		if cpu < 0 {
			cpu = 0
		}
		if children < 0 {
			children = 0
		}
		stats := model.CPUStats{
			Node: node.Addr, Role: node.Role, CPUPercent: cpu, ChildrenCPUPercent: children,
			OPS:           intValue(b.Info, "instantaneous_ops_per_sec"),
			CommandsDelta: nonnegative(intValue(b.Info, "total_commands_processed") - intValue(a.Info, "total_commands_processed")),
			BGSaveActive: b.Info["rdb_bgsave_in_progress"] == "1" || a.Info["rdb_bgsave_in_progress"] == "1" ||
				intValue(b.Info, "rdb_last_save_time") > intValue(a.Info, "rdb_last_save_time"),
			AOFRewriteActive: b.Info["aof_rewrite_in_progress"] == "1" || a.Info["aof_rewrite_in_progress"] == "1",
			FullSyncLikely:   b.Info["master_sync_in_progress"] == "1" || intValue(b.Info, "sync_full") > intValue(a.Info, "sync_full"),
		}
		if node.Role == "replica" {
			stats.ReplicationAbnormal = b.Info["master_link_status"] != "up"
		} else if intValue(b.Info, "connected_slaves") < intValue(a.Info, "connected_slaves") {
			stats.ReplicationAbnormal = true
		}
		for name, current := range b.Commands {
			previous := a.Commands[name]
			calls := nonnegative(current.Calls - previous.Calls)
			usec := nonnegative(current.Usec - previous.Usec)
			if calls == 0 {
				continue
			}
			stats.CommandDeltas = append(stats.CommandDeltas, model.CommandStat{Name: name, CallsDelta: calls, UsecDelta: usec, UsecPerCall: float64(usec) / float64(calls)})
		}
		sort.Slice(stats.CommandDeltas, func(i, j int) bool { return stats.CommandDeltas[i].UsecDelta > stats.CommandDeltas[j].UsecDelta })
		if len(stats.CommandDeltas) > 10 {
			stats.CommandDeltas = stats.CommandDeltas[:10]
		}
		lastID := int64(-1)
		for _, entry := range a.Slowlog {
			if entry.ID > lastID {
				lastID = entry.ID
			}
		}
		for _, entry := range b.Slowlog {
			if entry.ID > lastID {
				stats.Slowlog = append(stats.Slowlog, entry)
			}
		}
		out = append(out, stats)
	}
	return out
}

func Memory(first, second Snapshot, nodes []model.Node) []model.MemoryStats {
	var out []model.MemoryStats
	for _, node := range nodes {
		a, aok := first.Nodes[node.Addr]
		b, bok := second.Nodes[node.Addr]
		if !aok || !bok {
			continue
		}
		used, max, total := intValue(b.Info, "used_memory"), intValue(b.Info, "maxmemory"), intValue(b.Info, "total_system_memory")
		basis, denominator := "maxmemory", max
		if denominator == 0 {
			basis, denominator = "total_system_memory", total
		}
		percent := float64(0)
		if denominator > 0 {
			percent = float64(used) / float64(denominator) * 100
		}
		out = append(out, model.MemoryStats{
			Node: node.Addr, Role: node.Role, UsedMemory: used, UsedMemoryRSS: intValue(b.Info, "used_memory_rss"),
			UsedMemoryPeak: intValue(b.Info, "used_memory_peak"), MaxMemory: max, TotalSystemMemory: total,
			MemoryPercent: percent, MemoryPercentBasis: basis,
			FragmentationRatio: floatValue(b.Info, "mem_fragmentation_ratio"),
			AllocatorFragRatio: floatValue(b.Info, "allocator_frag_ratio"), AllocatorRSSRatio: floatValue(b.Info, "allocator_rss_ratio"),
			ClientsNormal: intValue(b.Info, "mem_clients_normal"), ClientsReplicas: intValue(b.Info, "mem_clients_slaves"),
			ReplicationBacklog: intValue(b.Info, "mem_replication_backlog"),
			EvictedKeysDelta:   nonnegative(intValue(b.Info, "evicted_keys") - intValue(a.Info, "evicted_keys")),
			ExpiredKeysDelta:   nonnegative(intValue(b.Info, "expired_keys") - intValue(a.Info, "expired_keys")),
		})
		if node.MasterID != "" {
			for _, master := range nodes {
				if master.ID == node.MasterID {
					out[len(out)-1].Master = master.Addr
					break
				}
			}
		}
	}
	return out
}

func parseCSV(value string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(value, ",") {
		k, v, ok := strings.Cut(part, "=")
		if ok {
			out[k] = v
		}
	}
	return out
}
func intValue(m map[string]string, key string) int64 {
	v, _ := strconv.ParseInt(m[key], 10, 64)
	return v
}
func floatValue(m map[string]string, key string) float64 {
	v, _ := strconv.ParseFloat(m[key], 64)
	return v
}
func nonnegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
func anyInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case uint64:
		if x > math.MaxInt64 {
			return math.MaxInt64
		}
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(x), 10, 64)
		return n
	}
	return 0
}
func anyString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}
