package collector

import (
	"sort"
	"strconv"
	"strings"

	"github.com/flyzstu/RedisSleuth/internal/cluster"
	"github.com/flyzstu/RedisSleuth/internal/model"
)

func Slots(env *cluster.Environment, snapshot Snapshot, samples []model.KeySample) []model.SlotStats {
	var out []model.SlotStats
	var totalKeys int64
	for _, node := range env.Nodes {
		if node.Role != "master" {
			continue
		}
		info := snapshot.Nodes[node.Addr].Info
		keys := keyspaceCount(info)
		item := model.SlotStats{Master: node.Addr, Ranges: node.Slots, SlotCount: countSlots(node.Slots), KeyCount: keys, UsedMemory: intValue(info, "used_memory"), OPS: intValue(info, "instantaneous_ops_per_sec")}
		totalKeys += keys
		bySlot := make(map[int]*model.SlotSample)
		for _, sample := range samples {
			if sample.Master != node.Addr {
				continue
			}
			s := bySlot[sample.Slot]
			if s == nil {
				s = &model.SlotSample{Slot: sample.Slot}
				bySlot[sample.Slot] = s
			}
			s.KeyCount++
			s.MemoryBytes += sample.MemoryBytes
		}
		for _, s := range bySlot {
			item.SampledSlots = append(item.SampledSlots, *s)
		}
		sort.Slice(item.SampledSlots, func(i, j int) bool { return item.SampledSlots[i].MemoryBytes > item.SampledSlots[j].MemoryBytes })
		if len(item.SampledSlots) > env.Config.Sampling.Top {
			item.SampledSlots = item.SampledSlots[:env.Config.Sampling.Top]
		}
		out = append(out, item)
	}
	if len(out) > 0 {
		avg := float64(totalKeys) / float64(len(out))
		for i := range out {
			out[i].Skewed = avg > 0 && float64(out[i].KeyCount) > avg*1.3
		}
	}
	return out
}

func keyspaceCount(info map[string]string) int64 {
	var total int64
	for key, value := range info {
		if !strings.HasPrefix(key, "db") {
			continue
		}
		fields := parseCSV(value)
		n, _ := strconv.ParseInt(fields["keys"], 10, 64)
		total += n
	}
	return total
}
func countSlots(ranges []string) int {
	total := 0
	for _, r := range ranges {
		parts := strings.SplitN(r, "-", 2)
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if len(parts) == 1 {
			total++
			continue
		}
		end, err := strconv.Atoi(parts[1])
		if err == nil && end >= start {
			total += end - start + 1
		}
	}
	return total
}
