package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"go.acuvity.ai/minibridge/cmd"
)

var (
	cfgFile  string
	cfgName  string
	logLevel string
)

func main() {

	cobra.OnInitialize(initCobra)

	cmd.Root.AddCommand(
		cmd.Server,
	)

	if err := cmd.Root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", color.RedString("error"), err)
		os.Exit(1)
	}
}
