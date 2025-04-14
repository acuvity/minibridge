package policer

import (
	"context"

	api "go.acuvity.ai/api/apex"
)

type User struct {
	Name   string
	Claims []string
}

// A Policer is the interface of objects that can police request.
type Policer interface {
	Police(context.Context, api.PoliceRequestTypeValue, []byte, User) error
}
