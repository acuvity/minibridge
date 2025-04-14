package frontend

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.acuvity.ai/wsc"
)

type stdioFrontend struct {
	backendURL string
	tlsConfig  *tls.Config
	cfg        stdioCfg
	wsWrite    chan []byte
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

	go p.wspump(ctx)
	go p.stdiopump(ctx)

	return nil
}

func (p *stdioFrontend) wspump(ctx context.Context) {

	connect := func(ctx context.Context) (wsc.Websocket, error) {

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

	var failures int

	for {

		select {

		case <-ctx.Done():
			return

		default:
			session, err := connect(ctx)
			if err != nil {
				if failures == 1 {
					slog.Error("Unable to connect. Will retry", err)
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
					fmt.Println(string(data))

				case err := <-session.Error():
					failures++
					slog.Error("Error from webscoket", err)

				case <-session.Done():
					failures++
					break L

				case <-ctx.Done():
					session.Close(1000)
					return
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
