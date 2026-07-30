package cmd

import "github.com/spf13/cobra"

func clientsCmd() *cobra.Command { return command("clients", "聚合分析客户端连接", "clients") }
