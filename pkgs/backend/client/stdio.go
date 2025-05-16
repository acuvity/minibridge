package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"go.acuvity.ai/minibridge/pkgs/internal/sanitize"
)

var _ Client = (*stdioClient)(nil)

type stdioClient struct {
	srv MCPServer
	cfg stdioCfg
}

// NewStdio returns a Client communicating through stdio.
func NewStdio(srv MCPServer, options ...StdioOption) Client {

	cfg := newStdioCfg()
	for _, o := range options {
		o(&cfg)
	}

	return &stdioClient{
		srv: srv,
		cfg: cfg,
	}
}

func (c *stdioClient) Type() string {
	return "stdio"
}

func (c *stdioClient) Server() string { return c.srv.Command }

func (c *stdioClient) Start(ctx context.Context, _ ...Option) (pipe *MCPStream, err error) {

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unable to get current directory: %w", err)
	}

	if c.cfg.useTempDir {
		dir, err = os.MkdirTemp(os.TempDir(), "minibridge-")
		if err != nil {
			return nil, fmt.Errorf("unable to create tempdir: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, c.srv.Command, c.srv.Args...) // #nosec: G204
	cmd.Env = append(os.Environ(), c.srv.Env...)
	for i, e := range cmd.Env {
		cmd.Env[i] = strings.ReplaceAll(e, "_MINIBRIDGE_PREFIX_", dir)
	}

	cmd.Dir = dir
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			return err
		}

		if c.cfg.useTempDir {
			return os.RemoveAll(dir)
		}

		return nil
	}

	setCaps(cmd, "", c.cfg.creds)

	slog.Debug("Client: starting command",
		"path", cmd.Path,
		"dir", cmd.Dir,
		"creds", c.cfg.creds,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create stderr pipe: %w", err)
	}

	stream := NewMCPStream(ctx)

	go c.readRequests(ctx, stdin, stream.stdin)
	go c.readResponses(ctx, stdout, stream.stdout)
	go c.readErrors(ctx, stderr, stream.stderr)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start command: %w", err)
	}

	go func() { stream.exit <- cmd.Wait() }()

	return stream, nil
}

func (c *stdioClient) readRequests(ctx context.Context, stdin io.WriteCloser, ch chan []byte) {

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			if _, err := stdin.Write(append(sanitize.Data(data), '\n')); err != nil {
				slog.Error("Unable to write data to stdin", "err", err)
				return
			}
		}
	}
}

func (c *stdioClient) readResponses(ctx context.Context, stdout io.ReadCloser, ch chan []byte) {

	bstdout := bufio.NewReader(stdout)
	for {
		data, err := bstdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("Unable to read response from stdout", "err", err)
			}
			return
		}
		select {
		case ch <- sanitize.Data(data):
		case <-ctx.Done():
			return
		}
	}
}

func (c *stdioClient) readErrors(ctx context.Context, stderr io.ReadCloser, ch chan []byte) {

	bstderr := bufio.NewReader(stderr)
	for {
		data, err := bstderr.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("Unable to read error response from stderr", "err", err)
			}
			return
		}
		select {
		case ch <- sanitize.Data(data):
		case <-ctx.Done():
			return
		}
	}
}
