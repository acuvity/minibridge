package frontend

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"go.acuvity.ai/wsc"
)

func connectWS(ctx context.Context, backendURL string, tlsConfig *tls.Config, token string, authHeaders []string) (wsc.Websocket, error) {

	slog.Debug("New websocket connection",
		"url", backendURL,
		"using-token", token != "",
		"using-headers", len(authHeaders) > 0,
		"tls", tlsConfig != nil,
	)

	if (token != "" || len(authHeaders) > 0) && tlsConfig == nil {
		slog.Warn("Security: connecting to a websocket with crendentials sent over the network in clear-text. Refused. Credentials have been stripped. Request will proceed and will likely fail.")
	}

	wsconfig := wsc.Config{
		WriteChanSize: 64,
		ReadChanSize:  16,
		TLSConfig:     tlsConfig,
	}

	if tlsConfig != nil {
		if token != "" {
			wsconfig.Headers = http.Header{
				"Authorization": {"Basic " + base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "Bearer:%s", token))},
			}
		} else if len(authHeaders) > 0 {
			wsconfig.Headers = http.Header{"Authorization": authHeaders}
		}
	}

	session, resp, err := wsc.Connect(ctx, backendURL, wsconfig)

	if err != nil {

		var data []byte
		var code int
		status := "<empty>"

		if resp != nil {
			data, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			code = resp.StatusCode
			status = resp.Status
		}

		slog.Debug("WS connection failed", "code", code, "status", status, "data", string(data))

		return nil, fmt.Errorf("unable to connect to the websocket. code: %d, status: %s: %w", code, status, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("invalid response from other end of the tunnel (must be 101): %s", resp.Status)
	}

	return session, nil
}
