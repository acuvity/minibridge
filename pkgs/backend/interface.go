package backend

import "context"

// A Backend is the interface of object that can
// act as a minibridge Backend.
type Backend interface {
	Start(context.Context) error
}
