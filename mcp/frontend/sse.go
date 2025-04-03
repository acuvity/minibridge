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
	"go.acuvity.ai/wsc"
)

type sseFrontend struct {
	backendURL string
	server     *http.Server
	sessions   map[string]wsc.Websocket
	tlsConfig  *tls.Config
	cfg        sseCfg

	sync.RWMutex
}

// NewSSE returns a new frontend.Frontend that will listen to the given addr
// and will connect to the given minibridge backend using the given options.
// For every new connection to the /sse endpoint, a new websocket connection will
// be initiated to the backend, thus keeping track of the session.
func NewSSE(addr string, backend string, tlsConfig *tls.Config, opts ...SSEOption) Frontend {

	cfg := newSSECfg()
	for _, o := range opts {
		o(&cfg)
	}

	p := &sseFrontend{
		backendURL: backend,
		tlsConfig:  tlsConfig,
		sessions:   map[string]wsc.Websocket{},
		cfg:        cfg,
	}

	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}

	return p
}

// Start starts the frontend. It will block until the given context cancels or
// until the server returns an error.
func (p *sseFrontend) Start(ctx context.Context) error {

	errCh := make(chan error)

	go func() {
		if err := p.server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("unable to start server", "err", err)
			}
			errCh <- err
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

func (p *sseFrontend) connect(ctx context.Context) (wsc.Websocket, error) {

	session, resp, err := wsc.Connect(
		ctx,
		p.backendURL,
		wsc.Config{
			WriteChanSize: 64,
			ReadChanSize:  16,
			TLSConfig:     p.tlsConfig,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to the websocket '%s': %w", p.backendURL, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("invalid response from other end of the tunnel (must be 101): %s", resp.Status)
	}

	return session, nil
}

func (p *sseFrontend) registerSession(sid string, ws wsc.Websocket) {
	p.Lock()
	p.sessions[sid] = ws
	p.Unlock()
}

func (p *sseFrontend) unregisterSession(sid string) {
	p.Lock()
	delete(p.sessions, sid)
	p.Unlock()
}

func (p *sseFrontend) getSession(sid string) wsc.Websocket {

	p.RLock()
	ws := p.sessions[sid]
	p.RUnlock()

	return ws
}

// ServeHTTP is the main HTTP handler. If you decide to not start the built-in server
// you can use this function directly into your own *http.Server.
func (p *sseFrontend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	switch req.URL.Path {

	case p.cfg.messagesEndpoint:

		if req.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		sid := req.URL.Query().Get("id")
		if sid == "" {
			http.Error(w, "Query parameter ID is required", http.StatusBadRequest)
			return
		}

		ws := p.getSession(sid)
		if ws == nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		data, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read body: %s", err), http.StatusBadRequest)
			return
		}
		defer func() { _ = req.Body.Close() }()

		ws.Write(data)

		w.WriteHeader(http.StatusOK)

	case p.cfg.sseEndpoint:

		if req.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		sid := uuid.Must(uuid.NewV6()).String()

		ws, err := p.connect(req.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to connect to minibridge end: %s", err), http.StatusInternalServerError)
			return
		}

		defer ws.Close(1000)

		p.registerSession(sid, ws)
		defer p.unregisterSession(sid)

		w.Header().Add("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Connection", "keep-alive")
		w.Header().Add("Set-Access-Control-Allow-Origin", "*")

		w.WriteHeader(http.StatusOK)

		rc := http.NewResponseController(w)

		if _, err := fmt.Fprintf(w, "event: endpoint\ndata: /messages?id=%s\n\n", sid); err != nil {
			slog.Error("Unable to send endpoint event", "id", sid, err)
			return
		}

		if err := rc.Flush(); err != nil {
			slog.Error("Unable to flush endpoint event", "id", sid, err)
			return
		}

		for {
			select {
			case <-req.Context().Done():
				return
			case data := <-ws.Read():
				if _, err := w.Write(data); err != nil {
					slog.Error("Unable to write event", "id", sid, err)
					continue
				}
				if err := rc.Flush(); err != nil {
					slog.Error("Unable to flush remote event", "id", sid, err)
					continue
				}
			case <-ws.Done():
				return
			}
		}
	}
}
