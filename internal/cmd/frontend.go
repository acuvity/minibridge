package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/frontend"
)

var fFrontend = pflag.NewFlagSet("", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fFrontend.String("listen", "", "Listen address of the bridge for incoming connections. If this is unset, stdio is used.")
	fFrontend.String("backend", "", "Address of the minibridge backend")
	fFrontend.String("endpoint-messages", "/message", "When using HTTP, sets the endpoint to post messages")
	fFrontend.String("endpoint-sse", "/sse", "When using HTTP, sets the endpoint to connect to the event stream")

	Frontend.Flags().AddFlagSet(fFrontend)
	Frontend.Flags().AddFlagSet(fTLSClient)
	Frontend.Flags().AddFlagSet(fTLSServer)
	Frontend.Flags().AddFlagSet(fHealth)
	Frontend.Flags().AddFlagSet(fProfiler)
}

// Frontend is the cobra command to run the client.
var Frontend = &cobra.Command{
	Use:              "frontend",
	Short:            "Start a secure frontend bridge to minibridge backend",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,

	RunE: func(cmd *cobra.Command, args []string) error {

		listen := viper.GetString("listen")
		backendURL := viper.GetString("backend")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")

		backendTLSConfig, err := tlsConfigFromFlags(fTLSClient)
		if err != nil {
			return err
		}

		startHelperServers(cmd.Context())

		var proxy frontend.Frontend

		if listen != "" {

			frontendTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			slog.Info("Starting frontend",
				"mode", "sse",
				"tls", frontendTLSConfig != nil,
				"backend", backendURL,
				"listen", listen,
				"sse", sseEndpoint,
				"messages", messageEndpoint,
			)

			proxy = frontend.NewSSE(listen, backendURL, frontendTLSConfig, backendTLSConfig,
				frontend.OptSSEStreamEndpoint(sseEndpoint),
				frontend.OptSSEMessageEndpoint(messageEndpoint),
			)

		} else {

			slog.Info("Starting frontend",
				"mode", "stdio",
				"backend", backendURL,
			)

			proxy = frontend.NewStdio(backendURL, backendTLSConfig)
		}

		return proxy.Start(cmd.Context())
	},
}
