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

	"go.acuvity.ai/minibridge/pkgs/data"
)

type stdioClient struct {
	srv MCPServer
	cfg cfg
}

// NewStdio returns a Client communicating through stdio.
func NewStdio(srv MCPServer, options ...Option) Client {

	cfg := newCfg()
	for _, o := range options {
		o(&cfg)
	}

	return &stdioClient{
		srv: srv,
		cfg: cfg,
	}
}

func (c *stdioClient) Start(ctx context.Context) (pipe *MCPStream, err error) {

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

	inCh := make(chan []byte)
	go c.readRequests(ctx, stdin, inCh)

	outCh := make(chan []byte)
	go c.readResponses(ctx, stdout, outCh)

	errCh := make(chan []byte)
	go c.readErrors(ctx, stderr, errCh)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start command: %w", err)
	}

	exitCh := make(chan error)
	go func() {
		exitCh <- cmd.Wait()
	}()

	return &MCPStream{
		Stdin:  inCh,
		Stdout: outCh,
		Stderr: errCh,
		Exit:   exitCh,
	}, nil
}

func (c *stdioClient) readRequests(ctx context.Context, stdin io.WriteCloser, ch chan []byte) {

	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-ch:
			if _, err := stdin.Write(append(data.Sanitize(buf), '\n')); err != nil {
				slog.Error("Unable to write data to stdin", "err", err)
				return
			}
		}
	}
}

func (c *stdioClient) readResponses(ctx context.Context, stdout io.ReadCloser, ch chan []byte) {

	bstdout := bufio.NewReader(stdout)
	for {
		buf, err := bstdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("Unable to read response from stdout", "err", err)
			}
			return
		}
		select {
		case ch <- data.Sanitize(buf):
		case <-ctx.Done():
			return
		}
	}
}

func (c *stdioClient) readErrors(ctx context.Context, stderr io.ReadCloser, ch chan []byte) {

	bstderr := bufio.NewReader(stderr)
	for {
		buf, err := bstderr.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				slog.Error("Unable to read error response from stderr", "err", err)
			}
			return
		}
		select {
		case ch <- data.Sanitize(buf):
		case <-ctx.Done():
			return
		}
	}
}
