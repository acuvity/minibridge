package main

import (
	"context"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cmd.Root.ExecuteContext(ctx); err != nil {
		if _, ok := slog.Default().Handler().(*slog.JSONHandler); ok {
			slog.Error("Minibridge exited with error", "err", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		}
		os.Exit(1)
	}
}
