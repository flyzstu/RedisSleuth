package cmd

import "github.com/spf13/cobra"

func diagnoseCmd() *cobra.Command { return command("diagnose", "执行统一中文诊断", "diagnose") }
