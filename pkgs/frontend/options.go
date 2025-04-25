package frontend

import (
	"context"
	"net"

	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type sseCfg struct {
	sseEndpoint           string
	messagesEndpoint      string
	agentTokenPassthrough bool
	agentToken            string
	corsPolicy            *bahamut.CORSPolicy
	metricsManager        *metrics.Manager
	tracer                trace.Tracer
	backendDialer         func(ctx context.Context, network, addr string) (net.Conn, error)
}

func newSSECfg() sseCfg {
	return sseCfg{
		sseEndpoint:      "/sse",
		messagesEndpoint: "/message",
		tracer:           noop.NewTracerProvider().Tracer("noop"),
	}
}

// OptSSE are options that can be given to NewSSE().
type OptSSE func(*sseCfg)

// OptSSEStreamEndpoint sets the sse endpoint
// where agents can connect to the response stream.
// Defaults to /sse
func OptSSEStreamEndpoint(ep string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.sseEndpoint = ep
	}
}

// OptSSECORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptSSECORSPolicy(policy *bahamut.CORSPolicy) OptSSE {
	return func(cfg *sseCfg) {
		cfg.corsPolicy = policy
	}
}

// OptSSEMessageEndpoint sets the message endpoint
// where agents can post request.
// Defaults to /messages
func OptSSEMessageEndpoint(ep string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.messagesEndpoint = ep
	}
}

// OptSSEAgentToken sets the token to send to the minibridge
// backend in order to authenticate the agent sending a request though
// the minibridge frontend.
func OptSSEAgentToken(tokenString string) OptSSE {
	return func(cfg *sseCfg) {
		cfg.agentToken = tokenString
	}
}

// OptSSEAgentTokenPassthrough decides if the HTTP request Authorization header
// should be passed as-is to the minibridge backend.
func OptSSEAgentTokenPassthrough(passthrough bool) OptSSE {
	return func(cfg *sseCfg) {
		cfg.agentTokenPassthrough = passthrough
	}
}

// OptSSEMetricsManager sets the metric manager to use to collect
// prometheus metrics.
func OptSSEMetricsManager(m *metrics.Manager) OptSSE {
	return func(cfg *sseCfg) {
		cfg.metricsManager = m
	}
}

// OptSSETracer sets the otel trace.Tracer to use to trace requests
func OptSSETracer(tracer trace.Tracer) OptSSE {
	return func(cfg *sseCfg) {
		if tracer == nil {
			tracer = noop.NewTracerProvider().Tracer("noop")
		}
		cfg.tracer = tracer
	}
}

// OptSSEBackendDialer sets the dialer to use to connect to the backend.
func OptSSEBackendDialer(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) OptSSE {
	return func(cfg *sseCfg) {
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
