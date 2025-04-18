package main

import (
	"fmt"
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
		cmd.Completion,
		cmd.Scan,
	)

	if err := cmd.Root.Execute(); err != nil {
		if _, ok := slog.Default().Handler().(*slog.JSONHandler); ok {
			slog.Error("Minibridge exited with error", "err", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s", err.Error())
		}
		os.Exit(1)
	}
}
