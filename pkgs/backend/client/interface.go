package client

import (
	"context"
	"net/http"

	"go.acuvity.ai/minibridge/pkgs/auth"
)

type cfg struct {
	auth *auth.Auth
}

type Option func(*cfg)

func OptionAuth(a *auth.Auth) Option {
	return func(c *cfg) {
		c.auth = a
	}
}

// A Client is the interface of object that can
// act as a minibridge mcp Client.
type Client interface {
	Start(context.Context, ...Option) (*MCPStream, error)
	Type() string
	Server() string
}

type RemoteClient interface {
	HTTPClient() *http.Client
	BaseURL() string
	Client
}
