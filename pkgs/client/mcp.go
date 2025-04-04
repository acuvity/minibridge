package client

// A MCPServer contains the information needed
// to launch an MCP Server.
type MCPServer struct {
	Command string
	Args    []string
	Env     []string
}
