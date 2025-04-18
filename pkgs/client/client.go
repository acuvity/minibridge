package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type stdioClient struct {
	srv MCPServer
}

// NewStdio returns a Client communicating through stdio.
func NewStdio(srv MCPServer) Client {
	return &stdioClient{
		srv: srv,
	}
}

func (c *stdioClient) Start(ctx context.Context) (pipe *MCPStream, err error) {

	subctx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(subctx, c.srv.Command, c.srv.Args...) // #nosec: G204
	cmd.Env = append(os.Environ(), c.srv.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to create stderr pipe: %w", err)
	}

	inCh := make(chan []byte, 1024)
	go c.readRequests(subctx, stdin, inCh)

	outCh := make(chan []byte, 1024)
	go c.readResponses(subctx, stdout, outCh)

	errCh := make(chan []byte, 1024)
	go c.readErrors(subctx, stderr, errCh)

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("unable to start command: %w", err)
	}

	exitCh := make(chan error)
	go func() {
		defer cancel()
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
			_ = stdin.Close()
			return
		case data := <-ch:
			if _, err := stdin.Write(data); err != nil {
				if ctx.Err() == nil {
					slog.Error("Unable to write data to stdin", "err", err)
				}
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
			if ctx.Err() != nil || err == io.EOF {
				return
			}
			slog.Error("Unable to read response from stdout", "err", err)
			return
		}
		select {
		case ch <- data:
		case <-ctx.Done():
			return
		}
	}
}

func (c *stdioClient) readErrors(ctx context.Context, stderr io.ReadCloser, ch chan []byte) {

	bstderr := bufio.NewReader(stderr)
	for {
		line, err := bstderr.ReadBytes('\n')
		if err != nil {
			if ctx.Err() != nil || err == io.EOF {
				return
			}
			slog.Error("Unable to read error response from stderr", "err", err)
			return
		}
		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}
}
