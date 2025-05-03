package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.acuvity.ai/tg/tglib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func tlsConfigFromFlags(flags *pflag.FlagSet) (*tls.Config, error) {

	var hasTLS bool

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	var certPath, keyPath, keyPass string
	var skipVerify bool

	if flags.Name() == "tlsclient" {
		certPath = viper.GetString("tls-client-cert")
		keyPath = viper.GetString("tls-client-key")
		keyPass = viper.GetString("tls-client-key-pass")
		skipVerify = viper.GetBool("tls-client-insecure-skip-verify")
	}
	serverCAPath := viper.GetString("tls-client-backend-ca")

	if flags.Name() == "tlsserver" {
		certPath = viper.GetString("tls-server-cert")
		keyPath = viper.GetString("tls-server-key")
		keyPass = viper.GetString("tls-server-key-pass")
	}
	clientCAPath := viper.GetString("tls-server-client-ca")

	if skipVerify {
		slog.Warn("Backend certificates validation deactivated. Connection will not be secure")
		tlsConfig.InsecureSkipVerify = true
		hasTLS = true
	}

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
		data, err := os.ReadFile(serverCAPath) // #nosec: G304
		if err != nil {
			return nil, fmt.Errorf("unable to read trusted ca: %w", err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(data)

		tlsConfig.RootCAs = pool
		hasTLS = true
	}

	if clientCAPath != "" {
		data, err := os.ReadFile(clientCAPath) // #nosec: G304
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

func startHealthServer(ctx context.Context) (manager *metrics.Manager) {

	healthListen := viper.GetString("health-listen")

	if healthListen == "" {
		return nil
	}

	manager = metrics.NewManager(healthListen)

	go func() {
		if err := manager.Start(ctx); err != nil {
			slog.Error("unable to start metric managed: %w", err)
		}
	}()

	slog.Info("Metrics manager configured", "listen", healthListen, "health", "/", "metrics", "/metrics")

	return manager
}

func makePolicer() (policer.Policer, bool, error) {

	pType := viper.GetString("policer-type")
	pEnforce := viper.GetBool("policer-enforce")

	switch pType {

	case "http":

		httpCA := viper.GetString("policer-http-ca")
		httpSkip := viper.GetBool("policer-http-insecure-skip-verify")
		httpURL := viper.GetString("policer-http-url")
		httpUser := viper.GetString("policer-http-basic-user")
		httpPassword := viper.GetString("policer-http-basic-password")
		httpToken := viper.GetString("policer-http-bearer-token")

		if httpURL == "" {
			return nil, false, fmt.Errorf("you must set --policer-http-url when using an http policer")
		}

		if (httpUser != "" && httpPassword == "") || (httpUser == "" && httpPassword != "") {
			return nil, false, fmt.Errorf("you must set both --policer-http-basic-user and --policer-http-auth-basic-password")
		}

		if httpUser != "" && httpToken != "" {
			return nil, false, fmt.Errorf("if you set --policer-http-bearer-token, you can't --policer-http-auth-basic-*")
		}

		var a *auth.Auth
		if httpToken != "" {
			a = auth.NewBearerAuth(httpToken)
		} else {
			a = auth.NewBasicAuth(httpUser, httpPassword)
		}

		var pool *x509.CertPool
		if httpCA != "" {
			caData, err := os.ReadFile(httpCA) // #nosec: G304
			if err != nil {
				return nil, false, fmt.Errorf("unable to read policer CA: %w", err)
			}
			pool.AppendCertsFromPEM(caData)
		} else {
			var err error
			pool, err = x509.SystemCertPool()
			if err != nil {
				return nil, false, fmt.Errorf("unable to load system ca pool: %w", err)
			}
		}

		tlsConfig := &tls.Config{
			InsecureSkipVerify: httpSkip, // #nosec: G402
			RootCAs:            pool,
		}

		slog.Info("Policer configured", "type", "http", "url", httpURL, "enforced", pEnforce, "auth")
		if a != nil {
			slog.Info("Policer auth enabled", "type", a.Type(), "user", a.User(), "password", a.Password() != "")
		}

		return policer.NewHTTP(httpURL, a, tlsConfig), pEnforce, nil

	case "rego":

		regoFile := viper.GetString("policer-rego-policy")

		if regoFile == "" {
			return nil, false, fmt.Errorf("you must set --policer-rego-policy when using a rego policer")
		}

		data, err := os.ReadFile(regoFile) // #nosec: G304
		if err != nil {
			return nil, false, fmt.Errorf("unable open rego policy file: %w", err)
		}

		slog.Info("Policer configured", "type", "rego", "policy", regoFile, "enforced", pEnforce)

		p, err := policer.NewRego(string(data))
		if err != nil {
			return nil, false, err
		}

		return p, pEnforce, nil

	case "":
		return nil, false, nil

	default:
		return nil, false, fmt.Errorf("unknown type of policer: %s", pType)
	}
}

func makeAgentAuth() (a *auth.Auth, err error) {

	user := viper.GetString("agent-user")
	pass := viper.GetString("agent-pass")
	token := viper.GetString("agent-token")

	if (user != "" && pass == "") || (user == "" && pass != "") {
		return a, fmt.Errorf("you must set both --agent-user --agent-pass")
	}

	if user != "" && token != "" {
		return a, fmt.Errorf("if you set --agent-token, you cannot set --agent-user and --agent-pass")
	}

	if token != "" {
		a = auth.NewBearerAuth(token)
	} else if user != "" && pass != "" {
		a = auth.NewBasicAuth(user, pass)
	}

	if a != nil {
		slog.Info("Agent credential configured", "type", a.Type(), "user", a.User(), "password", a.Password() != "")
	}

	return a, nil
}

func makeCORSPolicy() *bahamut.CORSPolicy {

	origin := viper.GetString("cors-origin")

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

func makeSBOM() (scan.SBOM, error) {

	sbomFile := viper.GetString("sbom")

	if sbomFile == "" {
		return scan.SBOM{}, nil
	}

	sbom, err := scan.LoadSBOM(sbomFile)
	if err != nil {
		return sbom, fmt.Errorf("unable load sbom file: %w", err)
	}

	slog.Info("SBOM configured",
		"tools", len(sbom.Tools),
		"prompts", len(sbom.Prompts),
	)

	return sbom, nil
}

func makeMCPClientOptions() []client.Option {

	uid := viper.GetInt("mcp-uid")
	gid := viper.GetInt("mcp-gid")
	groups := viper.GetIntSlice("mcp-groups")
	tmp := viper.GetBool("mcp-use-tempdir")

	opts := []client.Option{
		client.OptUseTempDir(tmp),
	}

	if uid > -1 && gid > -1 || len(groups) > 0 {
		opts = append(opts, client.OptCredentials(uid, gid, groups))
	}

	if uid > -1 || gid > -1 || len(groups) > 0 || tmp {
		slog.Info("MCP server isolation",
			"use-temp", tmp,
			"uid", uid,
			"gid", gid,
			"groups", groups,
		)
	}

	return opts
}

func makeTracer(ctx context.Context, name string) (trace.Tracer, error) {

	var err error
	var exp sdktrace.SpanExporter

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if e := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); e != "" {
		endpoint = e
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(
			resource.NewSchemaless(
				attribute.String("service.name", "minibridge"),
			),
		),
	}

	if endpoint != "" {
		proto := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
		if p := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"); p != "" {
			proto = p
		}

		if proto == "" {
			proto = "http/protobuf"
		}

		if proto == "grpc" {
			exp, err = otlptracegrpc.New(ctx)
		} else {
			exp, err = otlptracehttp.New(ctx)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OTEL %s exporter: %w", proto, err)
		}

		slog.Info("OTEL exporter configured", "proto", proto)
		opts = append(opts, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(opts...)

	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = tp.Shutdown(sctx)
	}()

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Tracer(name), nil
}
