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
	"go.acuvity.ai/minibridge/mcp/frontend"
	"go.acuvity.ai/tg/tglib"
)

func init() {
	Frontend.Flags().String("listen", "", "Listen address of the bridge for incoming connections. If this is unset, stdio is used.")
	Frontend.Flags().String("backend", "", "Address of the minibridge backend")
	Frontend.Flags().String("cert", "", "Path to a client certificate for WSS")
	Frontend.Flags().String("key", "", "Path to the key for the client certificate")
	Frontend.Flags().String("key-pass", "", "Passphrase for the key")
	Frontend.Flags().String("ca", "", "Path to a CA to validate server connections")
	Frontend.Flags().Bool("insecure-skip-verify", false, "If set, don't validate server's CA. Do not do this.")
	Frontend.Flags().Bool("health-enable", false, "If set, start a health server for production deployments")
	Frontend.Flags().String("health-listen", ":8080", "Listen address of the health server")
	Frontend.Flags().Bool("profiling-enable", false, "If set, enable profiling server")
	Frontend.Flags().String("profiling-listen", ":6060", "Listen address of the health server")
	Frontend.Flags().String("endpoint-messages", "/messages", "When using HTTP, sets the endpoint to post messages")
	Frontend.Flags().String("endpoint-sse", "/sse", "When using HTTP, sets the endpoint to connect to the event stream")
}

// Frontend is the cobra command to run the client.
var Frontend = &cobra.Command{
	Use:              "frontend",
	Short:            "Start a secure frontend bridge to minibridge backend",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
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
		backendURL := viper.GetString("backend")
		certPath := viper.GetString("cert")
		keyPath := viper.GetString("key")
		keyPass := viper.GetString("key-pass")
		caPath := viper.GetString("client-ca")
		skipVerify := viper.GetBool("insecure-skip-verify")
		healthEnabled := viper.GetBool("health-enable")
		healthListen := viper.GetString("health-listen")
		profilingEnabled := viper.GetBool("profiling-enable")
		profilingListen := viper.GetString("profiling-listen")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")

		tlsConfig := &tls.Config{
			InsecureSkipVerify: skipVerify,
		}

		if skipVerify {
			slog.Warn("Server certificate validation deactivated. Connection will not be secure")
		}

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
		}

		if caPath != "" {
			data, err := os.ReadFile(caPath)
			if err != nil {
				return fmt.Errorf("unable to read server ca: %w", err)
			}
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(data)

			tlsConfig.RootCAs = pool
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

		var proxy frontend.Frontend

		if listen != "" {
			slog.Info("Starting SSE Frontend", "backend", backendURL, "listen", listen, "sse", sseEndpoint, "messages", messageEndpoint)
			proxy = frontend.NewSSE(listen, backendURL, tlsConfig,
				frontend.SSEOptionSSEEndpoint(sseEndpoint),
				frontend.SSEOptionMessageEndpoint(messageEndpoint),
			)
		} else {
			slog.Info("Starting Stdio Frontend", "backend", backendURL)
			proxy = frontend.NewStdio(backendURL, tlsConfig)
		}

		return proxy.Start(cmd.Context())
	},
}
