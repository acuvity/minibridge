package policer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

var ErrBlocked = errors.New("request blocked")

type policer struct {
	endpoint string
	token    string
	client   *http.Client
}

// New returns a new Policer.
func New(endpoint string, token string, tlsConfig *tls.Config) Policer {

	return &policer{
		endpoint: endpoint,
		token:    token,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

func (p *policer) Police(ctx context.Context, preq api.Request) error {

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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response from policer `%s`: %s", string(rbody), resp.Status)
	}

	sresp := api.Response{}
	if err := elemental.Decode(elemental.EncodingTypeJSON, rbody, &sresp); err != nil {
		return fmt.Errorf("unable to decode response body: %w", err)
	}

	// We check with equalfolds to be compatible with an exiting API.
	if !strings.EqualFold(string(sresp.Decision), string(api.DecisionAllow)) {
		return fmt.Errorf("%w: %s: %s", ErrBlocked, sresp.Decision, strings.Join(sresp.Reasons, ", "))
	}

	return nil
}
