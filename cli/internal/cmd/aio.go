package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend"
	"go.acuvity.ai/minibridge/pkgs/frontend"
	"go.acuvity.ai/minibridge/pkgs/memconn"
	"golang.org/x/sync/errgroup"
)

var fAIO = pflag.NewFlagSet("aio", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fAIO.StringP("listen", "l", "", "listen address of the bridge for incoming connections. If unset, stdio is used.")
	fAIO.String("endpoint-mcp", "/mcp", "when using HTTP, sets the endpoint to send messages (proto 2025-03-26).")
	fAIO.String("endpoint-messages", "/message", "when using HTTP, sets the endpoint to post messages (proto 2024-11-05).")
	fAIO.String("endpoint-sse", "/sse", "when using HTTP, sets the endpoint to connect to the event stream (proto 2024-11-05).")

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
		mcpEndpoint := viper.GetString("endpoint-mcp")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")

		auth, err := makeAgentAuth()
		if err != nil {
			return fmt.Errorf("unable to build auth: %w", err)
		}

		policer, penforce, err := makePolicer()
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

		mcpClient, err := makeMCPClient(args)
		if err != nil {
			return fmt.Errorf("unable to create MCP client: %w", err)
		}

		mm := startHealthServer(ctx)

		listener := memconn.NewListener()
		defer func() { _ = listener.Close() }()

		var eg errgroup.Group

		eg.Go(func() error {

			defer cancel()

			slog.Info("Minibridge backend configured")

			proxy := backend.NewWebSocket("self", nil, mcpClient,
				backend.OptListener(listener),
				backend.OptPolicer(policer),
				backend.OptPolicerEnforce(penforce),
				backend.OptDumpStderrOnError(viper.GetString("log-format") != "json"),
				backend.OptSBOM(sbom),
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
				return listener.DialContext(cmd.Context(), "127.0.0.1:443")
			}

			if listen != "" {

				slog.Info("Minibridge frontend configured",
					"mcp", mcpEndpoint,
					"sse", sseEndpoint,
					"messages", messageEndpoint,
					"agent-token", auth != nil,
					"mode", "http",
					"server-tls", frontendServerTLSConfig != nil,
					"server-mtls", mtlsMode(frontendServerTLSConfig),
					"listen", listen,
				)

				proxy = frontend.NewHTTP(listen, "ws://self/ws", frontendServerTLSConfig, nil,
					frontend.OptHTTPBackendDialer(dialer),
					frontend.OptHTTPMCPEndpoint(mcpEndpoint),
					frontend.OptHTTPSSEEndpoint(sseEndpoint),
					frontend.OptHTTPMessageEndpoint(messageEndpoint),
					frontend.OptHTTPAgentAuth(auth),
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
					frontend.OptStioAgentAuth(auth),
					frontend.OptStdioTracer(tracer),
				)
			}

			time.Sleep(300 * time.Millisecond)
			return proxy.Start(ctx)
		})

		return eg.Wait()
	},
}
