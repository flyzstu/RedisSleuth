package cmd

import "github.com/spf13/cobra"

func cpuCmd() *cobra.Command { return command("cpu", "采样并分析 Redis CPU", "cpu") }
