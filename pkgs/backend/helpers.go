package backend

import (
	"log/slog"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

func makeMCPError(ID any, err error) []byte {

	mpcerr := api.MCPCall{
		JSONRPC: "2.0",
		ID:      ID,
		Error: &api.MCPError{
			Code:    451,
			Message: err.Error(),
		},
	}

	data, err := elemental.Encode(elemental.EncodingTypeJSON, mpcerr)
	if err != nil {
		panic(err)
	}

	slog.Debug("Injecting MCP error", "err", string(data))

	return append(data, '\n')
}
