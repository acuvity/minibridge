package policer

import (
	"context"

	api "go.acuvity.ai/api/apex"
)

// A Policer is the interface of objects that can police request.
type Policer interface {
	Police(context.Context, api.PoliceRequestTypeValue, []byte) error
}
