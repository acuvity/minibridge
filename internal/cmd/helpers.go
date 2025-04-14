package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"go.acuvity.ai/a3s/pkgs/token"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/tg/tglib"
)

func tlsConfigFromFlags(flags *pflag.FlagSet) (*tls.Config, error) {

	var hasTLS bool

	skipVerify, _ := flags.GetBool("insecure-skip-verify")

	if skipVerify {
		slog.Warn("Certificate validation deactivated. Connection will not be secure")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	var certPath, keyPath, keyPass string

	if flags.Name() == "tlsclient" {
		certPath, _ = flags.GetString("client-cert")
		keyPath, _ = flags.GetString("client-key")
		keyPass, _ = flags.GetString("client-key-pass")
	}

	if flags.Name() == "tlsserver" {
		certPath, _ = flags.GetString("cert")
		keyPath, _ = flags.GetString("key")
		keyPass, _ = flags.GetString("key-pass")
	}

	serverCAPath, _ := flags.GetString("server-ca")
	clientCAPath, _ := flags.GetString("client-ca")

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

type jwtVerifierConfig struct {
	jwksURL        string
	jwksCAPath     string
	jwksSkipVerify bool
	jwtCertPath    string
	reqIss         string
	reqAud         string
	principalClaim string
}

func jwtVerifierConfigFromFlags() jwtVerifierConfig {

	cfg := jwtVerifierConfig{}

	cfg.jwksURL, _ = fJWTVerifier.GetString("jwks-url")
	cfg.jwksCAPath, _ = fJWTVerifier.GetString("jwks-ca")
	cfg.jwksSkipVerify, _ = fJWTVerifier.GetBool("jwks-insecure-skip-verify")
	cfg.jwtCertPath, _ = fJWTVerifier.GetString("jwt-cert")
	cfg.reqIss, _ = fJWTVerifier.GetString("jwt-required-issuer")
	cfg.reqAud, _ = fJWTVerifier.GetString("jwt-required-audience")
	cfg.principalClaim, _ = fJWTVerifier.GetString("jwt-principal-claim")

	if cfg.jwksURL != "" || cfg.jwtCertPath != "" {
		slog.Info("JWT verifier configured",
			"jwks-url", cfg.jwksURL,
			"jwks-custom-ca", cfg.jwksCAPath != "",
			"cert", cfg.jwtCertPath,
			"req-iss", cfg.reqIss,
			"req-aud", cfg.reqAud,
			"principal-claim", cfg.principalClaim,
		)
	} else {
		slog.Info("No JWT verifier configured")
	}

	if cfg.jwksSkipVerify {
		slog.Warn("Security: JWT verifier will trust any jwks server ca. This is VERY insecure.")
	}

	return cfg
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

func makeJWKS(ctx context.Context, cfg jwtVerifierConfig) (jwks *token.JWKS, err error) {

	if cfg.jwtCertPath == "" && cfg.jwksURL == "" {
		return nil, nil
	}

	switch {

	case cfg.jwksURL != "":

		var transport http.RoundTripper

		if cfg.jwksSkipVerify {

			slog.Warn("JWKS TLS Certificate verification disabled. This is highly insecure.")

			transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}

		} else if cfg.jwksCAPath != "" {

			caBytes, err := os.ReadFile(cfg.jwksCAPath)
			if err != nil {
				return nil, fmt.Errorf("unable to load jwks CA from '%s': %w", cfg.jwksCAPath, err)
			}

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caBytes) {
				return nil, fmt.Errorf("unable to append ca to jwks client pool")
			}

			transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: pool,
				},
			}
		}

		if jwks, err = token.NewRemoteJWKS(ctx, &http.Client{Transport: transport}, cfg.jwksURL); err != nil {
			return nil, fmt.Errorf("unable to load remote jwks from '%s': %w", cfg.jwksURL, err)
		}

	case cfg.jwtCertPath != "":

		certs, err := tglib.ParseCertificatePEMs(cfg.jwtCertPath)
		if err != nil {
			return nil, fmt.Errorf("unable parse jwt certificate from '%s': %w", cfg.jwtCertPath, err)
		}

		jwks = token.NewJWKS()
		for _, c := range certs {
			if err := jwks.Append(c); err != nil {
				return nil, fmt.Errorf("unable to append certificate '%s' to JWKS: %w", cfg.jwtCertPath, err)
			}
		}
	}

	return jwks, nil
}
