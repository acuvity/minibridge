package policer

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

type httpPolicer struct {
	endpoint string
	token    string
	client   *http.Client
}

// NewHTTP returns a new HTTP based Policer.
func NewHTTP(endpoint string, token string, tlsConfig *tls.Config) Policer {

	return &httpPolicer{
		endpoint: endpoint,
		token:    token,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

func (p *httpPolicer) Police(ctx context.Context, preq api.Request) error {

	body, err := elemental.Encode(elemental.EncodingTypeJSON, preq)
	if err != nil {
		return fmt.Errorf("unable to encode scan request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("unable to create new http request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.token))

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to send request: %w", err)
	}

	rbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response from policer `%s`: %s", string(rbody), resp.Status)
	}

	sresp := api.Response{}
	if err := elemental.Decode(elemental.EncodingTypeJSON, rbody, &sresp); err != nil {
		return fmt.Errorf("unable to decode response body: %w", err)
	}

	if len(sresp.Deny) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrBlocked, strings.Join(sresp.Deny, ", "))
}
