package backend

import (
	"context"
	"crypto/tls"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.acuvity.ai/bahamut"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/metrics"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.opentelemetry.io/otel/trace/noop"
)

type fakePolicer struct {
}

func (f fakePolicer) Police(context.Context, api.Request) (*api.MCPCall, error) {
	return nil, nil
}

func (f fakePolicer) Type() string { return "fake" }

func TestThing(t *testing.T) {

	Convey("OptPolicer should work", t, func() {
		cfg := newWSCfg()
		f := fakePolicer{}
		OptPolicer(f)(&cfg)
		So(cfg.policer, ShouldEqual, f)
	})

	Convey("OptDumpStderrOnError", t, func() {
		cfg := newWSCfg()
		OptDumpStderrOnError(true)(&cfg)
		So(cfg.dumpStderr, ShouldBeTrue)
	})

	Convey("OPtCORSPolicy should work", t, func() {
		cfg := newWSCfg()
		p := &bahamut.CORSPolicy{}
		OptCORSPolicy(p)(&cfg)
		So(cfg.corsPolicy, ShouldEqual, p)
	})

	Convey("OptSBOM should work", t, func() {
		cfg := newWSCfg()
		s := scan.SBOM{}
		OptSBOM(s)(&cfg)
		So(cfg.sbom, ShouldEqual, s)
	})

	Convey("OptClientOtions should work", t, func() {
		cfg := newWSCfg()
		opts := []client.Option{client.OptUseTempDir(true)}
		OptClientOptions(opts...)(&cfg)
		So(cfg.clientOpts, ShouldResemble, opts)
	})

	Convey("OptMetricsManager should work", t, func() {
		cfg := newWSCfg()
		mm := &metrics.Manager{}
		OptMetricsManager(mm)(&cfg)
		So(cfg.metricsManager, ShouldEqual, mm)
	})

	Convey("OptTracer should work", t, func() {
		cfg := newWSCfg()
		t := noop.NewTracerProvider().Tracer("test")
		OptTracer(t)(&cfg)
		So(cfg.tracer, ShouldEqual, t)
	})

	Convey("OptTracer with nil should work", t, func() {
		cfg := newWSCfg()
		OptTracer(nil)(&cfg)
		So(cfg.tracer, ShouldHaveSameTypeAs, noop.NewTracerProvider().Tracer("test"))
	})

	Convey("OptListener should work", t, func() {
		cfg := newWSCfg()
		listener := tls.NewListener(nil, nil)
		OptListener(listener)(&cfg)
		So(cfg.listener, ShouldEqual, listener)
	})
}
