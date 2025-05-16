package mcp

// An Error represents an inline MPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// NewError returns an *MCPError with code 500
// and the given error
func NewError(err error) *Error {
	return &Error{
		Code:    500,
		Message: err.Error(),
	}
}
