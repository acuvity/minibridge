package backend

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/smallnest/ringbuffer"
	"go.acuvity.ai/a3s/pkgs/claims"
	"go.acuvity.ai/a3s/pkgs/token"
	api "go.acuvity.ai/api/apex"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/cors"
	"go.acuvity.ai/minibridge/pkgs/policer"
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

	stopctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return p.server.Shutdown(stopctx)
}

func (p *wsBackend) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if !cors.HandleGenericHeaders(w, req, p.cfg.corsPolicy) {
		return
	}

	if req.Method != http.MethodGet || req.URL.Path != "/ws" {
		http.Error(w, "only supports GET /ws", http.StatusBadRequest)
		return
	}

	stream, err := client.NewStdio(p.mcpServer).Start(req.Context())
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

	tokenString, ok := parseBasicAuth(req.Header.Get("Authorization"))
	if (!ok || tokenString == "") && p.cfg.jwtJWKS != nil {
		slog.Error("Authentication invalid", "reason", "no token provided")
		http.Error(w, "Unauthenticated", http.StatusUnauthorized)
		return
	}

	user, err := authenticate(tokenString, p.cfg.jwtJWKS, p.cfg.jwtRequiredIss, p.cfg.jwtRequiredAud, p.cfg.jwtPrincipalClaims)
	if err != nil {
		slog.Error("Authentication rejected", err)
		http.Error(w, fmt.Sprintf("Invalid JWT: %s", err), http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("Unable to upgrade to websocket", err)
		return
	}

	session, err := wsc.Accept(req.Context(), ws, wsc.Config{WriteChanSize: 64, ReadChanSize: 16})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to accept websocket: %s", err), http.StatusBadRequest)
		return
	}

	defer session.Close(1001)

	rb := ringbuffer.New(4096)

	for {

		select {

		case data := <-session.Read():

			slog.Debug("Received data from websocket", "msg", string(data))

			if !bytes.HasSuffix(data, []byte("\n")) {
				data = append(data, '\n')
			}

			if pol := p.cfg.policer; pol != nil {
				if err := pol.Police(req.Context(), api.PoliceRequestTypeInput, data, user); err != nil {
					if errors.Is(err, policer.ErrBlocked) {
						stream.Stdout <- makeMCPError(data, err)
						continue
					}
					slog.Error("Unable to run input analysis", err)
					continue
				}
			}

			stream.Stdin <- data

		case data := <-stream.Stdout:

			slog.Debug("Received data from MCP Server", "msg", string(data))

			if pol := p.cfg.policer; pol != nil {
				if err := pol.Police(req.Context(), api.PoliceRequestTypeOutput, data, user); err != nil {
					if errors.Is(err, policer.ErrBlocked) {
						stream.Stdout <- makeMCPError(data, err)
						continue
					}
					slog.Error("Unable to run output analysis", err)
					continue
				}
			}

			session.Write(data)

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

		case <-req.Context().Done():
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
	_, password, ok = strings.Cut(cs, ":")
	if !ok {
		return "", false
	}
	return password, true
}

func authenticate(tokenString string, jwks *token.JWKS, reqIss string, reqAud string, pClaim string) (policer.User, error) {

	user := policer.User{}

	if tokenString == "" || jwks == nil {
		return user, nil
	}

	idt, err := token.Parse(tokenString, jwks, reqIss, reqAud)
	if err != nil {
		return user, fmt.Errorf("unable to parse token: %w", err)
	}

	cmap, err := claims.ToMap(idt.Identity)
	if err != nil {
		return user, fmt.Errorf("unable to parse token claims: %w", err)
	}

	pclaims := cmap[pClaim]
	if len(pclaims) == 0 {
		return user, fmt.Errorf("missing principal claim")
	}

	user.Claims = idt.Identity
	user.Name = pclaims[0]

	slog.Debug("Authenticated agent", "name", user.Name, "claims", user.Claims)

	return user, nil
}
