package backend

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.acuvity.ai/minibridge/mcp"
	"go.acuvity.ai/wsc"
)

type wsBackend struct {
	cfg           wsCfg
	clients       chan *mcp.StdioClient
	mcpServer     mcp.Server
	server        *http.Server
	policerClient *http.Client
}

// NewWebSocket retrurns a new backend.Backend exposing a Websocket to communicate
// with the given mcp.Server. It will use the given *tls.Config for everything TLS.
// It tls.Config is nil, the server will run as plain HTTP.
func NewWebSocket(listen string, tlsConfig *tls.Config, mcpServer mcp.Server, opts ...OptWS) Backend {

	cfg := newWSCfg()
	for _, o := range opts {
		o(&cfg)
	}

	p := &wsBackend{
		mcpServer: mcpServer,
		clients:   make(chan *mcp.StdioClient),
		cfg:       cfg,
	}

	if cfg.policerURL != "" {
		p.policerClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: cfg.policerTLSConfig,
			},
		}
	}

	p.server = &http.Server{
		TLSConfig: tlsConfig,
		Addr:      listen,
		Handler:   p,
	}

	return p
}

// Start starts the server and will block until the given
// context is canceled.
func (p *wsBackend) Start(ctx context.Context) error {

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
					slog.Error("unable to start server", "err", err)
				}
				errCh <- err
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				client, err := mcp.NewStdioMCPClient(p.mcpServer)
				if err != nil {
					slog.Error("Unable to spawn MCP Server", err)
					continue
				}

				p.clients <- client

			}
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	stopctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return p.server.Shutdown(stopctx)
}

func (p *wsBackend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodGet || req.URL.Path != "/ws" {
		http.Error(w, "only supports GET /ws", http.StatusBadRequest)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to start websocket: %s", err), http.StatusBadRequest)
		return
	}

	session, err := wsc.Accept(req.Context(), ws, wsc.Config{WriteChanSize: 64, ReadChanSize: 16})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to accept websocket: %s", err), http.StatusBadRequest)
		return
	}

	defer session.Close(1001)

	inCh, outCh, errCh := (<-p.clients).Start(req.Context())

	for {

		select {

		case data := <-session.Read():

			if !bytes.HasSuffix(data, []byte("\n")) {
				data = append(data, '\n')
			}

			if p.cfg.policerURL != "" {
				if err := police(
					req.Context(),
					p.policerClient,
					p.cfg.policerURL,
					p.cfg.policerToken,
					data,
				); err != nil {
					if errors.Is(err, ErrBlocked) {
						outCh <- makeMCPError(data, err)
						continue
					}
					slog.Error("Unable to run analysis", err)
					continue
				}
			}

			slog.Debug("Received data from websocket", "msg", string(data))
			inCh <- data

		case data := <-outCh:
			slog.Debug("Received data from MCP Server", "msg", string(data))
			session.Write(data)

		case data := <-errCh:
			_ = data
			// slog.Debug("MCP Server log", "log", string(data))

		case <-session.Done():
			slog.Debug("Websocket has closed")
			return

		case <-req.Context().Done():
			slog.Debug("Client is gone")
			return
		}
	}
}
