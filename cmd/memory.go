package cmd

import "github.com/spf13/cobra"

func memoryCmd() *cobra.Command { return command("memory", "采样并分析 Redis 内存", "memory") }
