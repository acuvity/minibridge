package backend

import (
	"net"

	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type wsCfg struct {
	corsPolicy      *bahamut.CORSPolicy
	dumpStderr      bool
	listener        net.Listener
	metricsManager  *metrics.Manager
	policer         policer.Policer
	policerEnforced bool
	sbom            scan.SBOM
	tracer          trace.Tracer
}

func newWSCfg() wsCfg {
	return wsCfg{
		tracer:          noop.NewTracerProvider().Tracer("noop"),
		policerEnforced: true,
	}
}

// Option are options that can be given to NewStdio().
type Option func(*wsCfg)

// OptPolicer sets the Policer to forward the traffic to.
func OptPolicer(policer policer.Policer) Option {
	return func(cfg *wsCfg) {
		cfg.policer = policer
	}
}

// OptPolicerEnforce sets the Policer decision should be enforced
// or just logged. The default is true if a policer is set
func OptPolicerEnforce(enforced bool) Option {
	return func(cfg *wsCfg) {
		cfg.policerEnforced = enforced
	}
}

// OptDumpStderrOnError controls whether the WS server should
// dump the stderr of the MCP server as is, or in a log.
func OptDumpStderrOnError(dump bool) Option {
	return func(cfg *wsCfg) {
		cfg.dumpStderr = dump
	}
}

// OptCORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptCORSPolicy(policy *bahamut.CORSPolicy) Option {
	return func(cfg *wsCfg) {
		cfg.corsPolicy = policy
	}
}

// OptSBOM sets a the utils.SBOM to use to verify
// server integrity.
func OptSBOM(sbom scan.SBOM) Option {
	return func(cfg *wsCfg) {
		cfg.sbom = sbom
	}
}

// OptMetricsManager sets the metric manager to use to collect
// prometheus metrics.
func OptMetricsManager(m *metrics.Manager) Option {
	return func(cfg *wsCfg) {
		cfg.metricsManager = m
	}
}

// OptTracer sets the otel trace.Tracer to use to trace requests
func OptTracer(tracer trace.Tracer) Option {
	return func(cfg *wsCfg) {
		if tracer == nil {
			tracer = noop.NewTracerProvider().Tracer("noop")
		}
		cfg.tracer = tracer
	}
}

// OptListener sets the listener to use for the server.
// by defaut, it will use a classic listener.
func OptListener(listener net.Listener) Option {
	return func(cfg *wsCfg) {
		cfg.listener = listener
	}
}
