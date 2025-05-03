package frontend

import (
	"fmt"

	"github.com/spaolacci/murmur3"
	"go.acuvity.ai/minibridge/pkgs/auth"
)

func hash(v string) uint64 {
	return murmur3.Sum64([]byte(v)) & 0x7FFFFFFFFFFFFFFF // #nosec G115
}

func hashCreds(auth *auth.Auth, authHeaders []string) uint64 {
	if auth != nil {
		return hash(fmt.Sprintf("%s-%v", auth.Encode(), authHeaders))
	}
	return hash(fmt.Sprintf("%v", authHeaders))
}
