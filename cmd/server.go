package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/mcp"
	"go.acuvity.ai/tg/tglib"
)

func init() {
	Server.Flags().String("listen", ":8000", "Listen address of the bridge for incoming connections")
	Server.Flags().String("cert", "", "Path to a certificate for WSS")
	Server.Flags().String("key", "", "Path to the key for the certificate")
	Server.Flags().String("key-pass", "", "Passphrase for the key")
	Server.Flags().String("client-ca", "", "Path to a client CA to validate incoming connections")
	Server.Flags().String("mcp-cmd", "", "Command to launch the MCP server")
	Server.Flags().StringSlice("mcp-arg", nil, "List of argument to pass to the MCP server")
	Server.Flags().Bool("health-enable", false, "If set, start a health server for production deployments")
	Server.Flags().String("health-listen", ":8080", "Listen address of the health server")
	Server.Flags().Bool("profiling-enable", false, "If set, enable profiling server")
	Server.Flags().String("profiling-listen", ":6060", "Listen address of the health server")
}

// Server is the cobra command to run the server.
var Server = &cobra.Command{
	Use:              "server",
	Short:            "Start a secure bridge to an MCP server",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	Args:             cobra.MinimumNArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
			return err
		}
		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		listen := viper.GetString("listen")
		certPath := viper.GetString("cert")
		keyPath := viper.GetString("key")
		keyPass := viper.GetString("key-pass")
		clientCAPath := viper.GetString("client-ca")
		healthEnabled := viper.GetBool("health-enable")
		healthListen := viper.GetString("health-listen")
		profilingEnabled := viper.GetBool("profiling-enable")
		profilingListen := viper.GetString("profiling-listen")

		tlsConfig := &tls.Config{}
		var hasTLS bool

		if certPath != "" && keyPath != "" {
			x509Cert, x509Key, err := tglib.ReadCertificatePEM(certPath, keyPath, keyPass)
			if err != nil {
				return fmt.Errorf("unable to read server certificate: %w", err)
			}

			tlsCert, err := tglib.ToTLSCertificate(x509Cert, x509Key)
			if err != nil {
				return fmt.Errorf("unable to convert X509 certificate: %w", err)
			}

			tlsConfig.Certificates = []tls.Certificate{tlsCert}
			hasTLS = true
		}

		if clientCAPath != "" {
			data, err := os.ReadFile(clientCAPath)
			if err != nil {
				return fmt.Errorf("unable to read client ca: %w", err)
			}
			clientPool := x509.NewCertPool()
			clientPool.AppendCertsFromPEM(data)

			tlsConfig.ClientCAs = clientPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			hasTLS = true
		}

		if !hasTLS {
			tlsConfig = nil
		}

		metricsManager := bahamut.NewPrometheusMetricsManager()
		opts := []bahamut.Option{}

		if healthEnabled && healthListen != "" {
			opts = append(
				opts,
				bahamut.OptHealthServer(healthListen, nil),
				bahamut.OptHealthServerMetricsManager(metricsManager),
			)
		}

		if profilingEnabled && profilingListen != "" {
			opts = append(opts, bahamut.OptProfilingLocal(profilingListen))
		}

		if len(opts) > 0 {
			go bahamut.New(opts...).Run(cmd.Context())
		}

		mcpServer := mcp.Server{Command: args[0], Args: args[1:]}
		slog.Info("WS Server configured", "tls", hasTLS, "listen", listen)
		slog.Info("MCP Server configured", "command", mcpServer.Command, "args", mcpServer.Args)
		proxy := mcp.NewWSProxy(listen, tlsConfig, mcpServer)
		proxy.Start(cmd.Context())

		<-cmd.Context().Done()

		return nil
	},
}
