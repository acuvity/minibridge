package api

// CallType type of request to the policer.
type CallType string

// Various values of RequestType
var (
	CallTypeRequest CallType = "request"
	CallTypeOutput  CallType = "response"
)

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
