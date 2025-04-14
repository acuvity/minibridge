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

	fAIO.String("listen", "", "Listen address of the bridge for incoming connections. If this is unset, stdio is used.")
	fAIO.String("endpoint-messages", "/message", "When using HTTP, sets the endpoint to post messages")
	fAIO.String("endpoint-sse", "/sse", "When using HTTP, sets the endpoint to connect to the event stream")

	AIO.Flags().AddFlagSet(fAIO)
	AIO.Flags().AddFlagSet(fPolicer)
	AIO.Flags().AddFlagSet(fTLSServer)
	AIO.Flags().AddFlagSet(fHealth)
	AIO.Flags().AddFlagSet(fProfiler)
}

var AIO = &cobra.Command{
	Use:              "aio [flags] -- command [args...]",
	Short:            "Start both frontend and backend in the same process",
	Args:             cobra.MinimumNArgs(1),
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,

	RunE: func(cmd *cobra.Command, args []string) error {

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		cmd.SetContext(ctx)

		listen := viper.GetString("listen")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")
		policerURL := viper.GetString("policer-url")

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
			frontendClientTLSConfig = &tls.Config{}
		}

		frontendClientTLSConfig.RootCAs = trustPool

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

			mcpServer := client.MCPServer{Command: args[0], Args: args[1:]}

			slog.Info("Starting backend",
				"command", mcpServer.Command,
				"args", mcpServer.Args,
				"policer", policerURL,
			)

			proxy := backend.NewWebSocket(fmt.Sprintf("127.0.0.1:%d", iport), backendTLSConfig, mcpServer,
				backend.OptWSPolicer(policer),
			)

			return proxy.Start(ctx)
		})

		eg.Go(func() error {

			defer cancel()

			var proxy frontend.Frontend

			frontendTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			if listen != "" {

				slog.Info("Starting frontend",
					"mode", "sse",
					"tls", frontendTLSConfig != nil,
					"listen", listen,
					"sse", sseEndpoint,
					"messages", messageEndpoint,
				)

				proxy = frontend.NewSSE(listen, backendURL, frontendClientTLSConfig, frontendTLSConfig,
					frontend.OptSSEStreamEndpoint(sseEndpoint),
					frontend.OptSSEMessageEndpoint(messageEndpoint),
				)
			} else {

				slog.Info("Starting frontend",
					"mode", "stdio",
				)

				proxy = frontend.NewStdio(backendURL, frontendTLSConfig,
					frontend.OptStdioRetry(false),
				)
			}

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
