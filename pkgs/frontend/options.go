package frontend

import (
	"context"
	"net"

	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type httpCfg struct {
	sseEndpoint           string
	messagesEndpoint      string
	agentTokenPassthrough bool
	agentToken            string
	corsPolicy            *bahamut.CORSPolicy
	metricsManager        *metrics.Manager
	tracer                trace.Tracer
	backendDialer         func(ctx context.Context, network, addr string) (net.Conn, error)
}

func newHTTPCfg() httpCfg {
	return httpCfg{
		sseEndpoint:      "/sse",
		messagesEndpoint: "/message",
		tracer:           noop.NewTracerProvider().Tracer("noop"),
	}
}

// OptHTTP are options that can be given to NewSSE().
type OptHTTP func(*httpCfg)

// OptHTTPStreamEndpoint sets the sse endpoint
// where agents can connect to the response stream.
// Defaults to /sse
func OptHTTPStreamEndpoint(ep string) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.sseEndpoint = ep
	}
}

// OptHTTPCORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptHTTPCORSPolicy(policy *bahamut.CORSPolicy) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.corsPolicy = policy
	}
}

// OptHTTPMessageEndpoint sets the message endpoint
// where agents can post request.
// Defaults to /messages
func OptHTTPMessageEndpoint(ep string) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.messagesEndpoint = ep
	}
}

// OptHTTPAgentToken sets the token to send to the minibridge
// backend in order to authenticate the agent sending a request though
// the minibridge frontend.
func OptHTTPAgentToken(tokenString string) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.agentToken = tokenString
	}
}

// OptHTTPAgentTokenPassthrough decides if the HTTP request Authorization header
// should be passed as-is to the minibridge backend.
func OptHTTPAgentTokenPassthrough(passthrough bool) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.agentTokenPassthrough = passthrough
	}
}

// OptHTTPMetricsManager sets the metric manager to use to collect
// prometheus metrics.
func OptHTTPMetricsManager(m *metrics.Manager) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.metricsManager = m
	}
}

// OptHTTPTracer sets the otel trace.Tracer to use to trace requests
func OptHTTPTracer(tracer trace.Tracer) OptHTTP {
	return func(cfg *httpCfg) {
		if tracer == nil {
			tracer = noop.NewTracerProvider().Tracer("noop")
		}
		cfg.tracer = tracer
	}
}

// OptHTTPBackendDialer sets the dialer to use to connect to the backend.
func OptHTTPBackendDialer(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) OptHTTP {
	return func(cfg *httpCfg) {
		cfg.backendDialer = dialer
	}
}

type stdioCfg struct {
	retry         bool
	agentToken    string
	tracer        trace.Tracer
	backendDialer func(ctx context.Context, network, addr string) (net.Conn, error)
}

func newStdioCfg() stdioCfg {
	return stdioCfg{
		retry:  true,
		tracer: noop.NewTracerProvider().Tracer("noop"),
	}
}

// OptStdio are options that can be given to NewStdio().
type OptStdio func(*stdioCfg)

// OptStdioRetry allows to control if the Stdio frontend
// should retry or not after a wbesocket connection failure.
func OptStdioRetry(retry bool) OptStdio {
	return func(cfg *stdioCfg) {
		cfg.retry = retry
	}
}

// OptStdioAgentToken sets the token to send to the minibridge
// backend in order to authenticate the agent using the standard input.
func OptStioAgentToken(tokenString string) OptStdio {
	return func(cfg *stdioCfg) {
		cfg.agentToken = tokenString
	}
}

// OptStdioTracer sets the otel trace.Tracer to use to trace requests
func OptStdioTracer(tracer trace.Tracer) OptStdio {
	return func(cfg *stdioCfg) {
		if tracer == nil {
			tracer = noop.NewTracerProvider().Tracer("noop")
		}
		cfg.tracer = tracer
	}
}

// OptStdioBackendDialer sets the dialer to use to connect to the backend.
func OptStdioBackendDialer(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) OptStdio {
	return func(cfg *stdioCfg) {
		cfg.backendDialer = dialer
	}
}
