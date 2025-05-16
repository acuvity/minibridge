package client

import (
	"fmt"
	"os/exec"
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
