package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type Manager struct {
	reqDurationMetric         *prometheus.HistogramVec
	reqTotalMetric            *prometheus.CounterVec
	errorMetric               *prometheus.CounterVec
	tcpConnTotalMetric        prometheus.Counter
	tcpConnCurrentMetric      prometheus.Gauge
	wsConnTotalMetric         prometheus.Counter
	wsConnCurrentMetric       prometheus.Gauge
	policerDurationMetric     *prometheus.HistogramVec
	policerRequestTotalMetric *prometheus.CounterVec

	server *http.Server
}

func NewManager(listen string) *Manager {

	r := prometheus.DefaultRegisterer

	mc := &Manager{

		reqTotalMetric: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "The total number of requests.",
			},
			[]string{"method", "url", "code"},
		),
		reqDurationMetric: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_requests_duration_seconds",
				Help:    "The average duration of the requests",
				Buckets: []float64{0.001, 0.0025, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"method", "url"},
		),
		tcpConnTotalMetric: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "tcp_connections_total",
				Help: "The total number of TCP connection.",
			},
		),
		tcpConnCurrentMetric: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "tcp_connections_current",
				Help: "The current number of TCP connection.",
			},
		),
		wsConnTotalMetric: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "http_ws_connections_total",
				Help: "The total number of ws connection.",
			},
		),
		wsConnCurrentMetric: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_ws_connections_current",
				Help: "The current number of ws connection.",
			},
		),
		errorMetric: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_errors_5xx_total",
				Help: "The total number of 5xx errors.",
			},
			[]string{"trace", "method", "url", "code"},
		),

		policerDurationMetric: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "policer_requests_duration_seconds",
				Help:    "The average duration of the policing requests",
				Buckets: []float64{0.001, 0.0025, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0, 2.5, 5.0, 10.0},
			},
			[]string{"policer_type", "call_type"},
		),
		policerRequestTotalMetric: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "policer_request_total",
				Help: "The total number of policer requests.",
			},
			[]string{"policer_type", "call_type", "decision"},
		),
	}

	r.MustRegister(mc.tcpConnCurrentMetric)
	r.MustRegister(mc.tcpConnTotalMetric)
	r.MustRegister(mc.reqTotalMetric)
	r.MustRegister(mc.reqDurationMetric)
	r.MustRegister(mc.wsConnTotalMetric)
	r.MustRegister(mc.wsConnCurrentMetric)
	r.MustRegister(mc.errorMetric)
	r.MustRegister(mc.policerDurationMetric)
	r.MustRegister(mc.policerRequestTotalMetric)

	mc.server = &http.Server{
		Addr:              listen,
		ReadHeaderTimeout: time.Second,
		Handler:           mc,
	}

	return mc
}

func (c *Manager) Start(ctx context.Context) error {

	errCh := make(chan error, 1)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.server.BaseContext = func(net.Listener) context.Context { return sctx }
	c.server.RegisterOnShutdown(func() { cancel() })

	go func() {
		err := c.server.ListenAndServe()
		if err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("unable to start health server", "err", err)
			}
		}
		errCh <- err
	}()

	select {
	case <-sctx.Done():
	case err := <-errCh:
		return err
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	return c.server.Shutdown(stopCtx)
}

func (c *Manager) MeasureRequest(method string, path string) func(int) time.Duration {

	timer := prometheus.NewTimer(
		prometheus.ObserverFunc(
			func(v float64) {
				c.reqDurationMetric.With(
					prometheus.Labels{
						"method": method,
						"url":    path,
					},
				).Observe(v)
			},
		),
	)

	return func(code int) time.Duration {

		c.reqTotalMetric.With(prometheus.Labels{
			"method": method,
			"url":    path,
			"code":   strconv.Itoa(code),
		}).Inc()

		if code >= http.StatusInternalServerError {

			c.errorMetric.With(prometheus.Labels{
				"method": method,
				"url":    path,
				"code":   strconv.Itoa(code),
			}).Inc()
		}

		return timer.ObserveDuration()
	}
}

func (c *Manager) MeasurePolicer(ptype string, rtype api.CallType) func(allow bool) time.Duration {

	timer := prometheus.NewTimer(
		prometheus.ObserverFunc(
			func(v float64) {
				c.policerDurationMetric.With(
					prometheus.Labels{
						"policer_type": ptype,
						"call_type":    string(rtype),
					},
				).Observe(v)
			},
		),
	)

	return func(allow bool) time.Duration {
		c.policerRequestTotalMetric.With(prometheus.Labels{
			"policer_type": ptype,
			"call_type":    string(rtype),
			"decision": func() string {
				if allow {
					return "allow"
				}
				return "deny"
			}(),
		}).Inc()

		return timer.ObserveDuration()
	}
}

func (c *Manager) RegisterWSConnection() {
	c.wsConnTotalMetric.Inc()
	c.wsConnCurrentMetric.Inc()
}

func (c *Manager) UnregisterWSConnection() {
	c.wsConnCurrentMetric.Dec()
}

func (c *Manager) RegisterTCPConnection() {
	c.tcpConnTotalMetric.Inc()
	c.tcpConnCurrentMetric.Inc()
}

func (c *Manager) UnregisterTCPConnection() {
	c.tcpConnCurrentMetric.Dec()
}

func (c *Manager) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	switch req.URL.Path {

	case "/":
		w.WriteHeader(http.StatusNoContent)

	case "/metrics":
		promhttp.Handler().ServeHTTP(w, req)

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}
