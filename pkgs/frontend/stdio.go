package frontend

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"time"

	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/info"
	"go.acuvity.ai/minibridge/pkgs/internal/sanitize"
)

var _ Frontend = (*stdioFrontend)(nil)

type stdioFrontend struct {
	u               *url.URL
	agentAuth       *auth.Auth
	backendURL      string
	cfg             stdioCfg
	claims          []string
	tlsClientConfig *tls.Config
	user            string
	wsWrite         chan []byte
}

// NewStdio returns a new *StdioProxy that will connect to the given
// endpoint using the given tlsConfig. Agents can write request to stdin and read
// responses from stdout. stderr contains the logs.
//
// A single session to the backend will be created and it will
// reconnect in case of disconnection.
func NewStdio(backend string, tlsConfig *tls.Config, opts ...OptStdio) Frontend {

	cfg := newStdioCfg()
	for _, o := range opts {
		o(&cfg)
	}

	u, err := url.Parse(backend)
	if err != nil {
		panic(err)
	}

	return &stdioFrontend{
		u:               u,
		backendURL:      backend,
		tlsClientConfig: tlsConfig,
		wsWrite:         make(chan []byte),
		cfg:             cfg,
	}
}

// Start starts the proxy. It will run until the given context is canceled or until
// the server returns an error.
func (p *stdioFrontend) Start(ctx context.Context, agentAuth *auth.Auth) error {

	p.agentAuth = agentAuth

	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to get current user: %w", err)
	}

	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("unable to get current hostname: %w", err)
	}

	p.user = fmt.Sprintf("%s@%s", user.Username, host)
	p.claims = []string{
		fmt.Sprintf("gid=%s", user.Gid),
		fmt.Sprintf("uid=%s", user.Uid),
		fmt.Sprintf("username=%s", user.Username),
		fmt.Sprintf("hostname=%s", host),
		"minibridge=stdio",
	}

	slog.Debug("Local machine user set", "user", p.user, "claims", p.claims)

	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	go func() { errCh <- p.stdiopump(subctx) }()
	go func() { errCh <- p.wspump(subctx) }()

	return <-errCh
}

func (p *stdioFrontend) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: p.tlsClientConfig,
			DialContext:     p.cfg.backendDialer,
		},
	}
}

func (p *stdioFrontend) BackendURL() string {

	scheme := "http"
	if p.u.Scheme == "wss" || p.u.Scheme == "https" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, p.u.Host)
}

func (p *stdioFrontend) BackendInfo() (info.Info, error) {
	return getBackendInfo(p)
}

func (p *stdioFrontend) wspump(ctx context.Context) error {

	var failures int

	for {

		select {

		case <-ctx.Done():
			return nil

		default:
			session, err := Connect(ctx, p.cfg.backendDialer, p.backendURL, p.tlsClientConfig, AgentInfo{
				Auth:       p.agentAuth,
				UserAgent:  "stdio",
				RemoteAddr: "local",
			})
			if err != nil {

				if !p.cfg.retry {
					return err
				}

				if failures == 1 {
					slog.Error("Retrying...", err)
				}

				failures++
				time.Sleep(2 * time.Second)

				continue
			}

			if failures > 0 {
				slog.Info("Connection restored", "attempts", failures)
			}

			failures = 0

		L:
			for {

				select {

				case buf := <-p.wsWrite:
					session.Write(sanitize.Data(buf))

				case data := <-session.Read():
					fmt.Println(string(sanitize.Data(data)))

				case err := <-session.Error():
					failures++
					slog.Error("Error from webscoket", err)

				case err := <-session.Done():
					failures++
					if !p.cfg.retry {
						return err
					}
					break L

				case <-ctx.Done():
					session.Close(1000)
					return nil
				}
			}
		}
	}
}

func (p *stdioFrontend) stdiopump(ctx context.Context) error {

	stdin := bufio.NewReader(os.Stdin)

	for {
		select {

		default:

			buf, err := stdin.ReadBytes('\n')

			if err != nil {
				slog.Debug("unable to read stdin", err)
				return err
			}

			if len(buf) == 0 {
				continue
			}

			p.wsWrite <- sanitize.Data(buf)

		case <-ctx.Done():
			return nil
		}
	}
}
