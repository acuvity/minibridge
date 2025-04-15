package frontend

import (
	"fmt"

	"github.com/spaolacci/murmur3"
)

func hash(v string) uint64 {
	return murmur3.Sum64([]byte(v)) & 0x7FFFFFFFFFFFFFFF // #nosec G115
}

func hashCreds(token string, authHeaders []string) uint64 {
	return hash(fmt.Sprintf("%s-%v", token, authHeaders))
}
