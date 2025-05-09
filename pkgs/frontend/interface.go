package frontend

import (
	"context"
	"net/http"

	"go.acuvity.ai/minibridge/pkgs/auth"
	"go.acuvity.ai/minibridge/pkgs/info"
)

// A Frontend is the interface of object that can
// act as a minibridge Frontend.
type Frontend interface {

	// Start starts the frontend. It will run in background until
	// the given context is done.
	Start(context.Context, *auth.Auth) error

	// BackendURL returns the backend URL the frontend connects to.
	BackendURL() string

	// HTTPClient returns a client that can be used to communicate
	// with the backend.
	HTTPClient() *http.Client

	// BackendInfo queries the backend for information.
	BackendInfo() (info.Info, error)
}
