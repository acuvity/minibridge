package backend

import (
	"context"
)

// A Backend is the interface of object that can
// act as a minibridge Backend.
type Backend interface {

	// Sarts starts the backend. It will run in background
	// until the given context is done.
	Start(context.Context) error
}
