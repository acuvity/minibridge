package mcp

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
	"go.acuvity.ai/wsc"
)

// WSProxy is the end part of the tunnel
type WSProxy struct {
	server    *http.Server
	mcpServer Server
}

func NewWSProxy(listen string, tlsConfig *tls.Config, mcpServer Server) *WSProxy {

	p := &WSProxy{
		mcpServer: mcpServer,
	}

	p.server = &http.Server{
		TLSConfig: tlsConfig,
		Addr:      listen,
		Handler:   p,
	}

	return p
}

func (p *WSProxy) Start(ctx context.Context) {

	go func() {
		if p.server.TLSConfig == nil {
			slog.Info("Server started", "tls", false, "listen", p.server.Addr)
			if err := p.server.ListenAndServe(); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start server", "err", err)
				}
			}
		} else {
			slog.Info("Server started", "tls", true, "listen", p.server.Addr)
			if err := p.server.ListenAndServeTLS("", ""); err != nil {
				if !errors.Is(err, http.ErrServerClosed) {
					slog.Error("unable to start server", "err", err)
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		stopctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = p.server.Shutdown(stopctx)
	}()
}

func (p *WSProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodGet || req.URL.Path != "/ws" {
		http.Error(w, "only supports GET /ws", http.StatusBadRequest)
		return
	}

	client, err := NewStdioMCPClient(p.mcpServer)
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
