package analyzer

import (
	"testing"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/config"
	"github.com/flyzstu/RedisSleuth/internal/model"
)

func TestMemoryRules(t *testing.T) {
	th := config.Default().Thresholds
	stats := []model.MemoryStats{{Node: "n1", Role: "master", UsedMemory: 90, UsedMemoryRSS: 180, MemoryPercent: 90, MemoryPercentBasis: "maxmemory", FragmentationRatio: 2, EvictedKeysDelta: 3}}
	got := EvaluateMemory(th, stats, time.Now())
	categories := map[string]bool{}
	for _, f := range got {
		categories[f.Conclusion] = true
	}
	if !categories["内存利用率超过阈值"] || !categories["采样期间发生 Key 淘汰"] || !categories["内存碎片率过高"] {
		t.Fatalf("%#v", got)
	}
}

func TestMemorySwapRule(t *testing.T) {
	got := EvaluateMemory(config.Default().Thresholds, []model.MemoryStats{{Node: "n", FragmentationRatio: .8}}, time.Now())
	if len(got) != 1 || got[0].Severity != "high" {
		t.Fatalf("%#v", got)
	}
}
