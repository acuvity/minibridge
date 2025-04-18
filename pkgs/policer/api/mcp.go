package api

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
	ID      any            `json:"id,omitempty,omitzero"`
	Method  string         `json:"method,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *MCPError      `json:"error,omitempty"`
}

// NewMCPCall returns a MCPCall initialized with the given id.
func NewMCPCall(id int) MCPCall {
	c := MCPCall{
		JSONRPC: "2.0",
	}

	if id >= 0 {
		c.ID = id
	}

	return c
}

func NewInitCall(proto ProtocolVersion) MCPCall {
	return MCPCall{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": proto,
			"capabilities": map[string]any{
				"sampling": map[string]any{},
				"roots": map[string]any{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]any{
				"name":    "minibridge",
				"version": "1.0",
			},
		},
	}
}

type Tools []*Tool

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"InputSchema,omitempty"`
}

type Resources []*Resource

type Resource struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	Text        string `json:"text,omitempty"`
	Blob        []byte `json:"blob,omitempty"`
	URI         string `json:"uri,omitempty"`
}
