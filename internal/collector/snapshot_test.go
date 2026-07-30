package collector

import (
	"math"
	"testing"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/model"
)

func TestCPUDelta(t *testing.T) {
	a := Snapshot{Time: time.Unix(0, 0), Nodes: map[string]NodeSnapshot{"n": {Info: map[string]string{
		"used_cpu_sys": "10", "used_cpu_user": "20", "total_commands_processed": "100",
	}, Commands: map[string]CommandCounter{"get": {Calls: 50, Usec: 500}}}}}
	b := Snapshot{Time: time.Unix(10, 0), Nodes: map[string]NodeSnapshot{"n": {Info: map[string]string{
		"used_cpu_sys": "12", "used_cpu_user": "26", "total_commands_processed": "300", "instantaneous_ops_per_sec": "20",
	}, Commands: map[string]CommandCounter{"get": {Calls: 150, Usec: 2500}}}}}
	got := CPU(a, b, []model.Node{{Addr: "n", Role: "master"}})[0]
	if math.Abs(got.CPUPercent-80) > 0.001 {
		t.Fatalf("CPU=%f want 80", got.CPUPercent)
	}
	if got.CommandsDelta != 200 || got.CommandDeltas[0].UsecPerCall != 20 {
		t.Fatalf("%#v", got)
	}
}

func TestParseCommandStats(t *testing.T) {
	got := ParseCommandStats(map[string]string{"cmdstat_hgetall": "calls=12,usec=240,usec_per_call=20.00"})
	if got["hgetall"].Calls != 12 || got["hgetall"].Usec != 240 {
		t.Fatalf("%#v", got)
	}
}
