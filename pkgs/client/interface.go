package client

import "context"

// A Client is the interface of object that can
// act as a minibridge mcp Client.
type Client interface {
	Start(context.Context) (*MCPStream, error)
}
