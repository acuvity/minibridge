package frontend

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/frontend/internal/session"
	"go.acuvity.ai/minibridge/pkgs/info"
	"go.acuvity.ai/minibridge/pkgs/internal/cors"
	"go.acuvity.ai/minibridge/pkgs/internal/sanitize"
	"go.acuvity.ai/minibridge/pkgs/mcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

var _ Frontend = (*httpFrontend)(nil)

type httpFrontend struct {
	u               *url.URL
	backendURL      string
	server          *http.Server
	listen          string
	tlsServerConfig *tls.Config
	tlsClientConfig *tls.Config
	cfg             httpCfg
	smanager        *session.Manager
	agentAuth       *auth.Auth

	sync.RWMutex
}

// NewHTTP returns a new frontend.Frontend that will listen to the given addr
// and will connect to the given minibridge backend using the given options.
// For every new connection to the /sse endpoint, a new websocket connection will
// be initiated to the backend, thus keeping track of the session.
func NewHTTP(addr string, backend string, serverTLSConfig *tls.Config, clientTLSConfig *tls.Config, opts ...OptHTTP) Frontend {

	cfg := newHTTPCfg()
	for _, o := range opts {
		o(&cfg)
	}

	u, err := url.Parse(backend)
	if err != nil {
		panic(err)
	}

	p := &httpFrontend{
		u:               u,
		smanager:        session.NewManager(),
		backendURL:      backend,
		tlsClientConfig: clientTLSConfig,
		tlsServerConfig: serverTLSConfig,
		listen:          addr,
		cfg:             cfg,
	}

	p.server = &http.Server{
		Handler:           otelhttp.NewHandler(http.HandlerFunc(p.ServeHTTP), "frontend"),
		ReadHeaderTimeout: time.Second,
	}

	return p
}

// Start starts the frontend. It will block until the given context cancels or
// until the server returns an error.
func (p *httpFrontend) Start(ctx context.Context, agentAuth *auth.Auth) error {

	p.agentAuth = agentAuth

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

	listener, err := net.Listen("tcp", p.listen)
	if err != nil {
		return fmt.Errorf("unable to start listener: %w", err)
	}

	if p.tlsServerConfig != nil {
		listener = tls.NewListener(listener, p.tlsServerConfig)
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

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()

	return p.server.Shutdown(stopCtx)
}

func (p *httpFrontend) BackendURL() string {

	if p.u.Scheme == "wss" {
		return fmt.Sprintf("https://%s", p.u.Host)
	}

	return fmt.Sprintf("http://%s", p.u.Host)
}

func (p *httpFrontend) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: p.tlsClientConfig,
			DialContext:     p.cfg.backendDialer,
		},
	}
}

func (p *httpFrontend) BackendInfo() (info.Info, error) {
	return getBackendInfo(p)
}

// ServeHTTP is the main HTTP handler. If you decide to not start the built-in server
// you can use this function directly into your own *http.Server.
func (p *httpFrontend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleCORS(w, req, p.cfg.corsPolicy) {
		return
	}

	switch req.URL.Path {

	case p.cfg.mcpEndpoint:
		p.handleMCP(w, req)

	case p.cfg.sseEndpoint:
		p.handleSSE(w, req)

	case p.cfg.messagesEndpoint:
		p.handleMessages(w, req)

	case "/.well-known/oauth-authorization-server",
		p.cfg.oauthEndpointRegister,
		p.cfg.oauthEndpointAuthorize,
		p.cfg.oauthEndpointToken:
		p.handleOAuth2(w, req)

	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (p *httpFrontend) handleMCP(w http.ResponseWriter, req *http.Request) {

	var err error

	m := func(int) time.Duration { return 0 }
	if p.cfg.metricsManager != nil {
		m = p.cfg.metricsManager.MeasureRequest(req.Method, req.URL.Path)
	}

	ctx, span := p.cfg.tracer.Start(req.Context(), "streamable")
	defer span.End()

	if req.Method != http.MethodPost && req.Method != http.MethodGet && req.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		m(http.StatusMethodNotAllowed)
		return
	}

	sid := req.Header.Get("Mcp-Session-Id")

	if req.Method == http.MethodDelete {
		slog.Info("Handling streamable session delete request", "sid", sid)
		p.smanager.Release(sid, nil)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	slog.Debug(
		"Handling new streamable request",
		"client", req.RemoteAddr,
		"method", req.Method,
		"sid", sid,
	)

	var data []byte
	if req.Method == http.MethodPost {
		if data, err = io.ReadAll(req.Body); err != nil {
			http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
			m(http.StatusBadRequest)
			return
		}
		defer func() { _ = req.Body.Close() }()
	}

	call := mcp.Message{}

	// Is this the protocol? a bug in Inspector?
	if len(data) > 0 {
		if err := json.Unmarshal(data, &call); err != nil {
			http.Error(w, fmt.Sprintf("unable to decode json body: %s", err), http.StatusBadRequest)
			m(http.StatusBadRequest)
			return
		}
	}

	var s *session.Session

	// We now need to understand the protocol in order to transport it...
	if call.Method == "initialize" {

		if s, err = p.startSession(ctx, req); err != nil {

			if errors.Is(err, ErrAuthRequired) {
				w.WriteHeader(http.StatusUnauthorized)
				m(http.StatusUnauthorized)
				return
			}

			http.Error(w, fmt.Sprintf("unable to start session: %s", err), http.StatusForbidden)
			m(http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", s.ID())

		sid = s.ID()
	}

	// From now on we MUST have a session ID.
	// If it's not set we return 400, per spec.
	if sid == "" {
		http.Error(w, "Mcp-Session-Id must be set", http.StatusBadRequest)
		m(http.StatusBadRequest)
		return
	}

	// If we can't find the session, we return 404, per spec.
	respCh := make(chan []byte, 10)
	if s = p.smanager.Acquire(sid, respCh); s == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		m(http.StatusNotFound)
		return
	}
	defer p.smanager.Release(s.ID(), respCh)

	span.SetAttributes(attribute.String("session", s.ID()))

	// check the creds hash are identical to prevent session id reuse.
	if !s.ValidateHash(hashCreds(p.getCreds(req))) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		m(http.StatusUnauthorized)
		return
	}

	if req.Method == http.MethodPost {
		s.Write(sanitize.Data(data))
	}

	// we wrote the data to the server at that point
	// If it was a notification, or a response, we say accepted
	// and we move on.
	if strings.HasPrefix(call.Method, "notifications") || call.Result != nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	rc := http.NewResponseController(w)
	defer func() { _ = rc.Flush() }()

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	for {

		select {

		case data := <-respCh:

			if len(data) == 0 {
				continue
			}

			resp := mcp.Message{}
			if err = json.Unmarshal(data, &resp); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				m(http.StatusInternalServerError)
				return
			}

			if !mcp.RelatedIDs(call.ID, resp.ID) {
				continue
			}

			if err := writeSSEMessage(w, rc, data); err != nil {
				slog.Error("Unable to write SSE message", "sid", s.ID(), err)
				continue
			}

			return

		case <-ctx.Done():
			return

		case err := <-s.Done():
			handleSessionDone(err)
			return
		}
	}
}

func (p *httpFrontend) handleSSE(w http.ResponseWriter, req *http.Request) {

	m := func(int) time.Duration { return 0 }
	if mm := p.cfg.metricsManager; mm != nil {
		m = mm.MeasureRequest(req.Method, req.URL.Path)
	}

	ctx, span := p.cfg.tracer.Start(req.Context(), "sse")
	defer span.End()

	if req.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		m(http.StatusMethodNotAllowed)
		return
	}

	slog.Debug("Handling new SSE", "client", req.RemoteAddr)

	s, err := p.startSession(ctx, req)
	if err != nil {
		if errors.Is(err, ErrAuthRequired) {
			w.WriteHeader(http.StatusUnauthorized)
			m(http.StatusUnauthorized)
			return
		}

		http.Error(w, fmt.Sprintf("Unable to connect to minibridge end: %s", err), http.StatusForbidden)
		m(http.StatusForbidden)
		return
	}

	readCh := make(chan []byte, 10)
	p.smanager.Acquire(s.ID(), readCh)
	defer p.smanager.Release(s.ID(), readCh)

	w.Header().Add("Content-Type", "text/event-stream")
	if req.Proto == "HTTP/1.1" {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Connection", "keep-alive")
	}

	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	defer func() { _ = rc.Flush() }()

	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s?sessionId=%s\n\n", p.cfg.messagesEndpoint, s.ID()); err != nil {
		slog.Error("Unable to send endpoint event", "sid", s.ID(), err)
		return
	}

	if err := rc.Flush(); err != nil {
		slog.Error("Unable to flush endpoint event", "sid", s.ID(), err)
		return
	}

	for {
		select {

		case <-ctx.Done():
			slog.Debug("Client is gone from stream")
			return

		case data := <-readCh:

			if len(data) == 0 {
				continue
			}

			slog.Debug("Received data from backend", "data", string(data))

			if err := writeSSEMessage(w, rc, data); err != nil {
				slog.Error("Unable to write SSE message", err)
				continue
			}

		case err := <-s.Done():
			handleSessionDone(err)
			return
		}
	}
}

func (p *httpFrontend) handleMessages(w http.ResponseWriter, req *http.Request) {

	m := func(int) time.Duration { return 0 }
	if p.cfg.metricsManager != nil {
		m = p.cfg.metricsManager.MeasureRequest(req.Method, req.URL.Path)
	}

	_, span := p.cfg.tracer.Start(req.Context(), "message")
	defer span.End()

	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		m(http.StatusMethodNotAllowed)
		return
	}

	sid := req.URL.Query().Get("sessionId")
	if sid == "" {
		http.Error(w, "Query parameter ID is required", http.StatusBadRequest)
		m(http.StatusBadRequest)
		return
	}
	span.SetAttributes(attribute.String("session", sid))

	accepts := req.Header.Get("Accept")

	log := slog.With("sid", sid)
	log.Debug("Handling messages", "accept", accepts)

	s := p.smanager.Acquire(sid, nil)
	if s == nil {
		http.Error(w, "Session not found", http.StatusForbidden)
		m(http.StatusForbidden)
		return
	}
	defer p.smanager.Release(sid, nil)

	if !s.ValidateHash(hashCreds(p.getCreds(req))) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		m(http.StatusUnauthorized)
		return
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
		m(http.StatusBadRequest)
		return
	}
	defer func() { _ = req.Body.Close() }()

	log.Debug("Message data", "msg", string(data))

	if req.Proto == "HTTP/1.1" {
		w.Header().Add("Connection", "keep-alive")
		w.Header().Add("Keep-Alive", "timeout=5")
	}
	w.Header().Add("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusAccepted)

	defer m(http.StatusAccepted)

	s.Write(sanitize.Data(data))
}

func (p *httpFrontend) handleOAuth2(w http.ResponseWriter, req *http.Request) {

	u := strings.TrimSuffix(p.backendURL, "/ws")
	uu, err := url.Parse(u)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to parse oauth2 url: %s", err), http.StatusBadRequest)
	}

	uu.Scheme = "http"
	if p.tlsClientConfig != nil {
		uu.Scheme = "https"
	}

	uu.Path = "/oauth2"
	uu.RawPath = uu.Path

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			DialContext:     p.cfg.backendDialer,
			TLSClientConfig: p.tlsClientConfig,
		},
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(uu)
		},
	}

	proxy.ServeHTTP(w, req)
}

// startSession register a new session, connects to the backend ws, acquires it immediately, and returns it.
// Caller MUST release the session when you done using releaseSession.
func (p *httpFrontend) startSession(ctx context.Context, req *http.Request) (*session.Session, error) {

	p.Lock()
	defer p.Unlock()

	auth, authHeader := p.getCreds(req)
	ch := hashCreds(auth, authHeader)

	if ctx == nil {
		ctx = context.Background()
	}

	ws, err := Connect(ctx, p.cfg.backendDialer, p.backendURL, p.tlsClientConfig, AgentInfo{
		Auth:        auth,
		AuthHeaders: authHeader,
		RemoteAddr:  req.RemoteAddr,
		UserAgent:   req.UserAgent(),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to register session backend: %w", err)
	}

	sid := fmt.Sprintf("%x", ch)

	s := session.New(ws, ch, sid)

	p.smanager.Register(s)

	return s, nil
}

func (p *httpFrontend) getCreds(req *http.Request) (auth *auth.Auth, authHeaders []string) {

	if p.agentAuth != nil {
		return p.agentAuth, nil
	}

	if p.cfg.agentTokenPassthrough {
		return nil, req.Header["Authorization"]
	}

	return nil, nil
}

func writeSSEMessage(w http.ResponseWriter, rc *http.ResponseController, data []byte) error {

	if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(sanitize.Data(data))); err != nil {
		return fmt.Errorf("unable to write event: %w", err)
	}

	if err := rc.Flush(); err != nil {
		return fmt.Errorf("unable to flush remote event: %w", err)
	}

	return nil
}

func handleSessionDone(err error) {

	if err == nil {
		return
	}

	if !strings.HasSuffix(err.Error(), "websocket: close 1001 (going away)") {
		slog.Error("Client websocket has closed", err)
		return
	}

	slog.Debug("Client websocket has closed")
}
