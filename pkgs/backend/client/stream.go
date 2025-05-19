package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/mcp"
)

// MCPStream holds the MCP Server stdio streams as channels.
//
// It only deals with []byte containing MCP messages. The data
// is not validated, in or out.
//
// It accepts input via the chan []byte returned by Stdin().
//
// To access stdout, or stderr or the exit channel, you must call
// Stdout(), Stderr() or Exit() to get a chan []byte you can
// pull data from.
//
// The returned channel is registered in a pool of other subcriber
// channels that will all receive a broadcast of the data when they arrive.
//
// These functions also return a func() that must be called to
// unregister the channel from the pool.
// Failure to do so will leak go routines.
//
// The consumers of the channels must be mindful of the rest of the system.
// All channel operations are blocking to avoid possibly complete out of order
// messages (responses before requests). So we you register a channel, be sure
// to consume as fast as possible.
type MCPStream struct {
	stdin  chan []byte
	stdout chan []byte
	stderr chan []byte
	exit   chan error

	outChs  map[chan []byte]struct{}
	errChs  map[chan []byte]struct{}
	exitChs map[chan error]struct{}

	sync.RWMutex
}

// NewMCPStream returns an initialized *MCPStream.
// It will start a listener in the background that will run
// ultimately up until the provided context cancels.
func NewMCPStream(ctx context.Context) *MCPStream {

	s := &MCPStream{
		stderr:  make(chan []byte),
		stdout:  make(chan []byte),
		stdin:   make(chan []byte),
		exit:    make(chan error),
		outChs:  map[chan []byte]struct{}{},
		errChs:  map[chan []byte]struct{}{},
		exitChs: map[chan error]struct{}{},
	}

	s.start(ctx)

	return s
}

// Stdin returns a channel that will accepts []byte
// containing MCP messages from the client.
func (s *MCPStream) Stdin() chan []byte {
	return s.stdin
}

// Stdout returns a channel that will produce []byte
// containing MCP messages from the MCP server.
// It also returns a function that must be called
// when the channel is not needed anymore.
// Failure to do so will leak go routines.
func (s *MCPStream) Stdout() (chan []byte, func()) {
	c := make(chan []byte, 8)
	s.registerOut(c)
	return c, func() { s.unregisterOut(c) }
}

// Stderr returns a channel that will produce []byte
// containing MCP Server logs.
// It also returns a function that must be called
// when the channel is not needed anymore.
// Failure to do so will leak go routines.
func (s *MCPStream) Stderr() (chan []byte, func()) {
	c := make(chan []byte, 8)
	s.registerErr(c)
	return c, func() { s.unregisterErr(c) }
}

// Exit returns a channel that will produce an error
// representing the end of the MCP server execution.
// Once a message is received from this channel,
// The MCPStream should be considered dead.
// It also returns a function that must be called
// when the channel is not needed anymore.
// Failure to do so will leak go routines.
func (s *MCPStream) Exit() (chan error, func()) {
	c := make(chan error, 1)
	s.registerExit(c)
	return c, func() { s.unregisterExit(c) }
}

// SendNotification sends a mcp.Message without waiting for a reply.
func (s *MCPStream) SendNotification(ctx context.Context, notif mcp.Notification) error {

	data, err := elemental.Encode(elemental.EncodingTypeJSON, notif)
	if err != nil {
		return fmt.Errorf("unable to encode mcp notification: %w", err)
	}

	select {
	case s.stdin <- data:
	case <-ctx.Done():
		return fmt.Errorf("unable to send mcp notification: %w", ctx.Err())
	}

	return nil
}

// SendRequest sends the given MCP request and waits for a MCP response related to the
// request ID, up until the provider context expires.
// The request is not validated.
func (s *MCPStream) SendRequest(ctx context.Context, req mcp.Message) (resp mcp.Message, err error) {

	data, err := elemental.Encode(elemental.EncodingTypeJSON, req)
	if err != nil {
		return resp, fmt.Errorf("unable to encode mcp call: %w", err)
	}

	stdout, unregister := s.Stdout()
	defer unregister()

	select {
	case s.stdin <- data:
	case <-ctx.Done():
		return resp, fmt.Errorf("unable to send request: %w", ctx.Err())
	}

	summary := struct {
		ID any `json:"id"`
	}{}

	for {
		select {

		case <-ctx.Done():
			return resp, fmt.Errorf("unable to get response: %w", ctx.Err())

		case data := <-stdout:

			if err := elemental.Decode(elemental.EncodingTypeJSON, data, &summary); err != nil {
				return req, fmt.Errorf("unable to decode mcp call as summary: %w", err)
			}

			if !mcp.RelatedIDs(req.ID, summary.ID) {
				continue
			}

			if err := elemental.Decode(elemental.EncodingTypeJSON, data, &resp); err != nil {
				return req, fmt.Errorf("unable to decode mcp call: %w", err)
			}

			return resp, nil
		}
	}
}

// SendPaginatedRequest works like SendRequest, but will retrieve the next pages until
// it reaches the end. All responses are returned at once in an slice of mcp.Message.
func (s *MCPStream) SendPaginatedRequest(ctx context.Context, msg mcp.Message) (out []mcp.Message, err error) {

	var resp mcp.Message

	currentRequest := msg

	for {

		resp, err = s.SendRequest(ctx, currentRequest)
		if err != nil {
			return nil, fmt.Errorf("unable to send paginated request: %w", err)
		}

		out = append(out, resp)

		cursor, ok := resp.Result["nextCursor"].(string)
		if !ok || cursor == "" {
			return out, nil
		}

		currentRequest = mcp.NewMessage("")
		currentRequest.ID = msg.ID
		currentRequest.Method = msg.Method
		currentRequest.Params = map[string]any{"cursor": cursor}
	}
}

func (s *MCPStream) registerOut(ch chan []byte)   { s.Lock(); s.outChs[ch] = struct{}{}; s.Unlock() }
func (s *MCPStream) unregisterOut(ch chan []byte) { s.Lock(); delete(s.outChs, ch); s.Unlock() }

func (s *MCPStream) registerErr(ch chan []byte)   { s.Lock(); s.errChs[ch] = struct{}{}; s.Unlock() }
func (s *MCPStream) unregisterErr(ch chan []byte) { s.Lock(); delete(s.errChs, ch); s.Unlock() }

func (s *MCPStream) registerExit(ch chan error)   { s.Lock(); s.exitChs[ch] = struct{}{}; s.Unlock() }
func (s *MCPStream) unregisterExit(ch chan error) { s.Lock(); delete(s.exitChs, ch); s.Unlock() }

func (s *MCPStream) start(ctx context.Context) {

	go func() {

		for {
			select {

			case data := <-s.stdout:

				s.RLock()
				for c := range s.outChs {
					select {
					case c <- data:
					default:
						slog.Error("stdout message dropped for registered channel")
					}
				}
				s.RUnlock()

			case data := <-s.stderr:

				s.RLock()
				for c := range s.errChs {
					select {
					case c <- data:
					default:
						slog.Error("stderr message dropped for registered channel")
					}
				}
				s.RUnlock()

			case err := <-s.exit:

				s.RLock()
				for c := range s.exitChs {
					select {
					case c <- err:
					default:
						slog.Error("exit message dropped for registered channel")
					}
				}
				s.RUnlock()

			case <-ctx.Done():

				var err error
				select {
				case err = <-s.exit:
				case <-time.After(time.Second):
					break
				}

				if err == nil {
					err = ctx.Err()
				}

				s.RLock()
				for c := range s.exitChs {
					select {
					case c <- err:
					default:
					}
				}
				s.RUnlock()
			}
		}
	}()
}
