package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type stdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	cancel context.CancelFunc

	errBuffer []byte
	errMu     sync.Mutex
}

// NewStdio returns a Client communicating through stdio.
func NewStdio(srv MCPServer) (Client, error) {

	cmd := exec.Command(srv.Command, srv.Args...)
	cmd.Env = append(os.Environ(), srv.Env...)

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

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start command: %w", err)
	}

	client := &stdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stderr: stderr,
		stdout: stdout,
	}

	return client, nil
}

func (c *stdioClient) Start(ctx context.Context) (in chan []byte, out chan []byte, err chan []byte) {

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go c.resetErrBuffer(ctx, 5*time.Second)

	in = make(chan []byte, 1024)
	go c.readRequests(ctx, in)

	out = make(chan []byte, 1024)
	go c.readResponses(ctx, out)

	err = make(chan []byte, 1024)
	go c.readErrors(ctx, err)

	go c.monitorProcess(ctx)

	return in, out, err
}

func (c *stdioClient) monitorProcess(ctx context.Context) {
	done := make(chan error, 1)

	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		slog.Warn("Context canceled, killing process", "cmd", c.cmd.Args[0])

		if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			slog.Warn("Failed to send SIGTERM", "cmd", c.cmd.Args[0], "err", err)
		}

		select {
		case err := <-done:
			c.handleExit(err)
		case <-time.After(5 * time.Second):
			slog.Warn("Process didn't exit, killing", "cmd", c.cmd.Args[0])
			_ = c.cmd.Process.Kill()
			c.handleExit(<-done)
		}

	case err := <-done:
		c.handleExit(err)
	}

	if c.cancel != nil {
		c.cancel()
	}
}

func (c *stdioClient) handleExit(err error) {
	c.errMu.Lock()
	errContent := string(c.errBuffer)
	c.errMu.Unlock()

	if err != nil {
		if _, ok := slog.Default().Handler().(*slog.JSONHandler); ok {
			slog.Error("Command crashed", "cmd", c.cmd.Args[0], "err", err, "stderr", errContent)
		} else {
			slog.Error("Command crashed", "cmd", c.cmd.Args[0], "err", err)
			fmt.Fprintln(os.Stderr, errContent)
		}
	} else {
		slog.Warn("Command exited normally", "cmd", c.cmd.Args[0])
	}
}

func (c *stdioClient) readRequests(ctx context.Context, ch chan []byte) {

	for {
		select {
		case <-ctx.Done():
			_ = c.stdin.Close()
			return
		case data := <-ch:
			if _, err := c.stdin.Write(data); err != nil {
				if ctx.Err() == nil {
					slog.Error("Unable to write data to stdin", "err", err)
				}
				return
			}
		}
	}
}

func (c *stdioClient) readResponses(ctx context.Context, ch chan []byte) {

	stdout := bufio.NewReader(c.stdout)
	for {
		data, err := stdout.ReadBytes('\n')
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

func (c *stdioClient) readErrors(ctx context.Context, ch chan []byte) {

	stderr := bufio.NewReader(c.stderr)

	defer func(stderr *bufio.Reader) {

		flush, _ := io.ReadAll(stderr)
		c.errMu.Lock()
		c.errBuffer = append(c.errBuffer, flush...)
		c.errMu.Unlock()

	}(stderr)

	for {
		line, err := stderr.ReadBytes('\n')
		if err != nil {
			if ctx.Err() != nil || err == io.EOF {
				return
			}
			slog.Error("Unable to read error response from stderr", "err", err)
			return
		}

		c.errMu.Lock()
		c.errBuffer = append(c.errBuffer, line...)
		c.errMu.Unlock()

		select {
		case ch <- line:
		case <-ctx.Done():
			return
		}
	}
}

func (c *stdioClient) resetErrBuffer(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.errMu.Lock()
			c.errBuffer = nil
			c.errMu.Unlock()
		}
	}
}
