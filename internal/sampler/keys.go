package sampler

import (
	"context"
	"sort"
	"time"

	"github.com/flyzstu/RedisSleuth/internal/cluster"
	"github.com/flyzstu/RedisSleuth/internal/mask"
	"github.com/flyzstu/RedisSleuth/internal/model"
	"github.com/flyzstu/RedisSleuth/internal/redisclient"
	"github.com/flyzstu/RedisSleuth/internal/slot"
)

func Keys(ctx context.Context, env *cluster.Environment) []model.KeySample {
	var out []model.KeySample
	period := time.Second / time.Duration(env.Config.Sampling.ScanRate)
	nextAllowed := time.Now()
	for _, master := range env.Nodes {
		if master.Role != "master" || len(out) >= env.Config.Sampling.SampleSize {
			continue
		}
		scanNode, client := scanTarget(env, master)
		if client == nil {
			continue
		}
		cursor := uint64(0)
		for {
			keys, next, err := client.Scan(ctx, cursor, env.Config.Sampling.ScanCount)
			if err != nil {
				break
			}
			for _, key := range keys {
				if len(out) >= env.Config.Sampling.SampleSize {
					break
				}
				if wait := time.Until(nextAllowed); wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						return out
					case <-timer.C:
					}
				}
				nextAllowed = time.Now().Add(period)
				sample, ok := inspect(ctx, client, key, master.Addr, scanNode, env.Config.Output.ShowFullKey)
				if ok {
					out = append(out, sample)
				}
			}
			if len(out) >= env.Config.Sampling.SampleSize || next == 0 {
				break
			}
			cursor = next
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MemoryBytes > out[j].MemoryBytes })
	return out
}

func scanTarget(env *cluster.Environment, master model.Node) (string, redisclient.NodeAPI) {
	for _, node := range env.Nodes {
		if node.Role == "replica" && node.MasterID == master.ID && node.LinkState == "connected" {
			if client := env.Clients[node.Addr]; client != nil {
				return node.Addr, client
			}
		}
	}
	return master.Addr, env.Clients[master.Addr]
}

func inspect(ctx context.Context, client redisclient.NodeAPI, key, master, scanNode string, full bool) (model.KeySample, bool) {
	typ, err := client.Type(ctx, key)
	if err != nil || typ == "none" {
		return model.KeySample{}, false
	}
	ttl, err := client.PTTL(ctx, key)
	if err != nil {
		return model.KeySample{}, false
	}
	memory, err := client.MemoryUsage(ctx, key)
	if err != nil {
		return model.KeySample{}, false
	}
	length, err := client.Length(ctx, typ, key)
	if err != nil {
		length = 0
	}
	display := key
	if !full {
		display = mask.Key(key)
	}
	return model.KeySample{
		Key: display, Type: typ, Slot: slot.Slot(key), MemoryBytes: memory, Elements: length,
		TTLMillis: ttl.Milliseconds(), Master: master, ScanNode: scanNode, SampledAt: time.Now(),
	}, true
}
