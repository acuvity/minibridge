package backend

import (
	"context"
	"crypto/tls"
	"encoding/base64"
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
	"github.com/smallnest/ringbuffer"
	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/cors"
	"go.acuvity.ai/minibridge/pkgs/data"
	"go.acuvity.ai/minibridge/pkgs/policer"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
	"go.acuvity.ai/minibridge/pkgs/scan"
	"go.acuvity.ai/wsc"
)

type wsBackend struct {
	cfg       wsCfg
	mcpServer client.MCPServer
	server    *http.Server
}

// NewWebSocket retrurns a new backend.Backend exposing a Websocket to communicate
// with the given mcp.Server. It will use the given *tls.Config for everything TLS.
// It tls.Config is nil, the server will run as plain HTTP.
func NewWebSocket(listen string, tlsConfig *tls.Config, mcpServer client.MCPServer, opts ...OptWS) Backend {

	cfg := newWSCfg()
	for _, o := range opts {
		o(&cfg)
	}

	p := &wsBackend{
		mcpServer: mcpServer,
		cfg:       cfg,
	}

	p.server = &http.Server{
		TLSConfig:         tlsConfig,
		Addr:              listen,
		Handler:           p,
		ReadHeaderTimeout: time.Second,
	}

	return p
}

// Start starts the server and will block until the given
// context is canceled.
func (p *wsBackend) Start(ctx context.Context) error {

	errCh := make(chan error, 1)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.server.BaseContext = func(net.Listener) context.Context { return sctx }
	p.server.RegisterOnShutdown(func() { cancel() })

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

	stopctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return p.server.Shutdown(stopctx)
}

func (p *wsBackend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleCORS(w, req, p.cfg.corsPolicy) {
		return
	}

	if req.Method != http.MethodGet || req.URL.Path != "/ws" {
		http.Error(w, "only supports GET /ws", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	stream, err := client.NewStdio(p.mcpServer).Start(ctx)
	if err != nil {
		slog.Error("Unable to start mcp client", err)
		http.Error(w, fmt.Sprintf("unable to start mcp client: %s", err), http.StatusInternalServerError)
		return
	}

	select {
	default:
	case err := <-stream.Exit:
		slog.Error("MCP server has exited", err)
		http.Error(w, fmt.Sprintf("mcp server has exited: %s", err), http.StatusInternalServerError)
		return
	}

	agentToken, _ := parseBasicAuth(req.Header.Get("Authorization"))

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("Unable to upgrade to websocket", err)
		return
	}

	session, err := wsc.Accept(ctx, ws, wsc.Config{WriteChanSize: 64, ReadChanSize: 16})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to accept websocket: %s", err), http.StatusBadRequest)
		return
	}

	defer session.Close(1001)

	rb := ringbuffer.New(4096)

	agent := api.Agent{
		Token:      agentToken,
		RemoteAddr: req.Header.Get("X-Forwarded-For"),
		UserAgent:  req.Header.Get("X-Forwarded-UA"),
	}

	for {

		select {

		case buf := <-session.Read():

			slog.Debug("Received data from websocket", "msg", string(buf))

			if buf, err = policeData(ctx, p.cfg.policer, p.cfg.sbom, api.CallTypeRequest, agent, buf); err != nil {
				if errors.Is(err, api.ErrBlocked) {
					session.Write(data.Sanitize(buf))
					continue
				}
				slog.Error("Unable to police request", err)
				continue
			}

			stream.Stdin <- data.Sanitize(buf)

		case buf := <-stream.Stdout:

			slog.Debug("Received data from MCP Server", "msg", string(buf))

			if buf, err = policeData(ctx, p.cfg.policer, p.cfg.sbom, api.CallTypeOutput, agent, buf); err != nil {
				if errors.Is(err, api.ErrBlocked) {
					session.Write(data.Sanitize(buf))
					continue
				}
				slog.Error("Unable to police response", err)
				continue
			}

			session.Write(data.Sanitize(buf))

		case data := <-stream.Stderr:
			_, _ = rb.Write(data)
			slog.Debug("MCP Server Log", "stderr", string(data))

		case err := <-stream.Exit:
			select {
			case data := <-stream.Stderr:
				_, _ = rb.Write(data)
			default:
			}
			data, _ := io.ReadAll(rb)
			slog.Error("MCP Server exited", err)

			if p.cfg.dumpStderr {
				_, _ = fmt.Fprintf(os.Stderr, "---\n%s\n---\n", strings.TrimSpace(string(data)))
			} else {
				slog.Error("MCP Server stderr", "stderr", string(data))
			}
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

func parseBasicAuth(auth string) (password string, ok bool) {

	const prefix = "Basic "

	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			return parts[1], true
		}
		return "", false
	}

	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", false
	}

	cs := string(c)

	if _, password, ok = strings.Cut(cs, ":"); !ok {
		return "", false
	}

	return password, true
}

func policeData(ctx context.Context, pol policer.Policer, hashes scan.SBOM, typ api.CallType, agent api.Agent, data []byte) ([]byte, error) {

	call := api.MCPCall{}
	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &call); err != nil {
		return nil, fmt.Errorf("unable to decode mcp call: %w", err)
	}

	// This is tools/list response, if we have hashes for them, we verify their integrity.
	if dtools, ok := call.Result["tools"]; ok && len(hashes.Tools) > 0 {

		tools := api.Tools{}
		if err := mapstructure.Decode(dtools, &tools); err != nil {
			return nil, fmt.Errorf("unable to decode tools result for hashing: %w", err)
		}

		lhashes, err := scan.HashTools(tools)
		if err != nil {
			return nil, fmt.Errorf("unable to hash tools result: %w", err)
		}

		if err := hashes.Tools.Matches(lhashes); err != nil {
			return makeMCPError(call.ID, err), fmt.Errorf("%w: %w", api.ErrBlocked, err)
		}
	}

	// This is prompts/list response, if we have hashes for them, we verify their integrity.
	if dtools, ok := call.Result["prompts"]; ok && len(hashes.Prompts) > 0 {

		prompts := api.Prompts{}
		if err := mapstructure.Decode(dtools, &prompts); err != nil {
			return nil, fmt.Errorf("unable to decode prompts result for hashing: %w", err)
		}

		lhashes, err := scan.HashPrompts(prompts)
		if err != nil {
			return nil, fmt.Errorf("unable to hash prompts result: %w", err)
		}

		if err := hashes.Prompts.Matches(lhashes); err != nil {
			return makeMCPError(call.ID, err), fmt.Errorf("%w: %w", api.ErrBlocked, err)
		}
	}

	if pol == nil {
		return data, nil
	}

	rcall, err := pol.Police(ctx, api.Request{Type: typ, Agent: agent, MCP: call})
	if err != nil {

		if errors.Is(err, api.ErrBlocked) {
			return makeMCPError(call.ID, err), err
		}

		return nil, fmt.Errorf("unable to run policer.Police: %w", err)
	}

	if rcall != nil {

		// swap data
		data, err = elemental.Encode(elemental.EncodingTypeJSON, rcall)
		if err != nil {
			return nil, fmt.Errorf("unable to reencode modified mcp call: %w", err)
		}

	}

	return data, nil
}
