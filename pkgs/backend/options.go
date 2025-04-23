package backend

import (
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type wsCfg struct {
	policer        policer.Policer
	dumpStderr     bool
	corsPolicy     *bahamut.CORSPolicy
	sbom           scan.SBOM
	clientOpts     []client.Option
	metricsManager *metrics.Manager
	tracer         trace.Tracer
}

func newWSCfg() wsCfg {
	return wsCfg{
		tracer: noop.NewTracerProvider().Tracer("noop"),
	}
}

// OptWS are options that can be given to NewStdio().
type OptWS func(*wsCfg)

// OptPolicer sets the Policer to forward the traffic to.
func OptPolicer(policer policer.Policer) OptWS {
	return func(cfg *wsCfg) {
		cfg.policer = policer
	}
}

// OptDumpStderrOnError controls whether the WS server should
// dump the stderr of the MCP server as is, or in a log.
func OptDumpStderrOnError(dump bool) OptWS {
	return func(cfg *wsCfg) {
		cfg.dumpStderr = dump
	}
}

// OptCORSPolicy sets the bahamut.CORSPolicy to use for
// connection originating from a webrowser.
func OptCORSPolicy(policy *bahamut.CORSPolicy) OptWS {
	return func(cfg *wsCfg) {
		cfg.corsPolicy = policy
	}
}

// OptSBOM sets a the utils.SBOM to use to verify
// server integrity.
func OptSBOM(sbom scan.SBOM) OptWS {
	return func(cfg *wsCfg) {
		cfg.sbom = sbom
	}
}

// OptClientOptions sets the option to pass to the spawned clients
func OptClientOptions(opts ...client.Option) OptWS {
	return func(cfg *wsCfg) {
		cfg.clientOpts = opts
	}
}

// OptMetricsManager sets the metric manager to use to collect
// prometheus metrics.
func OptMetricsManager(m *metrics.Manager) OptWS {
	return func(cfg *wsCfg) {
		cfg.metricsManager = m
	}
}

// OptTracer sets the otel trace.Tracer to use to trace requests
func OptTracer(tracer trace.Tracer) OptWS {
	return func(cfg *wsCfg) {
		if tracer == nil {
			tracer = noop.NewTracerProvider().Tracer("noop")
		}
		cfg.tracer = tracer
	}
}
