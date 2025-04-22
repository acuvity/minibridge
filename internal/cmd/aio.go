package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/frontend"
	"go.acuvity.ai/tg/tglib"
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

		backendTLSConfig, trustPool, err := makeTempTLSConfig()
		if err != nil {
			return err
		}

		policer, err := makePolicer()
		if err != nil {
			return fmt.Errorf("unable to make policer: %w", err)
		}

		frontendClientTLSConfig, err := tlsConfigFromFlags(fTLSClient)
		if err != nil {
			return err
		}
		if frontendClientTLSConfig == nil {
			frontendClientTLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}

		frontendClientTLSConfig.RootCAs = trustPool

		sbom, err := makeSBOM()
		if err != nil {
			return fmt.Errorf("unable to make hashes: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		clientOpts := makeMCPClientOptions()

		startHelperServers(ctx)

		iport, err := randomFreePort()
		if err != nil {
			return fmt.Errorf("unable to find local free port")
		}
		backendURL := fmt.Sprintf("wss://127.0.0.1:%d/ws", iport)
		slog.Debug("Found internal free port", "port", iport)

		var eg errgroup.Group

		eg.Go(func() error {

			defer cancel()

			mcpServer, err := client.NewMCPServer(args[0], args[1:]...)
			if err != nil {
				return err
			}

			slog.Info("MCP server configured",
				"command", mcpServer.Command,
				"args", mcpServer.Args,
			)

			slog.Info("Minibridge backend configured")

			proxy := backend.NewWebSocket(fmt.Sprintf("127.0.0.1:%d", iport), backendTLSConfig, mcpServer,
				backend.OptWSPolicer(policer),
				backend.OptWSDumpStderrOnError(viper.GetString("log-format") != "json"),
				backend.OptSBOM(sbom),
				backend.OptClientOptions(clientOpts...),
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

			if listen != "" {

				slog.Info("Minibridge frontend configured",
					"sse", sseEndpoint,
					"messages", messageEndpoint,
					"agent-token", agentToken != "",
					"mode", "sse",
					"server-tls", frontendServerTLSConfig != nil,
					"server-mtls", mtlsMode(frontendServerTLSConfig),
					"client-tls", frontendClientTLSConfig != nil,
					"listen", listen,
				)

				proxy = frontend.NewSSE(listen, backendURL, frontendServerTLSConfig, frontendClientTLSConfig,
					frontend.OptSSEStreamEndpoint(sseEndpoint),
					frontend.OptSSEMessageEndpoint(messageEndpoint),
					frontend.OptSSEAgentToken(agentToken),
					frontend.OptSSEAgentTokenPassthrough(true),
					frontend.OptSSECORSPolicy(corsPolicy),
				)
			} else {

				slog.Info("Minibridge frontend configured",
					"mode", "stdio",
				)

				proxy = frontend.NewStdio(backendURL, frontendClientTLSConfig,
					frontend.OptStdioRetry(false),
					frontend.OptStioAgentToken(agentToken),
				)
			}

			time.Sleep(300 * time.Millisecond)
			return proxy.Start(ctx)
		})

		return eg.Wait()
	},
}

func makeTempTLSConfig() (*tls.Config, *x509.CertPool, error) {

	cert, key, err := tglib.Issue(
		pkix.Name{},
		tglib.OptIssueIPSANs(net.IP{127, 0, 0, 1}),
		tglib.OptIssueTypeServerAuth(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate internal certificate")
	}

	x509ServerKey, err := tglib.PEMToKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse server key pem: %w", err)
	}
	x509ServerCert, err := tglib.ParseCertificate(pem.EncodeToMemory(cert))
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse server cert: %w", err)
	}

	tlsServerCert, err := tglib.ToTLSCertificate(x509ServerCert, x509ServerKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to convert server cert to tls cert: %w", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(x509ServerCert)

	return &tls.Config{
		Certificates: []tls.Certificate{tlsServerCert},
		MinVersion:   tls.VersionTLS13,
	}, pool, nil
}

func randomFreePort() (int, error) {

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil
}
