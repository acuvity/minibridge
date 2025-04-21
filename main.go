package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	sigs := []os.Signal{syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGCHLD, syscall.SIGABRT}
	signalCh := make(chan os.Signal, 1)
	signal.Reset(sigs...)
	signal.Notify(signalCh, sigs...)

	go func() {
		<-signalCh
		cancel()
	}()
}
