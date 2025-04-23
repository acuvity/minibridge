package backend

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	return data
}

func parseBasicAuth(auth string) (password string, ok bool) {

	const prefix = "Basic "

	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			return parts[1], true
		}
		return "", false
	}

	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", false
	}

	cs := string(c)

	if _, password, ok = strings.Cut(cs, ":"); !ok {
		return "", false
	}

	return password, true
}

func hErr(w http.ResponseWriter, message string, code int, span trace.Span) {
	http.Error(w, message, code)
	span.SetStatus(codes.Error, message)
}
