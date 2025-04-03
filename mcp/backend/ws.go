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
	server    *http.Server
	mcpServer mcp.Server
}

// NewWebSocket retrurns a new backend.Backend exposing a Websocket to communicate
// with the given mcp.Server. It will use the given *tls.Config for everything TLS.
// It tls.Config is nil, the server will run as plain HTTP.
func NewWebSocket(listen string, tlsConfig *tls.Config, mcpServer mcp.Server) Backend {

	p := &wsBackend{
		mcpServer: mcpServer,
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

	client, err := mcp.NewStdioMCPClient(p.mcpServer)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to launch mcp session: %s", err), http.StatusBadRequest)
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

	inCh, outCh, errCh := client.Start(req.Context())

	for {

		select {
		case data := <-session.Read():
			if len(data) == 0 {
				continue
			}

			if !bytes.HasSuffix(data, []byte("\n")) {
				data = append(data, '\n')
			}

			inCh <- data

		case data := <-outCh:
			session.Write(data)

		case data := <-errCh:
			slog.Debug("MCP Server log", "log", string(data))

		case <-session.Done():
			return

		case <-req.Context().Done():
			return
		}
	}
}
