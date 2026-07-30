package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/flyzstu/RedisSleuth/internal/model"
	"github.com/jedib0t/go-pretty/v6/table"
)

func Write(w io.Writer, format string, r model.Report) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(r)
	}
	writeSummary(w, r)
	if len(r.Nodes) > 0 {
		writeNodes(w, r.Nodes)
	}
	if len(r.CPU) > 0 {
		writeCPU(w, r.CPU)
	}
	if len(r.Memory) > 0 {
		writeMemory(w, r.Memory)
	}
	if len(r.Slots) > 0 {
		writeSlots(w, r.Slots)
	}
	if len(r.Clients) > 0 {
		writeClients(w, r.Clients)
	}
	if len(r.ClientDetails) > 0 {
		writeClientDetails(w, r.ClientDetails)
	}
	if len(r.Keys) > 0 {
		writeKeys(w, r.Keys)
	}
	if len(r.Findings) > 0 {
		writeFindings(w, r.Findings, r.Recommendations)
	} else if len(r.CPU)+len(r.Memory)+len(r.Slots)+len(r.Clients) > 0 {
		fmt.Fprintln(w, "\n主要发现：未发现超过当前阈值的明确异常。")
	}
	return nil
}

func writeSummary(w io.Writer, r model.Report) {
	fmt.Fprintln(w, "RedisSleuth 诊断报告")
	fmt.Fprintf(w, "\n采样开始时间：%s\n采样结束时间：%s\n集群状态：%s\n节点数量：%d\n主节点数量：%d\n从节点数量：%d\n",
		r.Metadata.Start.Format("2006-01-02 15:04:05"), r.Metadata.End.Format("2006-01-02 15:04:05"),
		clusterState(r.Cluster.State), r.Cluster.KnownNodes, r.Cluster.Masters, r.Cluster.Replicas)
	if r.Metadata.Calculation != "" {
		fmt.Fprintf(w, "计算口径：%s\n", r.Metadata.Calculation)
	}
}

func newTable(w io.Writer, title string, header table.Row) table.Writer {
	fmt.Fprintf(w, "\n%s\n", title)
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.AppendHeader(header)
	t.SetStyle(table.StyleLight)
	return t
}
func writeNodes(w io.Writer, nodes []model.Node) {
	t := newTable(w, "集群拓扑", table.Row{"节点 ID", "地址", "角色", "主节点 ID", "槽位", "连接", "复制"})
	for _, n := range nodes {
		t.AppendRow(table.Row{short(n.ID), n.Addr, role(n.Role), short(n.MasterID), strings.Join(n.Slots, ","), n.LinkState, n.ReplicationState})
	}
	t.Render()
}
func writeCPU(w io.Writer, stats []model.CPUStats) {
	t := newTable(w, "CPU 分析（采样周期内 Redis 进程单核 CPU 百分比）", table.Row{"节点", "角色", "CPU", "子进程 CPU", "OPS", "命令增量", "Top 高耗命令", "慢日志新增", "后台任务"})
	for _, s := range stats {
		top := ""
		if len(s.CommandDeltas) > 0 {
			c := s.CommandDeltas[0]
			top = fmt.Sprintf("%s %d次/%.2fμs", c.Name, c.CallsDelta, c.UsecPerCall)
		}
		bg := "-"
		if s.BGSaveActive {
			bg = "BGSAVE"
		}
		if s.AOFRewriteActive {
			bg += " AOF重写"
		}
		if s.FullSyncLikely {
			bg += " 全量同步"
		}
		t.AppendRow(table.Row{s.Node, role(s.Role), fmt.Sprintf("%.1f%%", s.CPUPercent), fmt.Sprintf("%.1f%%", s.ChildrenCPUPercent), s.OPS, s.CommandsDelta, top, len(s.Slowlog), strings.TrimSpace(bg)})
	}
	t.Render()
	var commands []table.Row
	var slowEntries []table.Row
	for _, s := range stats {
		for _, c := range s.CommandDeltas {
			commands = append(commands, table.Row{s.Node, c.Name, c.CallsDelta, c.UsecDelta, fmt.Sprintf("%.2f", c.UsecPerCall)})
		}
		for _, entry := range s.Slowlog {
			slowEntries = append(slowEntries, table.Row{s.Node, entry.Time.Format("15:04:05"), entry.Duration, entry.Command, entry.Client})
		}
	}
	if len(commands) > 0 {
		commandTable := newTable(w, "Top 10 高消耗命令", table.Row{"节点", "命令", "调用增量", "耗时增量(μs)", "平均耗时(μs)"})
		commandTable.AppendRows(commands)
		commandTable.Render()
	}
	if len(slowEntries) > 0 {
		slowTable := newTable(w, "采样窗口新增慢命令", table.Row{"节点", "时间", "耗时", "命令", "客户端"})
		slowTable.AppendRows(slowEntries)
		slowTable.Render()
	}
}
func writeMemory(w io.Writer, stats []model.MemoryStats) {
	t := newTable(w, "内存分析", table.Row{"节点", "角色", "利用率/口径", "used_memory", "RSS", "峰值", "碎片率", "客户端缓冲", "复制 backlog", "淘汰增量", "过期增量"})
	for _, s := range stats {
		t.AppendRow(table.Row{s.Node, role(s.Role), fmt.Sprintf("%.1f%%/%s", s.MemoryPercent, s.MemoryPercentBasis), bytes(s.UsedMemory), bytes(s.UsedMemoryRSS), bytes(s.UsedMemoryPeak), fmt.Sprintf("%.2f", s.FragmentationRatio), bytes(s.ClientsNormal), bytes(s.ReplicationBacklog), s.EvictedKeysDelta, s.ExpiredKeysDelta})
	}
	t.Render()
}
func writeSlots(w io.Writer, stats []model.SlotStats) {
	t := newTable(w, "槽位与节点负载", table.Row{"Master", "槽位范围", "槽位数", "Key 数", "内存", "OPS", "倾斜", "抽样热点槽位"})
	for _, s := range stats {
		var hot []string
		for i, v := range s.SampledSlots {
			if i >= 5 {
				break
			}
			hot = append(hot, fmt.Sprintf("%d(%d/%s)", v.Slot, v.KeyCount, bytes(v.MemoryBytes)))
		}
		t.AppendRow(table.Row{s.Master, strings.Join(s.Ranges, ","), s.SlotCount, s.KeyCount, bytes(s.UsedMemory), s.OPS, yesNo(s.Skewed), strings.Join(hot, ",")})
	}
	t.Render()
}
func writeClients(w io.Writer, clients []model.ClientAggregate) {
	t := newTable(w, "客户端 IP 聚合", table.Row{"客户端 IP", "连接数", "连接增量", "活跃", "空闲", "命令分布", "缓冲区", "疑似连接风暴"})
	for _, c := range clients {
		t.AppendRow(table.Row{c.IP, c.Connections, c.ConnectionDelta, c.Active, c.Idle, formatCommands(c.Commands), bytes(c.BufferBytes), yesNo(c.Storm)})
	}
	t.Render()
}
func writeClientDetails(w io.Writer, clients []model.Client) {
	t := newTable(w, "客户端连接明细", table.Row{"IP", "端口", "名称", "age", "idle", "flags", "db", "当前命令", "输入缓冲", "输出缓冲"})
	for _, c := range clients {
		t.AppendRow(table.Row{c.IP, c.Port, c.Name, c.Age, c.Idle, c.Flags, c.DB, c.Command, bytes(c.InputBuf), bytes(c.OutputBuf)})
	}
	t.Render()
}
func writeKeys(w io.Writer, keys []model.KeySample) {
	t := newTable(w, "Key 抽样（非全量）", table.Row{"Key", "类型", "槽位", "内存", "元素数", "TTL(ms)", "Master", "扫描节点", "采样时间"})
	for i, k := range keys {
		if i >= 100 {
			break
		}
		t.AppendRow(table.Row{k.Key, k.Type, k.Slot, bytes(k.MemoryBytes), k.Elements, k.TTLMillis, k.Master, k.ScanNode, k.SampledAt.Format("15:04:05")})
	}
	t.Render()
}
func writeFindings(w io.Writer, findings []model.Finding, recommendations []string) {
	fmt.Fprintln(w, "\n主要发现：")
	for i, f := range findings {
		target := f.Node
		if target == "" {
			target = f.ClientIP
		}
		fmt.Fprintf(w, "%d. [%s/%s] %s %s（置信度：%s）\n", i+1, f.Severity, f.Category, target, f.Conclusion, f.Confidence)
	}
	fmt.Fprintln(w, "\n建议：")
	for i, v := range recommendations {
		fmt.Fprintf(w, "%d. %s\n", i+1, v)
	}
}
func clusterState(v string) string {
	if v == "ok" {
		return "正常"
	}
	if v == "" {
		return "未知"
	}
	return "异常(" + v + ")"
}
func role(v string) string {
	if v == "master" {
		return "主节点"
	}
	return "从节点"
}
func short(v string) string {
	if len(v) > 12 {
		return v[:12]
	}
	return v
}
func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
func bytes(v int64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := int64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(div), "KMGTPE"[exp])
}
func formatCommands(m map[string]int) string {
	type pair struct {
		k string
		v int
	}
	p := make([]pair, 0, len(m))
	for k, v := range m {
		p = append(p, pair{k, v})
	}
	sort.Slice(p, func(i, j int) bool { return p[i].v > p[j].v })
	var s []string
	for i, x := range p {
		if i >= 5 {
			break
		}
		s = append(s, fmt.Sprintf("%s:%d", x.k, x.v))
	}
	return strings.Join(s, ",")
}
