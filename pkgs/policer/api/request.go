package api

// CallType type of request to the policer.
type CallType string

// Various values of RequestType
var (
	CallTypeRequest CallType = "request"
	CallTypeOutput  CallType = "response"
)

type ProtocolVersion string

var (
	ProtocolVersion20250326 ProtocolVersion = "2025-03-26"
	ProtocolVersion20241105 ProtocolVersion = "2024-11-05"
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

	// Type of the request. Request will be set for request from the agent
	// and Response will be set for replies from the MPC server.
	Type CallType `json:"type"`

	// MPC embeds the full MPC call, either request or response,
	// based on the Type.
	MCP MCPCall `json:"mcp,omitzero"`

	// Agent contains callers information.
	Agent Agent `json:"agent,omitzero"`
}

// Agent contains information about the caller of the request.
type Agent struct {

	// Token is the agent token that as been received by the backend.
	Token string `json:"token"`

	// RemoteAddr contains the agent's RemoteAddr, as seen by minibridge.
	RemoteAddr string `json:"remoteAddr"`

	// User Agent contains the user agent field of the agent.
	UserAgent string `json:"userAgent,omitempty"`
}
