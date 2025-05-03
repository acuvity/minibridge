package backend

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/auth"
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

func parseBasicAuth(authString string) (a *auth.Auth, ok bool) {

	const prefix = "Basic "

	if len(authString) < len(prefix) || !strings.EqualFold(authString[:len(prefix)], prefix) {

		parts := strings.SplitN(authString, " ", 2)
		if len(parts) == 2 {
			a = auth.NewBearerAuth(parts[1])
			return a, true
		}
		return nil, false
	}

	c, err := base64.StdEncoding.DecodeString(authString[len(prefix):])
	if err != nil {
		return nil, false
	}

	cs := string(c)

	user, password, ok := strings.Cut(cs, ":")
	if !ok {
		return nil, false
	}

	return auth.NewBasicAuth(user, password), true
}

func hErr(w http.ResponseWriter, message string, code int, span trace.Span) {
	http.Error(w, message, code)
	span.SetStatus(codes.Error, message)
}
