package policer

import (
	"context"
	"crypto/tls"

	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/mcp"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
	"go.acuvity.ai/minibridge/pkgs/policer/internal/http"
	"go.acuvity.ai/minibridge/pkgs/policer/internal/rego"
)

// A Policer is the interface of objects that can police request.
type Policer interface {
	Police(context.Context, api.Request) (*mcp.Message, error)
	Type() string
}

// NewRego returns a new rego based Policer.
func NewRego(policy string) (Policer, error) {
	return rego.New(policy)
}

// NewHTTP returns a new HTTP based Policer
func NewHTTP(endpoint string, auth *auth.Auth, tlsConfig *tls.Config) Policer {
	return http.New(endpoint, auth, tlsConfig)
}
