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
	ws       wsc.Websocket
	credHash uint64
}

type sseFrontend struct {
	backendURL      string
	server          *http.Server
	sessions        map[string]session
	tlsClientConfig *tls.Config
	cfg             sseCfg

	sync.RWMutex
}

// NewSSE returns a new frontend.Frontend that will listen to the given addr
// and will connect to the given minibridge backend using the given options.
// For every new connection to the /sse endpoint, a new websocket connection will
// be initiated to the backend, thus keeping track of the session.
func NewSSE(addr string, backend string, serverTLSConfig *tls.Config, clientTLSConfig *tls.Config, opts ...OptSSE) Frontend {

	cfg := newSSECfg()
	for _, o := range opts {
		o(&cfg)
	}

	p := &sseFrontend{
		backendURL:      backend,
		tlsClientConfig: clientTLSConfig,
		sessions:        map[string]session{},
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
func (p *sseFrontend) Start(ctx context.Context) error {

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

func (p *sseFrontend) registerSession(sid string, ws wsc.Websocket, credsHash uint64) {
	p.Lock()
	p.sessions[sid] = session{
		ws:       ws,
		credHash: credsHash,
	}
	p.Unlock()
}

func (p *sseFrontend) unregisterSession(sid string) {
	p.Lock()
	delete(p.sessions, sid)
	p.Unlock()
}

func (p *sseFrontend) getSession(sid string) session {

	p.RLock()
	ws := p.sessions[sid]
	p.RUnlock()

	return ws
}

func (p *sseFrontend) handleSSE(w http.ResponseWriter, req *http.Request) {

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

	sid := uuid.Must(uuid.NewV6()).String()
	span.SetAttributes(attribute.String("session", sid))

	log := slog.With("sid", sid)

	log.Debug("Handling new SSE", "client", req.RemoteAddr)

	wsToken, wsAuthHeaders := p.getCreds(req)

	ws, err := connectWS(ctx, p.cfg.backendDialer, p.backendURL, p.tlsClientConfig, agentInfo{
		token:       wsToken,
		authHeaders: wsAuthHeaders,
		remoteAddr:  req.RemoteAddr,
		userAgent:   req.UserAgent(),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to connect to minibridge end: %s", err), http.StatusInternalServerError)
		m(http.StatusInternalServerError)
		return
	}

	defer ws.Close(1000)

	p.registerSession(sid, ws, hashCreds(wsToken, wsAuthHeaders))
	defer p.unregisterSession(sid)

	w.Header().Add("Content-Type", "text/event-stream")
	if req.Proto == "HTTP/1.1" {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Connection", "keep-alive")
	}

	w.WriteHeader(http.StatusOK)
	span.End()

	rc := http.NewResponseController(w)

	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s?sessionId=%s\n\n", p.cfg.messagesEndpoint, sid); err != nil {
		log.Error("Unable to send endpoint event", err)
		m(http.StatusInternalServerError)
		return
	}

	if err := rc.Flush(); err != nil {
		log.Error("Unable to flush endpoint event", err)
		m(http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = rc.Flush()
		m(http.StatusOK)
	}()

	for {

		select {

		case <-ctx.Done():
			log.Debug("Client is gone from /sse")
			return

		case buf := <-ws.Read():

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

		case <-ws.Done():
			log.Debug("Backend websocket is gone")
			return
		}
	}
}

func (p *sseFrontend) handleMessages(w http.ResponseWriter, req *http.Request) {

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

	session := p.getSession(sid)
	if session.ws == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		m(http.StatusNotFound)
		return
	}

	wsToken, wsAuthHeaders := p.getCreds(req)

	if hashCreds(wsToken, wsAuthHeaders) != session.credHash {
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

// ServeHTTP is the main HTTP handler. If you decide to not start the built-in server
// you can use this function directly into your own *http.Server.
func (p *sseFrontend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleCORS(w, req, p.cfg.corsPolicy) {
		return
	}

	switch req.URL.Path {

	case p.cfg.sseEndpoint:
		p.handleSSE(w, req)

	case p.cfg.messagesEndpoint:
		p.handleMessages(w, req)

	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (p *sseFrontend) getCreds(req *http.Request) (token string, authHeaders []string) {

	if p.cfg.agentTokenPassthrough {
		authHeaders = req.Header["Authorization"]
	} else if p.cfg.agentToken != "" {
		token = p.cfg.agentToken
	}

	return token, authHeaders
}
