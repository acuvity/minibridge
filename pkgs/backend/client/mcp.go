package client

import (
	"context"
	"fmt"
	"os/exec"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

// A MCPServer contains the information needed
// to launch an MCP Server.
type MCPServer struct {
	Command string
	Args    []string
	Env     []string
}

// NewMCPServer returtns a new MCPServer. Returns an error is the given cmd path
// does not exist.
func NewMCPServer(path string, args ...string) (MCPServer, error) {
	p, err := exec.LookPath(path)
	if err != nil {
		return MCPServer{}, fmt.Errorf("unable to find server binary: %w", err)
	}
	return MCPServer{
		Command: p,
		Args:    args,
	}, nil
}

// MCPStream holds MCPServer command standard streams as channels.
type MCPStream struct {
	Stdin  chan []byte
	Stdout chan []byte
	Stderr chan []byte

	Exit chan error
}

// Send sends the given api.MCPCall to the Stdin stream.
func (s *MCPStream) Send(call api.MCPCall) error {

	data, err := elemental.Encode(elemental.EncodingTypeJSON, call)
	if err != nil {
		return fmt.Errorf("unable to encode mcp call: %w", err)
	}

	s.Stdin <- data

	return nil
}

// Read reads the next MCP responsde.
func (s *MCPStream) Read(ctx context.Context) (api.MCPCall, error) {

	call := api.MCPCall{}

	var data []byte

	select {
	case data = <-s.Stdout:
	case <-ctx.Done():
		return call, ctx.Err()
	}

	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &call); err != nil {
		return call, fmt.Errorf("read: unable to decode mcp call: %w", err)
	}

	return call, nil
}

func (s *MCPStream) Roundtrip(ctx context.Context, call api.MCPCall) (api.MCPCall, error) {

	if err := s.Send(call); err != nil {
		return api.MCPCall{}, fmt.Errorf("unable to send request: %w", err)
	}

	resp, err := s.Read(ctx)
	if err != nil {
		return api.MCPCall{}, fmt.Errorf("unable to read request: %w", err)
	}

	return resp, nil
}

func (s *MCPStream) PRoundtrip(ctx context.Context, call api.MCPCall) ([]api.MCPCall, error) {

	respCh := make(chan api.MCPCall)
	errCh := make(chan error)

	out := []api.MCPCall{}
	origID := call.ID

	go func() {

		var cursor string
		for i := 0; ; i++ {

			if cursor != "" {
				if call.Params == nil {
					call.Params = map[string]any{}
				}

				call.Params["cursor"] = cursor
			}

			if i > 0 {
				if id, ok := origID.(int); ok {
					id += i
					call.ID = id
				} else if _, ok := origID.(string); ok {
					call.ID = fmt.Sprintf("%s-%d", origID, i)
				}
			}

			if err := s.Send(call); err != nil {
				errCh <- fmt.Errorf("unable to send request: %w", err)
			}

			resp, err := s.Read(ctx)
			if err != nil {
				errCh <- fmt.Errorf("unable to read request: %w", err)
			}

			ncursor, ok := resp.Result["nextCursor"].(string)
			if ok && ncursor != "" {
				cursor = ncursor
			}

			respCh <- resp

			if ncursor == "" {
				close(respCh)
				break
			}
		}
	}()

	for {
		select {
		case resp, ok := <-respCh:
			if !ok {
				return out, nil
			}
			out = append(out, resp)
		case err := <-errCh:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
