package backend

import (
	"fmt"

	"go.acuvity.ai/elemental"
)

func makeMCPError(data []byte, err error) []byte {

	s := struct {
		ID any `json:"id"`
	}{}
	_ = elemental.Decode(elemental.EncodingTypeJSON, data, &s)

	switch s.ID.(type) {
	case string:
		return fmt.Appendf([]byte{}, `{"jsonrpc":"2.0","id":"%s","error":{"code":451,"message":"%s"}}`, s.ID, err.Error())
	default:
		return fmt.Appendf([]byte{}, `{"jsonrpc":"2.0","id":%d,"error":{"code":451,"message":"%s"}}`, s.ID, err.Error())
	}
}
