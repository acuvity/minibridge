package frontend

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"strings"
	"time"
)

type stdioFrontend struct {
	backendURL string
	tlsConfig  *tls.Config
	cfg        stdioCfg
	wsWrite    chan []byte
	user       string
	claims     []string
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

	return &stdioFrontend{
		backendURL: backend,
		tlsConfig:  tlsConfig,
		wsWrite:    make(chan []byte),
		cfg:        cfg,
	}
}

// Start starts the proxy. It will run until the given context is canceled or until
// the server returns an error.
func (p *stdioFrontend) Start(ctx context.Context) error {

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

	go p.stdiopump(subctx)

	return p.wspump(subctx)
}

func (p *stdioFrontend) wspump(ctx context.Context) error {

	var failures int

	for {

		select {

		case <-ctx.Done():
			return nil

		default:
			session, err := connectWS(ctx, p.backendURL, p.tlsConfig, agentInfo{
				token:      p.cfg.agentToken,
				userAgent:  "stdio",
				remoteAddr: "local",
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

				case data := <-p.wsWrite:
					session.Write(data)

				case data := <-session.Read():
					if len(data) > 0 {
						fmt.Println(strings.TrimRight(string(data), "\n"))
					}

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

func (p *stdioFrontend) stdiopump(ctx context.Context) {

	stdin := bufio.NewReader(os.Stdin)

	for {
		select {

		default:

			data, err := stdin.ReadBytes('\n')

			if err != nil {
				if err != io.EOF {
					slog.Error("Unable to read data from stdin", "err", err)
				}
				continue
			}

			if len(data) == 0 {
				continue
			}

			if !bytes.HasSuffix(data, []byte("\n")) {
				data = append(data, '\n')
			}

			p.wsWrite <- data

		case <-ctx.Done():
			return
		}
	}
}
