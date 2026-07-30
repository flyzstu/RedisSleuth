package cmd

import "github.com/spf13/cobra"

func slotsCmd() *cobra.Command { return command("slots", "分析槽位与 Key 抽样", "slots") }
