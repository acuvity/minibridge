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

type agentInfo struct {
	token       string
	authHeaders []string
	userAgent   string
	remoteAddr  string
}

func connectWS(ctx context.Context, backendURL string, tlsConfig *tls.Config, info agentInfo) (wsc.Websocket, error) {

	slog.Debug("New websocket connection",
		"url", backendURL,
		"using-token", info.token != "",
		"using-headers", len(info.authHeaders) > 0,
		"tls", tlsConfig != nil,
	)

	if (info.token != "" || len(info.authHeaders) > 0) && tlsConfig == nil {
		slog.Warn("Security: connecting to a websocket with crendentials sent over the network in clear-text. Refused. Credentials have been stripped. Request will proceed and will likely fail.")
	}

	wsconfig := wsc.Config{
		WriteChanSize: 64,
		ReadChanSize:  16,
		TLSConfig:     tlsConfig,
	}

	wsconfig.Headers = http.Header{
		"X-Forwarded-UA":  {info.userAgent},
		"X-Forwarded-For": {info.remoteAddr},
	}

	if tlsConfig != nil {
		if info.token != "" {
			wsconfig.Headers["Authorization"] = []string{"Basic " + base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "Bearer:%s", info.token))}
		} else if len(info.authHeaders) > 0 {
			wsconfig.Headers["Authorization"] = info.authHeaders
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
