package frontend

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"go.acuvity.ai/minibridge/pkgs/cors"
	"go.acuvity.ai/minibridge/pkgs/data"
	"go.acuvity.ai/wsc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

type session struct {
	id       string
	ws       wsc.Websocket
	credHash uint64
	count    int
}

type httpFrontend struct {
	backendURL      string
	server          *http.Server
	sessions        map[string]*session
	tlsClientConfig *tls.Config
	cfg             httpCfg

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

	p := &httpFrontend{
		backendURL:      backend,
		tlsClientConfig: clientTLSConfig,
		sessions:        map[string]*session{},
		cfg:             cfg,
	}

	p.server = &http.Server{
		Addr:              addr,
		Handler:           otelhttp.NewHandler(http.HandlerFunc(p.ServeHTTP), "frontend"),
		TLSConfig:         serverTLSConfig,
		ReadHeaderTimeout: time.Second,
	}

	return p
}

// Start starts the frontend. It will block until the given context cancels or
// until the server returns an error.
func (p *httpFrontend) Start(ctx context.Context) error {

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

	go func() {
		if p.server.TLSConfig == nil {
			err := p.server.ListenAndServe()
			if err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start server", "err", err)
				}
			}
			errCh <- err
		} else {
			err := p.server.ListenAndServeTLS("", "")
			if err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start tls server", "err", err)
				}
			}
			errCh <- err
		}
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

// startSession register a new session, connects to the backend ws, acquires it immediately, and returns it.
// Caller MUST release the session when you done using releaseSession.
func (p *httpFrontend) startSession(ctx context.Context, req *http.Request) (*session, error) {

	token, authHeader := p.getCreds(req)
	ch := hashCreds(token, authHeader)

	ws, err := connectWS(ctx, p.cfg.backendDialer, p.backendURL, p.tlsClientConfig, agentInfo{
		token:       token,
		authHeaders: authHeader,
		remoteAddr:  req.RemoteAddr,
		userAgent:   req.UserAgent(),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to register session backend: %w", err)
	}

	s := &session{
		ws:       ws,
		credHash: ch,
		count:    1,
		id:       uuid.Must(uuid.NewV6()).String(),
	}

	p.Lock()
	p.sessions[s.id] = s
	p.Unlock()

	slog.Debug("HTTP: session reqgistered and acquired", "sid", s.id, "c", 1)
	return s, nil
}

// acquireSession acquires and returns the session with the given sid.
// It returns nil if not session with that sid is found.
func (p *httpFrontend) acquireSession(sid string) *session {

	p.Lock()
	s := p.sessions[sid]
	if s != nil {
		s.count++
		slog.Debug("HTTP: session acquired", "sid", sid, "c", s.count)
	}
	p.Unlock()

	return s
}

// releaseSession sessions releases an acquired session.
// if the session is not acquired by anything, the ws connection
// will be closed, and the session deleted.
func (p *httpFrontend) releaseSession(sid string) {

	p.Lock()
	s := p.sessions[sid]
	if s == nil {
		return
	}
	s.count--
	slog.Debug("HTTP: session released", "sid", sid, "c", s.count, "deleted", s.count <= 0)
	if s.count <= 0 {
		s.ws.Close(1001)
		delete(p.sessions, sid)
	}
	p.Unlock()
}

func (p *httpFrontend) handleMCP(w http.ResponseWriter, req *http.Request) {

	var err error

	m := func(int) time.Duration { return 0 }
	if p.cfg.metricsManager != nil {
		m = p.cfg.metricsManager.MeasureRequest(req.Method, req.URL.Path)
	}

	ctx, span := p.cfg.tracer.Start(req.Context(), "streamable")
	defer span.End()

	if req.Method != http.MethodPost && req.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		m(http.StatusMethodNotAllowed)
		return
	}

	slog.Debug("Handling new streamable request", "client", req.RemoteAddr)

	var s *session
	if sid := req.Header.Get("Mcp-Session-Id"); sid != "" {
		s = p.acquireSession(sid)
	} else if s, err = p.startSession(ctx, req); err != nil {
		http.Error(w, fmt.Sprintf("unable to start session: %s", err), http.StatusForbidden)
		m(http.StatusForbidden)
		return
	}
	defer p.releaseSession(s.id)
	w.Header().Set("Mcp-Session-Id", s.id)

	span.SetAttributes(attribute.String("session", s.id))

	// check the creds hash are identical to prevent session id reuse.
	if hashCreds(p.getCreds(req)) != s.credHash {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		m(http.StatusUnauthorized)
		return
	}

	if req.Method == http.MethodPost {
		buf, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
			m(http.StatusBadRequest)
			return
		}
		defer func() { _ = req.Body.Close() }()

		s.ws.Write(data.Sanitize(buf))
	}

	switch req.Header.Get("Accept") {

	case "application/json":

		if req.Method == http.MethodGet {
			http.Error(w, "Not Acceptable: only text/event-stream can be accepted during GET", http.StatusNotAcceptable)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		select {

		case data := <-s.ws.Read():
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)

		case <-ctx.Done():
			return
		}

	case "text/event-stream":
		p.startStream(ctx, w, req, s, false)

	default:
		http.Error(w, "Accept must be application/json or text/event-stream", http.StatusNotAcceptable)
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
		http.Error(w, fmt.Sprintf("Unable to connect to minibridge end: %s", err), http.StatusForbidden)
		m(http.StatusForbidden)
		return
	}
	defer p.releaseSession(s.id)

	p.startStream(ctx, w, req, s, true)
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

	session := p.acquireSession(sid)
	if session == nil {
		http.Error(w, "Session not found", http.StatusForbidden)
		m(http.StatusForbidden)
		return
	}
	defer p.releaseSession(sid)

	if hashCreds(p.getCreds(req)) != session.credHash {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		m(http.StatusUnauthorized)
		return
	}

	buf, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
		m(http.StatusBadRequest)
		return
	}
	defer func() { _ = req.Body.Close() }()

	log.Debug("Message data", "msg", string(buf))

	if req.Proto == "HTTP/1.1" {
		w.Header().Add("Connection", "keep-alive")
		w.Header().Add("Keep-Alive", "timeout=5")
	}
	w.Header().Add("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusAccepted)

	defer m(http.StatusAccepted)

	session.ws.Write(data.Sanitize(buf))
}

func (p *httpFrontend) startStream(ctx context.Context, w http.ResponseWriter, req *http.Request, s *session, backwardCompat bool) {

	log := slog.With("sid", s.id)

	w.Header().Add("Content-Type", "text/event-stream")
	if req.Proto == "HTTP/1.1" {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Connection", "keep-alive")
	}

	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)

	if backwardCompat {
		if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s?sessionId=%s\n\n", p.cfg.messagesEndpoint, s.id); err != nil {
			log.Error("Unable to send endpoint event", err)
			return
		}

		if err := rc.Flush(); err != nil {
			log.Error("Unable to flush endpoint event", err)
			return
		}
	}

	defer func() { _ = rc.Flush() }()

	for {

		select {

		case <-ctx.Done():
			log.Debug("Client is gone from stream")
			return

		case buf := <-s.ws.Read():

			if len(buf) == 0 {
				continue
			}

			log.Debug("Received data from backend", "data", string(buf))

			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data.Sanitize(buf))); err != nil {
				log.Error("Unable to write event", err)
				continue
			}

			if err := rc.Flush(); err != nil {
				log.Error("Unable to flush remote event", err)
				continue
			}

		case <-s.ws.Done():
			log.Debug("Backend websocket is gone")
			return
		}
	}
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

	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (p *httpFrontend) getCreds(req *http.Request) (token string, authHeaders []string) {

	if p.cfg.agentTokenPassthrough {
		authHeaders = req.Header["Authorization"]
	} else if p.cfg.agentToken != "" {
		token = p.cfg.agentToken
	}

	return token, authHeaders
}
