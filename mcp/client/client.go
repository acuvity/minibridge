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
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
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

	client := &stdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stderr: stderr,
		stdout: stdout,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable to start command: %w", err)
	}

	return client, nil
}

func (c *stdioClient) Start(ctx context.Context) (in chan []byte, out chan []byte, err chan []byte) {

	in = make(chan []byte, 1024)
	go c.readRequests(ctx, in)

	out = make(chan []byte, 1024)
	go c.readResponses(ctx, out)

	err = make(chan []byte, 1024)
	go c.readErrors(ctx, err)

	return in, out, err
}

func (c *stdioClient) readRequests(ctx context.Context, ch chan []byte) {

	for {

		select {

		case <-ctx.Done():
			_ = c.stdin.Close()
			return

		case data := <-ch:
			if _, err := c.stdin.Write(data); err != nil {
				slog.Error("Unable to write data to stdin", "err", err)
			}
		}
	}
}

func (c *stdioClient) readResponses(ctx context.Context, ch chan []byte) {

	stdout := bufio.NewReader(c.stdout)

	for {

		select {

		case <-ctx.Done():
			_ = c.stdout.Close()
			return

		default:

			data, err := stdout.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					slog.Error("Unable to read response from stdout", "err", err)
				}
				continue
			}

			select {
			case ch <- data:
			default:
			}
		}
	}
}

func (c *stdioClient) readErrors(ctx context.Context, ch chan []byte) {

	stderr := bufio.NewReader(c.stderr)

	for {

		select {

		case <-ctx.Done():
			_ = c.stderr.Close()
			return

		default:

			line, err := stderr.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					slog.Error("Unable to read response from stdout", "err", err)
				}
				continue
			}

			select {
			case ch <- line:
			default:
			}
		}
	}
}
