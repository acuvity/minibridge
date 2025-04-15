package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/pflag"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/tg/tglib"
)

func tlsConfigFromFlags(flags *pflag.FlagSet) (*tls.Config, error) {

	var hasTLS bool

	skipVerify, _ := flags.GetBool("tls-client-insecure-skip-verify")

	if skipVerify {
		slog.Warn("Certificate validation deactivated. Connection will not be secure")
		hasTLS = true
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	var certPath, keyPath, keyPass string

	if flags.Name() == "tlsclient" {
		certPath, _ = flags.GetString("tls-client-cert")
		keyPath, _ = flags.GetString("tls-client-key")
		keyPass, _ = flags.GetString("tls-client-key-pass")
	}
	serverCAPath, _ := flags.GetString("tls-client-backend-ca")

	if flags.Name() == "tlsserver" {
		certPath, _ = flags.GetString("tls-server-cert")
		keyPath, _ = flags.GetString("tls-server-key")
		keyPass, _ = flags.GetString("tls-server-key-pass")
	}
	clientCAPath, _ := flags.GetString("tls-server-client-ca")

	if certPath != "" && keyPath != "" {
		x509Cert, x509Key, err := tglib.ReadCertificatePEM(certPath, keyPath, keyPass)
		if err != nil {
			return nil, fmt.Errorf("unable to read server certificate: %w", err)
		}

		tlsCert, err := tglib.ToTLSCertificate(x509Cert, x509Key)
		if err != nil {
			return nil, fmt.Errorf("unable to convert X509 certificate: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{tlsCert}
		hasTLS = true
	}

	if serverCAPath != "" {
		data, err := os.ReadFile(serverCAPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read trusted ca: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(data)

		tlsConfig.RootCAs = pool
		hasTLS = true
	}

	if clientCAPath != "" {
		data, err := os.ReadFile(clientCAPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read client ca: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(data)

		tlsConfig.ClientCAs = pool
		hasTLS = true
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if !hasTLS {
		return nil, nil
	}

	return tlsConfig, nil
}

func startHelperServers(ctx context.Context) bahamut.MetricsManager { // nolint: unparam

	healthEnabled, _ := fHealth.GetBool("health-enable")
	healthListen, _ := fHealth.GetString("health-listen")
	profilingEnabled, _ := fProfiler.GetBool("profiling-enable")
	profilingListen, _ := fProfiler.GetString("profiling-listen")

	opts := []bahamut.Option{}
	var metricsManager bahamut.MetricsManager

	if healthEnabled && healthListen != "" {

		metricsManager = bahamut.NewPrometheusMetricsManager()

		opts = append(opts,
			bahamut.OptHealthServer(healthListen, nil),
			bahamut.OptHealthServerMetricsManager(metricsManager),
		)
	}

	if profilingEnabled && profilingListen != "" {
		opts = append(opts, bahamut.OptProfilingLocal(profilingListen))
	}

	if len(opts) > 0 {
		go bahamut.New(opts...).Run(ctx)
	}

	return metricsManager
}

func makePolicer() (policer.Policer, error) {

	policerCA, _ := fPolicer.GetString("policer-ca")
	policerSkip, _ := fPolicer.GetBool("policer-insecure-skip-verify")
	policerURL, _ := fPolicer.GetString("policer-url")
	policerToken, _ := fPolicer.GetString("policer-token")

	if policerURL == "" {
		return nil, nil
	}

	var pool *x509.CertPool
	if policerCA != "" {
		caData, err := os.ReadFile(policerCA)
		if err != nil {
			return nil, fmt.Errorf("unable to read policer CA: %w", err)
		}
		pool.AppendCertsFromPEM(caData)
	} else {
		var err error
		pool, err = x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("unable to load system ca pool: %w", err)
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: policerSkip,
		RootCAs:            pool,
	}

	return policer.New(policerURL, policerToken, tlsConfig), nil
}

func makeCORSPolicy() *bahamut.CORSPolicy {

	origin, _ := fCORS.GetString("cors-origin")

	if origin == "mirror" {
		origin = bahamut.CORSOriginMirror
	}

	return &bahamut.CORSPolicy{
		AllowOrigin:      origin,
		AllowCredentials: true,
		MaxAge:           1500,
		AllowHeaders: []string{
			"Authorization",
			"Accept",
			"Content-Type",
			"Cache-Control",
			"Cookie",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"OPTIONS",
		},
	}
}

func mtlsMode(tlsCfg *tls.Config) string {

	if tlsCfg == nil {
		return "NoClientCert"
	}

	return tlsCfg.ClientAuth.String()
}
