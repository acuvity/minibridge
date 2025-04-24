package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/frontend"
)

var fFrontend = pflag.NewFlagSet("", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fFrontend.StringP("listen", "l", "", "listen address of the bridge for incoming connections. If this is unset, stdio is used.")
	fFrontend.StringP("backend", "A", "", "URL of the minibridge backend to connect to.")
	fFrontend.String("endpoint-messages", "/message", "when using HTTP, sets the endpoint to post messages.")
	fFrontend.String("endpoint-sse", "/sse", "when using HTTP, sets the endpoint to connect to the event stream.")
	fFrontend.BoolP("agent-token-passthrough", "b", false, "forwards incoming HTTP Authorization header to the minibridge backend as-is.")

	Frontend.Flags().AddFlagSet(fFrontend)
	Frontend.Flags().AddFlagSet(fTLSClient)
	Frontend.Flags().AddFlagSet(fTLSServer)
	Frontend.Flags().AddFlagSet(fHealth)
	Frontend.Flags().AddFlagSet(fProfiler)
	Frontend.Flags().AddFlagSet(fCORS)
	Frontend.Flags().AddFlagSet(fAgentAuth)
}

// Frontend is the cobra command to run the client.
var Frontend = &cobra.Command{
	Use:              "frontend",
	Short:            "Start a minibridge frontend to connect to a minibridge backend",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,

	RunE: func(cmd *cobra.Command, args []string) error {

		listen := viper.GetString("listen")
		backendURL := viper.GetString("backend")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")
		agentToken := viper.GetString("agent-token")
		agentTokenPassthrough := viper.GetBool("agent-token-passthrough")

		if backendURL == "" {
			return fmt.Errorf("--backend must be set")
		}

		if agentToken != "" || agentTokenPassthrough {
			slog.Info("Agent authentication configured",
				"agent-token", agentToken != "",
				"agent-token-passthrough", agentTokenPassthrough,
			)
		}

		clientTLSConfig, err := tlsConfigFromFlags(fTLSClient)
		if err != nil {
			return err
		}

		tracer, err := makeTracer(cmd.Context(), "backend")
		if err != nil {
			return fmt.Errorf("unable to configure tracer: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		mm := startHealthServer(cmd.Context())

		var proxy frontend.Frontend

		if listen != "" {

			serverTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			slog.Info("Minibridge frontend configured",
				"backend", backendURL,
				"sse", sseEndpoint,
				"messages", messageEndpoint,
				"mode", "sse",
				"server-tls", serverTLSConfig != nil,
				"server-mtls", mtlsMode(serverTLSConfig),
				"client-tls", clientTLSConfig != nil,
				"listen", listen,
			)

			proxy = frontend.NewSSE(listen, backendURL, serverTLSConfig, clientTLSConfig,
				frontend.OptSSEStreamEndpoint(sseEndpoint),
				frontend.OptSSEMessageEndpoint(messageEndpoint),
				frontend.OptSSEAgentToken(agentToken),
				frontend.OptSSEAgentTokenPassthrough(agentTokenPassthrough),
				frontend.OptSSECORSPolicy(corsPolicy),
				frontend.OptSSEMetricsManager(mm),
				frontend.OptSSETracer(tracer),
			)

		} else {

			slog.Info("Minibridge frontend configured",
				"backend", backendURL,
				"mode", "stdio",
			)

			proxy = frontend.NewStdio(backendURL, clientTLSConfig,
				frontend.OptStioAgentToken(agentToken),
				frontend.OptStdioTracer(tracer),
			)
		}

		return proxy.Start(cmd.Context())
	},
}
