package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/mcp"
	"go.acuvity.ai/minibridge/mcp/backend"
)

var fBackend = pflag.NewFlagSet("backend", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fBackend.String("listen", ":8000", "Listen address of the bridge for incoming connections")

	Backend.Flags().AddFlagSet(fBackend)
	Backend.Flags().AddFlagSet(fApex)
	Backend.Flags().AddFlagSet(fTLSFrontend)
	Backend.Flags().AddFlagSet(fHealth)
	Backend.Flags().AddFlagSet(fProfiler)
}

// Backend is the cobra command to run the server.
var Backend = &cobra.Command{
	Use:              "backend [flags] -- command [args...]",
	Short:            "Start a minibridge backend for an MCP server",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	Args:             cobra.MinimumNArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {

		listen := viper.GetString("listen")
		apexURL := viper.GetString("apex-url")
		apexToken := viper.GetString("apex-token")

		backendTLSConfig, err := tlsConfigFromFlags(fTLSFrontend)
		if err != nil {
			return err
		}

		apexTLSConfig, err := makeApexTLSConfig()
		if err != nil {
			return err
		}

		startHelperServers(cmd.Context())

		mcpServer := mcp.Server{Command: args[0], Args: args[1:]}

		slog.Info("Backend configured",
			"mode", "ws",
			"command", mcpServer.Command,
			"args", mcpServer.Args,
			"tls", backendTLSConfig != nil,
			"listen", listen,
			"apex", apexURL,
		)

		proxy := backend.NewWebSocket(listen, backendTLSConfig, mcpServer,
			backend.OptWSApexURL(apexURL, apexToken),
			backend.OptWSApexTLSConfig(apexTLSConfig),
		)

		return proxy.Start(cmd.Context())
	},
}
