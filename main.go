package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"go.acuvity.ai/minibridge/internal/cmd"
)

func main() {

	cobra.OnInitialize(initCobra)

	cmd.Root.AddCommand(
		cmd.Backend,
		cmd.Frontend,
		cmd.AIO,
	)

	if err := cmd.Root.Execute(); err != nil {
		slog.Error("Minibridge exited with error", err)
		os.Exit(1)
	}
}
