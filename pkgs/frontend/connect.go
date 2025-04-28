package frontend

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"go.acuvity.ai/wsc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// AgentInfo holds information about the agent
// who sent an MCPCall.
type AgentInfo struct {
	Token       string
	AuthHeaders []string
	UserAgent   string
	RemoteAddr  string
}

// Connect is a low level function to connect to the backend's websocket
func Connect(
	ctx context.Context,
	dialer func(ctx context.Context, network, addr string) (net.Conn, error),
	backendURL string,
	tlsConfig *tls.Config,
	info AgentInfo,
) (wsc.Websocket, error) {

	slog.Debug("New websocket connection",
		"url", backendURL,
		"using-token", info.Token != "",
		"using-headers", len(info.AuthHeaders) > 0,
		"tls", strings.HasPrefix(backendURL, "wss://"),
		"tls-config", tlsConfig != nil,
	)

	if dialer == nil && (info.Token != "" || len(info.AuthHeaders) > 0) && tlsConfig == nil {
		slog.Warn("Security: connecting to a websocket with crendentials sent over the network in clear-text. Refused. Credentials have been stripped. Request will proceed and will likely fail.")
	}

	wsconfig := wsc.Config{
		WriteChanSize:      64,
		ReadChanSize:       16,
		TLSConfig:          tlsConfig,
		NetDialContextFunc: dialer,
	}

	wsconfig.Headers = http.Header{
		"X-Forwarded-UA":  {info.UserAgent},
		"X-Forwarded-For": {info.RemoteAddr},
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(wsconfig.Headers))

	if tlsConfig != nil || dialer != nil {
		if info.Token != "" {
			wsconfig.Headers["Authorization"] = []string{"Basic " + base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "Bearer:%s", info.Token))}
		} else if len(info.AuthHeaders) > 0 {
			wsconfig.Headers["Authorization"] = info.AuthHeaders
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

		slog.Error("WS connection failed", "code", code, "status", status, "data", strings.TrimSpace(string(data)), err)

		return nil, fmt.Errorf("unable to connect to the websocket. code: %d, status: %s: %w", code, status, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("invalid response from other end of the tunnel (must be 101): %s", resp.Status)
	}

	return session, nil
}
