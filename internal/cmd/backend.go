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

		policer, err := makePolicer()
		if err != nil {
			return fmt.Errorf("unable to make policer: %w", err)
		}

		sbom, err := makeSBOM()
		if err != nil {
			return fmt.Errorf("unable to make hashes: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		startHelperServers(cmd.Context())

		mcpServer := client.MCPServer{Command: args[0], Args: args[1:]}
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
			backend.OptWSPolicer(policer),
			backend.OptWSDumpStderrOnError(viper.GetString("log-format") != "json"),
			backend.OptWSCORSPolicy(corsPolicy),
			backend.OptSBOM(sbom),
		)

		return proxy.Start(cmd.Context())
	},
}
