package cmd

import "github.com/spf13/cobra"

func topologyCmd() *cobra.Command {
	return command("topology", "发现并展示 Redis Cluster 拓扑", "topology")
}
