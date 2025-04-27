package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend"
	"go.acuvity.ai/minibridge/pkgs/client"
)

var fBackend = pflag.NewFlagSet("backend", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fBackend.StringP("listen", "l", ":8000", "listen address of the bridge for incoming websocket connections.")

	Backend.Flags().AddFlagSet(fBackend)
	Backend.Flags().AddFlagSet(fPolicer)
	Backend.Flags().AddFlagSet(fTLSServer)
	Backend.Flags().AddFlagSet(fHealth)
	Backend.Flags().AddFlagSet(fProfiler)
	Backend.Flags().AddFlagSet(fCORS)
	Backend.Flags().AddFlagSet(fSBOM)
	Backend.Flags().AddFlagSet(fMCP)
}

// Backend is the cobra command to run the server.
var Backend = &cobra.Command{
	Use:              "backend [flags] -- command [args...]",
	Short:            "Start a minibridge backend to expose an MCP server",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	Args:             cobra.MinimumNArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {

		listen := viper.GetString("listen")

		if listen == "" {
			return fmt.Errorf("--listen must be set")
		}

		backendTLSConfig, err := tlsConfigFromFlags(fTLSServer)
		if err != nil {
			return err
		}

		policer, penforce, err := makePolicer()
		if err != nil {
			return fmt.Errorf("unable to make policer: %w", err)
		}

		sbom, err := makeSBOM()
		if err != nil {
			return fmt.Errorf("unable to make hashes: %w", err)
		}

		tracer, err := makeTracer(cmd.Context(), "backend")
		if err != nil {
			return fmt.Errorf("unable to configure tracer: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		clientOpts := makeMCPClientOptions()

		mm := startHealthServer(cmd.Context())

		mcpServer, err := client.NewMCPServer(args[0], args[1:]...)
		if err != nil {
			return err
		}

		slog.Info("MCP server configured",
			"command", mcpServer.Command,
			"args", mcpServer.Args,
		)

		slog.Info("Minibridge backend configured",
			"server-tls", backendTLSConfig != nil,
			"server-mtls", mtlsMode(backendTLSConfig),
			"listen", listen,
		)

		proxy := backend.NewWebSocket(listen, backendTLSConfig, mcpServer,
			backend.OptPolicer(policer),
			backend.OptPolicerEnforce(penforce),
			backend.OptDumpStderrOnError(viper.GetString("log-format") != "json"),
			backend.OptCORSPolicy(corsPolicy),
			backend.OptSBOM(sbom),
			backend.OptClientOptions(clientOpts...),
			backend.OptMetricsManager(mm),
			backend.OptTracer(tracer),
		)

		return proxy.Start(cmd.Context())
	},
}
