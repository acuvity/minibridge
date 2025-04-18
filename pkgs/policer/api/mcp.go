package api

// NewMCPCall returns a MCPCall initialized with the given id.
func NewMCPCall(id int) MCPCall {
	return MCPCall{
		JSONRPC: "2.0",
		ID:      id,
	}
}

func NewInitCall(proto ProtocolVersion) MCPCall {
	return MCPCall{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": proto,
			"capabilities":    map[string]any{},
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
