package analyzer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/config"
	"github.com/flyzstu/RedisSleuth/internal/model"
)

func Analyze(cfg config.Config, cpu []model.CPUStats, memory []model.MemoryStats, clients []model.ClientAggregate, keys []model.KeySample, slots []model.SlotStats) ([]model.Finding, []string) {
	now := time.Now()
	var findings []model.Finding
	findings = append(findings, EvaluateMemory(cfg.Thresholds, memory, now)...)
	bigByNode := make(map[string]int)
	noTTL := 0
	for _, key := range keys {
		if key.MemoryBytes >= cfg.Thresholds.BigKeyBytes {
			bigByNode[key.Master]++
			slotValue := key.Slot
			findings = append(findings, model.Finding{
				Time: now, Severity: "medium", Category: "key", Node: key.Master, Slot: &slotValue, Key: key.Key,
				Evidence:   map[string]any{"memory_bytes": key.MemoryBytes, "type": key.Type, "elements": key.Elements, "sampled": true},
				Conclusion: "Key 抽样发现大 Key", Recommendation: "确认访问模式并拆分大 Key", Confidence: "high",
			})
		}
		if key.TTLMillis < 0 {
			noTTL++
		}
	}
	for _, stat := range cpu {
		if stat.CPUPercent < cfg.Thresholds.CPUPercent {
			continue
		}
		findings = append(findings, finding(now, "high", "cpu", stat.Node,
			map[string]any{"cpu_percent": stat.CPUPercent, "ops": stat.OPS},
			"Redis 进程在采样周期内单核 CPU 消耗较高", "检查高频命令、慢命令和业务流量", "high"))
		if stat.OPS > 10000 {
			findings = append(findings, finding(now, "medium", "cpu", stat.Node, map[string]any{"ops": stat.OPS}, "CPU 高且 OPS 较高，可能存在请求量突增", "对照业务 QPS 和历史基线", "medium"))
		}
		if len(stat.CommandDeltas) > 0 && stat.CommandsDelta > 0 {
			top := stat.CommandDeltas[0]
			share := float64(top.CallsDelta) / float64(stat.CommandsDelta) * 100
			if share >= 30 {
				findings = append(findings, finding(now, "high", "command", stat.Node,
					map[string]any{"command": top.Name, "calls_delta": top.CallsDelta, "share_percent": share, "usec_delta": top.UsecDelta},
					fmt.Sprintf("%s 在新增命令中占比较高", top.Name), "检查调用方并优化命令访问模式", "high"))
			}
		}
		if len(stat.Slowlog) > 0 {
			findings = append(findings, finding(now, "high", "slowlog", stat.Node, map[string]any{"new_entries": len(stat.Slowlog)}, "CPU 高且采样窗口出现新慢命令", "检查 SLOWLOG 命令及数据结构大小", "high"))
		}
		if bigByNode[stat.Node] > 0 {
			findings = append(findings, finding(now, "high", "key", stat.Node, map[string]any{"sampled_big_keys": bigByNode[stat.Node]}, "CPU 高节点的抽样中发现大 Key", "避免全量读取并拆分大 Key", "medium"))
		}
		if stat.BGSaveActive || stat.AOFRewriteActive {
			findings = append(findings, finding(now, "medium", "persistence", stat.Node, map[string]any{"bgsave": stat.BGSaveActive, "aof_rewrite": stat.AOFRewriteActive}, "CPU 高期间存在后台持久化", "调整持久化窗口并检查 fork 开销", "high"))
		}
		if stat.FullSyncLikely || stat.ReplicationAbnormal {
			findings = append(findings, finding(now, "high", "replication", stat.Node, map[string]any{"full_sync": stat.FullSyncLikely, "abnormal": stat.ReplicationAbnormal}, "CPU 高期间复制可能异常", "检查复制链路与全量同步原因", "medium"))
		}
	}
	for _, c := range clients {
		if c.Storm {
			findings = append(findings, model.Finding{Time: now, Severity: "high", Category: "client", ClientIP: c.IP,
				Evidence: map[string]any{"connections": c.Connections, "connection_delta": c.ConnectionDelta, "active": c.Active}, Conclusion: "单一客户端 IP 连接数或采样期增量超过阈值，疑似连接集中或连接风暴",
				Recommendation: "检查连接池配置与客户端重连行为", Confidence: "medium"})
		}
	}
	highCPU := false
	for _, stat := range cpu {
		if stat.CPUPercent >= cfg.Thresholds.CPUPercent {
			highCPU = true
			break
		}
	}
	if highCPU {
		for _, c := range clients {
			if c.ConnectionDelta >= cfg.Thresholds.ClientConnectionsPerIP {
				findings = append(findings, model.Finding{Time: now, Severity: "high", Category: "client", ClientIP: c.IP,
					Evidence:   map[string]any{"connection_delta": c.ConnectionDelta, "cpu_high": true},
					Conclusion: "CPU 高期间客户端连接数显著增加，疑似连接风暴", Recommendation: "检查客户端重连退避与连接池配置", Confidence: "high"})
			}
		}
	}
	if len(keys) > 0 && float64(noTTL)/float64(len(keys))*100 >= cfg.Thresholds.NoTTLPercent {
		findings = append(findings, finding(now, "medium", "memory", "", map[string]any{"sample_size": len(keys), "no_ttl": noTTL}, "抽样 Key 中无 TTL 比例较高，数据可能长期堆积", "核对数据生命周期并设置合理 TTL", "medium"))
	}
	for _, s := range slots {
		if s.Skewed {
			findings = append(findings, finding(now, "medium", "slot", s.Master, map[string]any{"keys": s.KeyCount, "slots": s.SlotCount}, "主节点 Key 数高于集群主节点均值，可能存在槽位或业务负载倾斜", "检查 Hash Tag 与槽位迁移方案", "medium"))
		}
	}
	findings = append(findings, cpuDeviation(cfg.Thresholds, cpu, now)...)
	recommendations := uniqueRecommendations(findings)
	return findings, recommendations
}

func EvaluateMemory(t config.Thresholds, stats []model.MemoryStats, now time.Time) []model.Finding {
	var out []model.Finding
	var masters []model.MemoryStats
	for _, m := range stats {
		if m.Role == "master" {
			masters = append(masters, m)
		}
		if m.MemoryPercent >= t.MemoryPercent {
			out = append(out, finding(now, "high", "memory", m.Node, map[string]any{"percent": m.MemoryPercent, "basis": m.MemoryPercentBasis}, "内存利用率超过阈值", "清理无效数据、设置 TTL 或扩容", "high"))
		}
		if m.UsedMemory > 0 && float64(m.UsedMemoryRSS)/float64(m.UsedMemory) >= t.FragmentationRatio {
			out = append(out, finding(now, "medium", "memory", m.Node, map[string]any{"used_memory": m.UsedMemory, "rss": m.UsedMemoryRSS}, "RSS 明显高于 used_memory，可能存在碎片或内存未归还", "评估 active-defrag、重启或滚动迁移", "high"))
		}
		if m.FragmentationRatio >= t.FragmentationRatio {
			out = append(out, finding(now, "medium", "memory", m.Node, map[string]any{"fragmentation_ratio": m.FragmentationRatio}, "内存碎片率过高", "检查分配器碎片并评估 active-defrag", "high"))
		} else if m.FragmentationRatio > 0 && m.FragmentationRatio < 1 {
			out = append(out, finding(now, "high", "memory", m.Node, map[string]any{"fragmentation_ratio": m.FragmentationRatio}, "内存碎片率小于 1，可能发生 Swap", "立即检查主机 Swap 与内存压力", "medium"))
		}
		if m.ClientsNormal >= t.ClientBufferBytes {
			out = append(out, finding(now, "medium", "client", m.Node, map[string]any{"mem_clients_normal": m.ClientsNormal}, "客户端缓冲区占用较高", "检查慢消费者和输出缓冲区限制", "high"))
		}
		if m.ReplicationBacklog >= t.ReplicationBacklogBytes {
			out = append(out, finding(now, "low", "replication", m.Node, map[string]any{"backlog_bytes": m.ReplicationBacklog}, "复制积压缓冲区占用较高", "确认 repl-backlog-size 是否符合故障恢复目标", "high"))
		}
		if m.EvictedKeysDelta > 0 {
			out = append(out, finding(now, "high", "memory", m.Node, map[string]any{"evicted_keys_delta": m.EvictedKeysDelta}, "采样期间发生 Key 淘汰", "检查 maxmemory、淘汰策略和容量水位", "high"))
		}
	}
	if len(masters) > 1 {
		var sum float64
		for _, m := range masters {
			sum += float64(m.UsedMemory)
		}
		avg := sum / float64(len(masters))
		for _, m := range masters {
			if avg == 0 {
				continue
			}
			deviation := math.Abs(float64(m.UsedMemory)-avg) / avg * 100
			if deviation >= t.MasterMemoryDeviationPercent {
				out = append(out, finding(now, "medium", "memory", m.Node, map[string]any{"used_memory": m.UsedMemory, "deviation_percent": deviation}, "主节点内存与主节点均值偏差较大", "检查 Key、Hash Tag 与槽位分布", "high"))
			}
		}
	}
	byNode := make(map[string]model.MemoryStats)
	for _, m := range stats {
		byNode[m.Node] = m
	}
	for _, replica := range stats {
		master, ok := byNode[replica.Master]
		if !ok || master.UsedMemory == 0 {
			continue
		}
		deviation := math.Abs(float64(replica.UsedMemory-master.UsedMemory)) / float64(master.UsedMemory) * 100
		if deviation >= 10 {
			out = append(out, finding(now, "low", "memory", replica.Node,
				map[string]any{"master": master.Node, "master_used_memory": master.UsedMemory, "replica_used_memory": replica.UsedMemory, "deviation_percent": deviation},
				"replica 与 master 的内存占用存在明显差异", "检查复制状态、过期 Key 和客户端缓冲差异", "medium"))
		}
	}
	return out
}

func cpuDeviation(t config.Thresholds, stats []model.CPUStats, now time.Time) []model.Finding {
	var masters []model.CPUStats
	for _, s := range stats {
		if s.Role == "master" {
			masters = append(masters, s)
		}
	}
	if len(masters) < 2 {
		return nil
	}
	var sum float64
	for _, s := range masters {
		sum += s.CPUPercent
	}
	avg := sum / float64(len(masters))
	var out []model.Finding
	for _, s := range masters {
		if avg > 0 && (s.CPUPercent-avg)/avg*100 >= t.MasterCPUDeviationPercent {
			out = append(out, finding(now, "high", "cpu", s.Node, map[string]any{"cpu_percent": s.CPUPercent, "master_average": avg}, "单个 master CPU 明显高于其他 master，可能存在流量或槽位倾斜", "检查业务热点、Hash Tag 和槽位分布", "high"))
		}
	}
	return out
}

func finding(now time.Time, severity, category, node string, evidence map[string]any, conclusion, recommendation, confidence string) model.Finding {
	return model.Finding{Time: now, Severity: severity, Category: category, Node: node, Evidence: evidence, Conclusion: conclusion, Recommendation: recommendation, Confidence: confidence}
}
func uniqueRecommendations(findings []model.Finding) []string {
	seen := map[string]bool{}
	var out []string
	sort.SliceStable(findings, func(i, j int) bool { return severityRank(findings[i].Severity) > severityRank(findings[j].Severity) })
	for _, f := range findings {
		if f.Recommendation != "" && !seen[f.Recommendation] {
			seen[f.Recommendation] = true
			out = append(out, f.Recommendation)
		}
	}
	return out
}
func severityRank(s string) int {
	if s == "high" {
		return 3
	}
	if s == "medium" {
		return 2
	}
	return 1
}
