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

	fFrontend.StringP("listen", "l", "", "Listen address of the bridge for incoming connections. If this is unset, stdio is used.")
	fFrontend.StringP("backend", "A", "", "Address of the minibridge backend")
	fFrontend.String("endpoint-messages", "/message", "When using HTTP, sets the endpoint to post messages")
	fFrontend.String("endpoint-sse", "/sse", "When using HTTP, sets the endpoint to connect to the event stream")
	fFrontend.StringP("agent-token", "t", "", "The user token to pass inline to the minibridge backend to identify the agent that will be passed to the policer. You must use sse server by setting --listen and configure tls for communications with minibridghe backend")
	fFrontend.BoolP("agent-token-passthrough", "b", false, "If set, the HTTP Authorization header of the incoming agent request will be forwarded as-is to the minibridge backend for agent identification")

	Frontend.Flags().AddFlagSet(fFrontend)
	Frontend.Flags().AddFlagSet(fTLSClient)
	Frontend.Flags().AddFlagSet(fTLSServer)
	Frontend.Flags().AddFlagSet(fHealth)
	Frontend.Flags().AddFlagSet(fProfiler)
	Frontend.Flags().AddFlagSet(fCORS)
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
		agentToken := viper.GetString("agent-token")
		agentTokenPassthrough := viper.GetBool("agent-token-passthrough")

		if agentToken != "" || agentTokenPassthrough {
			slog.Info("Agent authentication enabled",
				"token", agentToken != "",
				"passthrough", agentTokenPassthrough,
			)
		}

		clientTLSConfig, err := tlsConfigFromFlags(fTLSClient)
		if err != nil {
			return err
		}

		corsPolicy := makeCORSPolicy()

		startHelperServers(cmd.Context())

		var proxy frontend.Frontend

		if listen != "" {

			serverTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			slog.Info("Starting frontend",
				"mode", "sse",
				"tls", serverTLSConfig != nil,
				"backend", backendURL,
				"listen", listen,
				"sse", sseEndpoint,
				"messages", messageEndpoint,
				"agent-token", agentToken != "",
				"agent-token-passthrough", agentTokenPassthrough,
			)

			proxy = frontend.NewSSE(listen, backendURL, serverTLSConfig, clientTLSConfig,
				frontend.OptSSEStreamEndpoint(sseEndpoint),
				frontend.OptSSEMessageEndpoint(messageEndpoint),
				frontend.OptSSEAgentToken(agentToken),
				frontend.OptSSEAgentTokenPassthrough(agentTokenPassthrough),
				frontend.OptSSECORSPolicy(corsPolicy),
			)

		} else {

			slog.Info("Starting frontend",
				"mode", "stdio",
				"backend", backendURL,
			)

			proxy = frontend.NewStdio(backendURL, clientTLSConfig,
				frontend.OptStioAgentToken(agentToken),
			)
		}

		return proxy.Start(cmd.Context())
	},
}
