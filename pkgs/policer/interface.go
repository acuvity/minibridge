package policer

import (
	"context"
	"errors"

	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

var ErrBlocked = errors.New("request blocked")

// A Policer is the interface of objects that can police request.
type Policer interface {
	Police(context.Context, api.Request) error
}
