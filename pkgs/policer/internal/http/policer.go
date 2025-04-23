package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type Policer struct {
	endpoint string
	token    string
	client   *http.Client
}

// New returns a new HTTP based Policer.
func New(endpoint string, token string, tlsConfig *tls.Config) *Policer {

	return &Policer{
		endpoint: endpoint,
		token:    token,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

func (p *Policer) Type() string { return "http" }

func (p *Policer) Police(ctx context.Context, preq api.Request) (*api.MCPCall, error) {

	body, err := elemental.Encode(elemental.EncodingTypeJSON, preq)
	if err != nil {
		return nil, fmt.Errorf("unable to encode scan request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("unable to create new http request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.token))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send request: %w", err)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	rbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response from policer `%s`: %s", string(rbody), resp.Status)
	}

	sresp := api.Response{}
	if err := elemental.Decode(elemental.EncodingTypeJSON, rbody, &sresp); err != nil {
		return nil, fmt.Errorf("unable to decode response body: %w", err)
	}

	if sresp.Allow {
		return sresp.MCP, nil
	}

	if len(sresp.Reasons) == 0 {
		sresp.Reasons = []string{api.GenericDenyReason}
	}

	return nil, fmt.Errorf("%w: %s", api.ErrBlocked, strings.Join(sresp.Reasons, ", "))
}
