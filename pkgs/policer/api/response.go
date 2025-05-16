package api

import "go.acuvity.ai/minibridge/pkgs/mcp"

// GenericDenyReason is the generic reason returned if non is provided.
const GenericDenyReason = "You are not allowed to perform this operation"

// A Response is returned by the Policer.
type Response struct {

	// If true, the request is allowed. Otherwise
	// it will be rejected for the reasons set in
	// Messages.
	Allow bool `json:"allow"`

	// Reasons contains reasons for denying the request.
	// If no message is given, and Allow is false, a generic
	// forbidden message will be used.
	Reasons []string `json:"reasons,omitempty"`

	// If non-zero, replace the request MCP call with
	// this one. This allows Policers to modify the content
	// of an MCP call.
	MCP *mcp.Message `json:"mcp,omitempty"`
}
