package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/akutz/memconn"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/frontend"
	"golang.org/x/sync/errgroup"
)

var fAIO = pflag.NewFlagSet("aio", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fAIO.StringP("listen", "l", "", "listen address of the bridge for incoming connections. If unset, stdio is used.")
	fAIO.String("endpoint-messages", "/message", "when using HTTP, sets the endpoint to post messages.")
	fAIO.String("endpoint-sse", "/sse", "when using HTTP, sets the endpoint to connect to the event stream.")

	AIO.Flags().AddFlagSet(fAIO)
	AIO.Flags().AddFlagSet(fPolicer)
	AIO.Flags().AddFlagSet(fTLSServer)
	AIO.Flags().AddFlagSet(fHealth)
	AIO.Flags().AddFlagSet(fProfiler)
	AIO.Flags().AddFlagSet(fCORS)
	AIO.Flags().AddFlagSet(fAgentAuth)
	AIO.Flags().AddFlagSet(fSBOM)
	AIO.Flags().AddFlagSet(fMCP)
}

var AIO = &cobra.Command{
	Use:              "aio [flags] -- command [args...]",
	Short:            "Start an all-in-one minibridge frontend and backend",
	Args:             cobra.MinimumNArgs(1),
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,

	RunE: func(cmd *cobra.Command, args []string) (err error) {

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		listen := viper.GetString("listen")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")
		agentToken := viper.GetString("agent-token")

		if agentToken != "" {
			slog.Info("Agent authentication enabled",
				"token", agentToken != "",
			)
		}

		policer, err := makePolicer()
		if err != nil {
			return fmt.Errorf("unable to make policer: %w", err)
		}

		sbom, err := makeSBOM()
		if err != nil {
			return fmt.Errorf("unable to make hashes: %w", err)
		}

		tracer, err := makeTracer(ctx, "aio")
		if err != nil {
			return fmt.Errorf("unable to configure tracer: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		clientOpts := makeMCPClientOptions()

		mm := startHealthServer(ctx)

		var eg errgroup.Group

		eg.Go(func() error {

			defer cancel()

			mcpServer, err := client.NewMCPServer(args[0], args[1:]...)
			if err != nil {
				return fmt.Errorf("unable to create mcp server: %w", err)
			}

			slog.Info("MCP server configured",
				"command", mcpServer.Command,
				"args", mcpServer.Args,
			)

			slog.Info("Minibridge backend configured")

			listener, err := memconn.Listen("memu", "self")
			if err != nil {
				return fmt.Errorf("unable to start memory listener: %w", err)
			}

			proxy := backend.NewWebSocket("self", nil, mcpServer,
				backend.OptListener(listener),
				backend.OptPolicer(policer),
				backend.OptDumpStderrOnError(viper.GetString("log-format") != "json"),
				backend.OptSBOM(sbom),
				backend.OptClientOptions(clientOpts...),
				backend.OptMetricsManager(mm),
				backend.OptTracer(tracer),
			)

			return proxy.Start(ctx)
		})

		eg.Go(func() error {

			defer cancel()

			var proxy frontend.Frontend

			frontendServerTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
				return memconn.DialContext(cmd.Context(), "memu", "self")
			}

			if listen != "" {

				slog.Info("Minibridge frontend configured",
					"sse", sseEndpoint,
					"messages", messageEndpoint,
					"agent-token", agentToken != "",
					"mode", "http",
					"server-tls", frontendServerTLSConfig != nil,
					"server-mtls", mtlsMode(frontendServerTLSConfig),
					"listen", listen,
				)

				proxy = frontend.NewHTTP(listen, "ws://self/ws", frontendServerTLSConfig, nil,
					frontend.OptHTTPBackendDialer(dialer),
					frontend.OptHTTPStreamEndpoint(sseEndpoint),
					frontend.OptHTTPMessageEndpoint(messageEndpoint),
					frontend.OptHTTPAgentToken(agentToken),
					frontend.OptHTTPAgentTokenPassthrough(true),
					frontend.OptHTTPCORSPolicy(corsPolicy),
					frontend.OptHTTPMetricsManager(mm),
					frontend.OptHTTPTracer(tracer),
				)
			} else {

				slog.Info("Minibridge frontend configured",
					"mode", "stdio",
				)

				proxy = frontend.NewStdio("ws://self/ws", nil,
					frontend.OptStdioBackendDialer(dialer),
					frontend.OptStdioRetry(false),
					frontend.OptStioAgentToken(agentToken),
					frontend.OptStdioTracer(tracer),
				)
			}

			time.Sleep(300 * time.Millisecond)
			return proxy.Start(ctx)
		})

		return eg.Wait()
	},
}
