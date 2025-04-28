package backend

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/gorilla/websocket"
	"github.com/karlseguin/ccache/v3"
	"github.com/smallnest/ringbuffer"
	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/internal/cors"
	"go.acuvity.ai/minibridge/pkgs/internal/sanitize"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.acuvity.ai/wsc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type wsBackend struct {
	cfg       wsCfg
	mcpServer client.MCPServer
	server    *http.Server
	listen    string
	tlsConfig *tls.Config
}

// NewWebSocket retrurns a new backend.Backend exposing a Websocket to communicate
// with the given mcp.Server. It will use the given *tls.Config for everything TLS.
// It tls.Config is nil, the server will run as plain HTTP.
func NewWebSocket(listen string, tlsConfig *tls.Config, mcpServer client.MCPServer, opts ...Option) Backend {

	cfg := newWSCfg()
	for _, o := range opts {
		o(&cfg)
	}

	p := &wsBackend{
		mcpServer: mcpServer,
		cfg:       cfg,
		listen:    listen,
		tlsConfig: tlsConfig,
	}

	p.server = &http.Server{
		Handler:           otelhttp.NewHandler(http.HandlerFunc(p.ServeHTTP), "backend"),
		ReadHeaderTimeout: time.Second,
	}

	return p
}

// Start starts the server and will block until the given
// context is canceled.
func (p *wsBackend) Start(ctx context.Context) (err error) {

	errCh := make(chan error, 1)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.server.BaseContext = func(net.Listener) context.Context { return sctx }
	p.server.RegisterOnShutdown(func() { cancel() })

	if mm := p.cfg.metricsManager; mm != nil {
		p.server.ConnState = func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				mm.RegisterTCPConnection()
			case http.StateClosed, http.StateHijacked:
				mm.UnregisterTCPConnection()
			}
		}
	}

	listener := p.cfg.listener
	if listener == nil {
		if listener, err = net.Listen("tcp", p.listen); err != nil {
			return fmt.Errorf("unable to start listener: %w", err)
		}
	}
	if p.tlsConfig != nil {
		listener = tls.NewListener(listener, p.tlsConfig)
	}

	go func() {
		err := p.server.Serve(listener)
		if err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("unable to start server", "err", err)
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

	return p.server.Shutdown(stopCtx)
}

func (p *wsBackend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleCORS(w, req, p.cfg.corsPolicy) {
		return
	}

	ctx, span := p.cfg.tracer.Start(req.Context(), "backend")
	defer span.End()

	m := func(int) time.Duration { return 0 }
	if mm := p.cfg.metricsManager; mm != nil {
		m = mm.MeasureRequest(req.Method, req.URL.Path)
		mm.RegisterWSConnection()
		defer mm.UnregisterWSConnection()
	}

	if req.Method != http.MethodGet || req.URL.Path != "/ws" {
		hErr(w, "only supports GET /ws", http.StatusBadRequest, span)
		return
	}

	stream, err := client.NewStdio(p.mcpServer, p.cfg.clientOpts...).Start(ctx)
	if err != nil {
		slog.Error("Unable to start mcp client", err)
		hErr(w, fmt.Sprintf("unable to start mcp client: %s", err), http.StatusInternalServerError, span)
		m(http.StatusInternalServerError)
		return
	}

	select {
	default:
	case err := <-stream.Exit:
		slog.Error("MCP server has exited", err)
		hErr(w, fmt.Sprintf("mcp server has exited: %s", err), http.StatusInternalServerError, span)
		m(http.StatusInternalServerError)
		return
	}

	agentToken, _ := parseBasicAuth(req.Header.Get("Authorization"))

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("Unable to upgrade to websocket", err)
		m(http.StatusInternalServerError)
		return
	}

	session, err := wsc.Accept(ctx, ws, wsc.Config{WriteChanSize: 64, ReadChanSize: 16})
	if err != nil {
		hErr(w, fmt.Sprintf("unable to accept websocket: %s", err), http.StatusBadRequest, span)
		m(http.StatusBadRequest)
		return
	}

	span.End()
	m(http.StatusSwitchingProtocols)

	defer session.Close(1001)

	rb := ringbuffer.New(4096)

	agent := api.Agent{
		Token:      agentToken,
		RemoteAddr: req.Header.Get("X-Forwarded-For"),
		UserAgent:  req.Header.Get("X-Forwarded-UA"),
	}

	cache := ccache.New(ccache.Configure[context.Context]().MaxSize(64))

	for {

		select {

		case data := <-session.Read():

			slog.Debug("Received data from websocket", "msg", string(data))

			if data, err = p.handleMCPCall(ctx, cache, session, agent, data, api.CallTypeRequest); err != nil {
				slog.Error("Unable to handle mcp agent message", err)
				continue
			}

			stream.Stdin <- sanitize.Data(data)

		case data := <-stream.Stdout:

			slog.Debug("Received data from MCP Server", "msg", string(data))

			if data, err = p.handleMCPCall(ctx, cache, session, agent, data, api.CallTypeResponse); err != nil {
				slog.Error("Unable to handle mcp server message", err)
				continue
			}

			session.Write(sanitize.Data(data))

		case data := <-stream.Stderr:
			_, _ = rb.Write(data)
			slog.Debug("MCP Server Log", "stderr", string(data))

		case err := <-stream.Exit:

			select {
			default:
			case data := <-stream.Stderr:
				_, _ = rb.Write(data)
			}

			data, _ := io.ReadAll(rb)
			slog.Error("MCP Server exited", err)

			if p.cfg.dumpStderr {
				_, _ = fmt.Fprintf(os.Stderr, "---\n%s\n---\n", strings.TrimSpace(string(data)))
			} else {
				slog.Error("MCP Server stderr", "stderr", string(data))
			}

			return

		case err := <-session.Error():
			slog.Debug("Websocket encoountered and error", err)
			return

		case <-session.Done():
			slog.Debug("Websocket has closed")
			return

		case <-ctx.Done():
			slog.Debug("Client is gone")
			return
		}
	}
}

func (p *wsBackend) handleMCPCall(ctx context.Context, cache *ccache.Cache[context.Context], session wsc.Websocket, agent api.Agent, data []byte, rtype api.CallType) (buff []byte, err error) {

	call := api.NewMCPCall(-1)
	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &call); err != nil {
		var oerr = err
		call.Error = api.NewMCPError(err)
		if data, err = elemental.Encode(elemental.EncodingTypeJSON, call); err != nil {
			return nil, fmt.Errorf("unable to decode mcp call and to encode an error: %w (original: %w)", err, oerr)
		}
		return data, nil
	}

	// We check if we have the _meta params in the call and if so, we get the otel context from there.
	mc := newMCPMetaCarrier(call)
	if len(mc.meta) > 0 {
		ctx = otel.GetTextMapPropagator().Extract(ctx, mc)
	}

	kind := trace.SpanKindClient
	if rtype == api.CallTypeResponse {
		kind = trace.SpanKindServer
	}

	ctx, pctx, lspan, name := spanContextFromCache(ctx, cache, p.cfg.tracer, call, kind)
	defer lspan.End()

	var spc *api.SpanContext
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		spc = &api.SpanContext{}
		spc.TraceID = sc.TraceID().String()
		spc.ID = sc.SpanID().String()
		spc.Start = time.Now()
		spc.Name = name

		if parentCtx := trace.SpanContextFromContext(pctx); parentCtx.IsValid() {
			spc.ParentSpanID = parentCtx.SpanID().String()
		}
	}

	if data, err = p.police(ctx, spc, rtype, agent, call, data); err != nil {

		var oerr = err
		if errors.Is(err, api.ErrBlocked) {
			session.Write(sanitize.Data(data))
			return nil, nil
		}

		call.Error = api.NewMCPError(err)
		if data, err = elemental.Encode(elemental.EncodingTypeJSON, call); err != nil {
			return nil, fmt.Errorf("unable to police mcp call: %w (original: %w)", err, oerr)
		}
		return data, nil
	}

	return data, nil
}

func (p *wsBackend) police(ctx context.Context, spc *api.SpanContext, rtype api.CallType, agent api.Agent, call api.MCPCall, rawData []byte) ([]byte, error) {

	// This is tools/list response, if we have hashes for them, we verify their integrity.
	if dtools, ok := call.Result["tools"]; ok && len(p.cfg.sbom.Tools) > 0 {

		tools := api.Tools{}
		if err := mapstructure.Decode(dtools, &tools); err != nil {
			return nil, fmt.Errorf("unable to decode tools result for hashing: %w", err)
		}

		lhashes, err := scan.HashTools(tools)
		if err != nil {
			return nil, fmt.Errorf("unable to hash tools result: %w", err)
		}

		if err := p.cfg.sbom.Tools.Matches(lhashes); err != nil {
			return makeMCPError(call.ID, err), fmt.Errorf("%w: %w", api.ErrBlocked, err)
		}
	}

	// This is prompts/list response, if we have hashes for them, we verify their integrity.
	if dtools, ok := call.Result["prompts"]; ok && len(p.cfg.sbom.Prompts) > 0 {

		prompts := api.Prompts{}
		if err := mapstructure.Decode(dtools, &prompts); err != nil {
			return nil, fmt.Errorf("unable to decode prompts result for hashing: %w", err)
		}

		lhashes, err := scan.HashPrompts(prompts)
		if err != nil {
			return nil, fmt.Errorf("unable to hash prompts result: %w", err)
		}

		if err := p.cfg.sbom.Prompts.Matches(lhashes); err != nil {
			return makeMCPError(call.ID, err), fmt.Errorf("%w: %w", api.ErrBlocked, err)
		}
	}

	if p.cfg.policer == nil {
		return rawData, nil
	}

	m := func(bool) time.Duration { return 0 }
	if mm := p.cfg.metricsManager; mm != nil {
		m = mm.MeasurePolicer(p.cfg.policer.Type(), rtype)
	}

	ctx, span := p.cfg.tracer.Start(ctx, "policer")
	defer span.End()

	req := api.Request{
		Type:  rtype,
		MCP:   call,
		Agent: agent,
	}
	if spc != nil {
		req.SpanContext = *spc
		req.SpanContext.End = time.Now()
	}

	rcall, err := p.cfg.policer.Police(ctx, req)

	logFunc := slog.Info
	if !p.cfg.policerEnforced && err != nil {
		logFunc = slog.Warn
	}
	defer logFunc("Policer result", "allowed", err == nil, "enforced", p.cfg.policerEnforced, "err", err)

	if err != nil {
		defer m(false)

		if errors.Is(err, api.ErrBlocked) {
			span.SetStatus(codes.Error, err.Error())
			if !p.cfg.policerEnforced {
				return rawData, nil
			}
			return makeMCPError(call.ID, err), err
		}

		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("unable to run policer.Police: %w", err)
	}

	if rcall != nil {

		// swap data
		rawData, err = elemental.Encode(elemental.EncodingTypeJSON, rcall)
		if err != nil {
			defer m(false)
			return nil, fmt.Errorf("unable to reencode modified mcp call: %w", err)
		}
	}

	m(true)
	span.SetStatus(codes.Ok, "")

	return rawData, nil
}

func spanContextFromCache(
	ctx context.Context,
	cache *ccache.Cache[context.Context],
	tracer trace.Tracer,
	call api.MCPCall,
	kind trace.SpanKind,
) (context.Context, context.Context, trace.Span, string) {

	cid := call.IDString()

	name := "mcp.agent"
	if kind == trace.SpanKindServer {
		name = "mcp.server"
	}

	if cid == "" {
		rctx, rspan := tracer.Start(ctx, name,
			trace.WithAttributes(
				attribute.String("type", "notification"),
				attribute.String("mcp.method", call.Method),
			),
			trace.WithSpanKind(kind),
		)
		return rctx, nil, rspan, name
	}

	attrs := []attribute.KeyValue{}

	cached := false
	if item := cache.Get(cid); item != nil && !item.Expired() {
		ctx = item.Value()
		cache.Delete(cid)
		cached = true
	}

	if call.Error != nil {
		attrs = append(attrs,
			attribute.String("mcp.type", "response"),
			attribute.Bool("error", true),
		)

	} else if call.Result != nil {
		attrs = append(attrs, attribute.String("mcp.type", "response"))
	} else {
		attrs = append(attrs,
			attribute.String("mcp.type", "request"),
			attribute.String("mcp.method", call.Method),
		)

		if call.Method == "tools/call" {
			if n, ok := call.Params["name"].(string); ok {
				attrs = append(attrs, attribute.String("name", n))
			}

			if args, ok := call.Params["arguments"].(map[string]any); ok {
				for k, v := range args {
					attrs = append(attrs, attribute.String(fmt.Sprintf("mcp.param.%s", k), fmt.Sprintf("%v", v)))
				}
			}
		}
	}

	rctx, span := tracer.Start(ctx, name,
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(kind),
	)

	if !cached {
		cache.Set(cid, rctx, time.Minute)
	}

	return rctx, ctx, span, name
}
