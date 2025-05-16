package mcp

type Notification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
}

// NewNotification returns a new notification.
func NewNotification(name string) Notification {
	return Notification{
		JSONRPC: "2.0",
		Method:  name,
	}
}
