package client

// A MCPServer contains the information needed
// to launch an MCP Server.
type MCPServer struct {
	Command string
	Args    []string
	Env     []string
}

// MCPStream holds MCPServer command standard streams as channels.
type MCPStream struct {
	Stdin  chan []byte
	Stdout chan []byte
	Stderr chan []byte

	Exit chan error
}
