package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.acuvity.ai/minibridge/cli/internal/cmd"
)

// Main is the main run for the cli.
func Main(ctx context.Context) {

	cobra.OnInitialize(initCobra)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	installSIGINTHandler(cancel)

	if err := cmd.Root.ExecuteContext(ctx); err != nil {
		if _, ok := slog.Default().Handler().(*slog.JSONHandler); ok {
			slog.Error("Minibridge exited with error", "err", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		}
		os.Exit(1)
	}
}

func installSIGINTHandler(cancel context.CancelFunc) {

	sigs := []os.Signal{syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGABRT}
	signalCh := make(chan os.Signal, 1)
	signal.Reset(sigs...)
	signal.Notify(signalCh, sigs...)

	go func() {
		<-signalCh
		cancel()
	}()
}
