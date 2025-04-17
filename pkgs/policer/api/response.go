package api

// A Response is returned by the Policer.
type Response struct {

	// Deny contains reasons for denying the request.
	Deny []string `json:"deny,omitzero"`

	// If non-zero, replace the request MCP call with
	// this one. This allows Policers to modify the content
	// of an MCP call.
	MCP *MCPCall `json:"mcp,omitempty"`
}
