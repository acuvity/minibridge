package frontend

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"go.acuvity.ai/wsc"
)

func connectWS(ctx context.Context, backendURL string, tlsConfig *tls.Config) (wsc.Websocket, error) {

	session, resp, err := wsc.Connect(
		ctx,
		backendURL,
		wsc.Config{
			WriteChanSize: 64,
			ReadChanSize:  16,
			TLSConfig:     tlsConfig,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("unable to connect to the websocket '%s': %w", backendURL, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("invalid response from other end of the tunnel (must be 101): %s", resp.Status)
	}

	return session, nil
}
