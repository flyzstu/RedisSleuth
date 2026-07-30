package collector

import (
	"context"
	"sort"

	"github.com/flyzstu/RedisSleuth/internal/cluster"
	"github.com/flyzstu/RedisSleuth/internal/model"
)

func Clients(ctx context.Context, env *cluster.Environment) ([]model.ClientAggregate, []model.Client, error) {
	byIP := make(map[string]*model.ClientAggregate)
	var details []model.Client
	for _, client := range env.Clients {
		raw, err := client.ClientList(ctx)
		if err != nil {
			continue
		}
		clients, err := cluster.ParseClientList(raw)
		if err != nil {
			return nil, nil, err
		}
		details = append(details, clients...)
		for _, c := range clients {
			item := byIP[c.IP]
			if item == nil {
				item = &model.ClientAggregate{IP: c.IP, Commands: make(map[string]int)}
				byIP[c.IP] = item
			}
			item.Connections++
			if c.Idle <= 10 {
				item.Active++
			} else {
				item.Idle++
			}
			item.Commands[c.Command]++
			item.BufferBytes += c.InputBuf + c.OutputBuf
		}
	}
	out := make([]model.ClientAggregate, 0, len(byIP))
	for _, item := range byIP {
		item.Storm = item.Connections >= env.Config.Thresholds.ClientConnectionsPerIP
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Connections > out[j].Connections })
	sort.Slice(details, func(i, j int) bool {
		if details[i].IP == details[j].IP {
			return details[i].Port < details[j].Port
		}
		return details[i].IP < details[j].IP
	})
	return out, details, nil
}
