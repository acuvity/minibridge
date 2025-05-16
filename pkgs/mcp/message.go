package mcp

type ProtocolVersion string

var (
	ProtocolVersion20250326 ProtocolVersion = "2025-03-26"
	ProtocolVersion20241105 ProtocolVersion = "2024-11-05"
)

// Message represents the inline MPC request.
type Message struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty,omitzero"`
	Method  string         `json:"method,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *Error         `json:"error,omitempty"`
}

// NewMessage returns a MCPCall initialized with the given id.
// To initialize a call without ID set, use an empty string.
func NewMessage[T int | string](id T) Message {
	c := Message{
		JSONRPC: "2.0",
	}

	var zero T
	if id != zero {
		c.ID = id
	}

	return c
}

// IDString returns the call ID as a string
// whatever is the original type.
func (c *Message) IDString() string {

	if c.ID == nil {
		return ""
	}

	return normalizeID(c.ID)
}

// NewInitMessage makes a new init call using the given protocol version.
func NewInitMessage(proto ProtocolVersion) Message {
	return Message{
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
