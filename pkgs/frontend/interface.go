package frontend

import "context"

// A Frontend is the interface of object that can
// act as a minibridge Frontend.
type Frontend interface {
	Start(context.Context) error
}
