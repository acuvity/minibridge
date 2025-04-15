package policer

import (
	"context"

	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

// A Policer is the interface of objects that can police request.
type Policer interface {
	Police(context.Context, api.Request) error
}
