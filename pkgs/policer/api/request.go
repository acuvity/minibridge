package api

// RequestType type of request to the policer.
type RequestType string

// Various values of RequestType
var (
	Input  RequestType = "input"
	Output RequestType = "output"
)

// An MPCError represents an inline MPC error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// MPCCall represents the inline MPC request.
type MCPCall struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *MCPError      `json:"error,omitempty"`
}

// A Request represents the data sent to the Policer
type Request struct {

	// Token is the agent token that as been received by the backend.
	Token string `json:"token"`

	// Type of the request. Input will be set for request from the agent
	// and Output will be set for replies from the MPC server.
	Type RequestType `json:"type"`

	// MPC embeds the full MPC call, either request or response,
	// based on the Type.
	MCP MCPCall `json:"mcp,omitzero"`
}
