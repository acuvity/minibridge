package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/internal/sanitize"
)

var _ Client = (*sseClient)(nil)
var _ RemoteClient = (*sseClient)(nil)

var ErrAuthRequired = errors.New("authorization required")

type sseClient struct {
	u               *url.URL
	endpoint        string
	messageEndpoint string
	client          *http.Client
}

func NewSSE(endpoint string, tlsConfig *tls.Config) Client {

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:       tlsConfig,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		panic(err)
	}

	return &sseClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		client:   client,
		u:        u,
	}
}

func (c *sseClient) Type() string { return "sse" }

func (c *sseClient) Server() string { return c.BaseURL() }

func (c *sseClient) BaseURL() string { return fmt.Sprintf("%s://%s", c.u.Scheme, c.u.Host) }

func (c *sseClient) HTTPClient() *http.Client { return c.client }

func (c *sseClient) MPCClient() Client { return c }

func (c *sseClient) Start(ctx context.Context, opts ...Option) (pipe *MCPStream, err error) {

	cfg := cfg{}
	for _, o := range opts {
		o(&cfg)
	}

	sseEndpoint := fmt.Sprintf("%s/sse", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to initiate request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if cfg.auth != nil {
		req.Header.Set("Authorization", cfg.auth.Encode())
	}

	// we don't close the body here (which makes the linter all triggered)
	// because it's a long running connection. however readResponse will
	// do it on exit
	resp, err := c.client.Do(req) // nolint
	if err != nil {
		return nil, fmt.Errorf("unable to send initial sse request (%s): %w", req.URL.String(), err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrAuthRequired
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response from sse initialization (%s): %s", req.URL.String(), resp.Status)
	}

	errCh := make(chan []byte)
	exitCh := make(chan error)

	inCh := make(chan []byte)
	go c.readRequest(ctx, inCh, exitCh, cfg.auth)

	outCh := make(chan []byte)
	go c.readResponse(ctx, resp.Body, outCh, exitCh)

	// Get the first response to get the endpoint
	var data []byte

L:
	for {
		select {
		case data = <-outCh:
			break L

		case err := <-exitCh:
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("unable to process sse message: %w", err)
			}

		case <-time.After(3 * time.Second):
			return nil, fmt.Errorf("did not receive /message endpoint in time")

		case <-ctx.Done():
			return nil, fmt.Errorf("did not receive /message endpoint in time: %w", ctx.Err())
		}
	}

	c.messageEndpoint = fmt.Sprintf(
		"%s/%s",
		c.endpoint,
		strings.TrimLeft(string(data), "/"),
	)
	slog.Debug("SSE Client: message endpoint set", "endpoint", c.messageEndpoint)

	return &MCPStream{
		Stdin:  inCh,
		Stdout: outCh,
		Stderr: errCh,
		Exit:   exitCh,
	}, nil
}

func (c *sseClient) readRequest(ctx context.Context, ch chan []byte, exitCh chan error, auth *auth.Auth) {

	for {

		select {

		case <-ctx.Done():
			return

		case data := <-ch:

			buf := bytes.NewBuffer(append(sanitize.Data(data), '\n', '\n'))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messageEndpoint, buf)
			if err != nil {
				exitCh <- fmt.Errorf("unable to make post request: %w", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			if auth != nil {
				req.Header.Set("Authorization", auth.Encode())
			}

			resp, err := c.client.Do(req)
			if err != nil {
				exitCh <- fmt.Errorf("unable to send post request: %w", err)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == http.StatusUnauthorized {
				exitCh <- ErrAuthRequired
				return
			}

			if resp.StatusCode != http.StatusAccepted {
				exitCh <- fmt.Errorf("invalid mcp server response status: %s", resp.Status)
				return
			}
		}
	}
}

func (c *sseClient) readResponse(ctx context.Context, r io.ReadCloser, ch chan []byte, exitCh chan error) {

	defer func() { _ = r.Close() }()

	scan := bufio.NewScanner(r)
	scan.Split(split)

	for scan.Scan() {

		data := sanitize.Data(scan.Bytes())

		parts := bytes.SplitN(data, []byte{'\n'}, 2)
		if len(parts) != 2 {
			exitCh <- fmt.Errorf("invalid sse message: %s", string(data))
			return
		}

		data = bytes.TrimPrefix(parts[1], []byte("data: "))

		select {
		case ch <- data:
		case <-ctx.Done():
			return
		}
	}

	if err := scan.Err(); err != nil {
		exitCh <- fmt.Errorf("sse stream closed: %w", scan.Err())
	} else {
		exitCh <- fmt.Errorf("sse stream closed: %w", io.EOF)
	}
}

func split(data []byte, atEOF bool) (int, []byte, error) {

	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i, nlen := hasNewLine(data); i >= 0 {
		return i + nlen, data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}

func hasNewLine(data []byte) (int, int) {

	crunix := bytes.Index(data, []byte("\n\n"))
	crwin := bytes.Index(data, []byte("\r\n\r\n"))
	minPos := minPos(crunix, crwin)
	nlen := 2
	if minPos == crwin {
		nlen = 4
	}
	return minPos, nlen
}

func minPos(a, b int) int {
	if a < 0 {
		return b
	}
	if b < 0 {
		return a
	}
	if a > b {
		return b
	}
	return a
}
