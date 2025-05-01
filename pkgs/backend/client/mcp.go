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
		return api.MCPCall{}, fmt.Errorf("unable to read response: %w", err)
	}

	return resp, nil
}

func (s *MCPStream) PRoundtrip(ctx context.Context, call api.MCPCall) (out []api.MCPCall, err error) {

	var resp api.MCPCall

	if err := s.Send(call); err != nil {
		return nil, fmt.Errorf("unable to send paginated request: %w", err)
	}

	for {
		for {
			if resp, err = s.Read(ctx); err != nil {
				return nil, fmt.Errorf("unable to read paginated response: %w", err)
			}

			if resp.ID != call.ID {
				continue
			}

			out = append(out, resp)
			break
		}

		cursor, ok := resp.Result["nextCursor"].(string)
		if !ok || cursor == "" {
			return out, nil
		}

		ncall := api.NewMCPCall("")
		ncall.ID = call.ID
		ncall.Method = call.Method
		ncall.Params = map[string]any{"cursor": cursor}

		if err = s.Send(ncall); err != nil {
			return nil, fmt.Errorf("unable to send next paginated request: %w", err)
		}
	}
}
