package api

import "time"

// CallType type of request to the policer.
type CallType string

// Various values of RequestType
var (
	CallTypeRequest  CallType = "request"
	CallTypeResponse CallType = "response"
)

// SpanContext contains information about the OTEL span
// related to a Request.
type SpanContext struct {
	TraceID      string    `json:"traceID" `
	ParentSpanID string    `json:"parentSpanID,omitempty"`
	End          time.Time `json:"end"`
	ID           string    `json:"ID"`
	Name         string    `json:"name"`
	Start        time.Time `json:"start"`
}

func (c SpanContext) IsValid() bool {
	return c.TraceID != "" && c.ID != ""
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

	// SpanContext contains info about the eventual OTEL span
	// for the request. There are advanced use cases where you
	// want correlation between a Request and the OTEL traces
	// associated.
	SpanContext SpanContext `json:"spanContext,omitzero"`
}

// Agent contains information about the caller of the request.
type Agent struct {

	// User contains the user from the Auth header.
	User string `json:"user"`

	// Password contains the password from the Auth header.
	Password string `json:"password"`

	// RemoteAddr contains the agent's RemoteAddr, as seen by minibridge.
	RemoteAddr string `json:"remoteAddr"`

	// User Agent contains the user agent field of the agent.
	UserAgent string `json:"userAgent,omitempty"`
}
