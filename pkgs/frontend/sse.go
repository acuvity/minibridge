package frontend

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"go.acuvity.ai/minibridge/pkgs/cors"
	"go.acuvity.ai/wsc"
)

type session struct {
	ws       wsc.Websocket
	credHash uint64
}

type sseFrontend struct {
	backendURL string
	server     *http.Server
	sessions   map[string]session
	tlsConfig  *tls.Config
	cfg        sseCfg

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
		backendURL: backend,
		tlsConfig:  clientTLSConfig,
		sessions:   map[string]session{},
		cfg:        cfg,
	}

	p.server = &http.Server{
		Addr:      addr,
		Handler:   p,
		TLSConfig: serverTLSConfig,
	}

	return p
}

// Start starts the frontend. It will block until the given context cancels or
// until the server returns an error.
func (p *sseFrontend) Start(ctx context.Context) error {

	errCh := make(chan error)

	go func() {
		if p.server.TLSConfig == nil {
			if err := p.server.ListenAndServe(); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start server", "err", err)
				}
				errCh <- err
			}
		} else {
			if err := p.server.ListenAndServeTLS("", ""); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start tls server", "err", err)
				}
				errCh <- err
			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	if req.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sid := uuid.Must(uuid.NewV6()).String()
	log := slog.With("sid", sid)

	log.Debug("Handling new SSE", "client", req.RemoteAddr)

	wsToken, wsAuthHeaders := p.getCreds(req)

	ws, err := connectWS(req.Context(), p.backendURL, p.tlsConfig, agentInfo{
		token:       wsToken,
		authHeaders: wsAuthHeaders,
		remoteAddr:  req.RemoteAddr,
		userAgent:   req.UserAgent(),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to connect to minibridge end: %s", err), http.StatusInternalServerError)
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

	rc := http.NewResponseController(w)

	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s?sessionId=%s\n\n", p.cfg.messagesEndpoint, sid); err != nil {
		log.Error("Unable to send endpoint event", err)
		return
	}

	if err := rc.Flush(); err != nil {
		log.Error("Unable to flush endpoint event", err)
		return
	}

	defer func() { _ = rc.Flush() }()

	for {

		select {

		case <-req.Context().Done():
			log.Debug("Client is gone")
			return

		case data := <-ws.Read():

			if len(data) == 0 {
				continue
			}

			log.Debug("Received data from backend", "data", string(data))

			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n", string(data)); err != nil {
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

	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sid := req.URL.Query().Get("sessionId")
	if sid == "" {
		http.Error(w, "Query parameter ID is required", http.StatusBadRequest)
		return
	}

	accepts := req.Header.Get("Accept")

	log := slog.With("sid", sid)
	log.Debug("Handling messages", "accept", accepts)

	session := p.getSession(sid)
	if session.ws == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	wsToken, wsAuthHeaders := p.getCreds(req)

	if hashCreds(wsToken, wsAuthHeaders) != session.credHash {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
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

	session.ws.Write(data)
}

// ServeHTTP is the main HTTP handler. If you decide to not start the built-in server
// you can use this function directly into your own *http.Server.
func (p *sseFrontend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleGenericHeaders(w, req, p.cfg.corsPolicy) {
		return
	}

	switch req.URL.Path {

	case p.cfg.sseEndpoint:
		p.handleSSE(w, req)

	case p.cfg.messagesEndpoint:
		p.handleMessages(w, req)
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
