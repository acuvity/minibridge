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

	api "go.acuvity.ai/api/apex"
	"go.acuvity.ai/elemental"
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

func (p *policer) Police(ctx context.Context, rtype api.PoliceRequestTypeValue, data []byte, user User) error {

	sreq := api.NewPoliceRequest()
	sreq.Type = rtype
	sreq.Messages = []string{strings.TrimSpace(string(data))}

	if user.Name != "" || len(user.Claims) > 0 {
		sreq.User = &api.PoliceExternalUser{
			Name:   user.Name,
			Claims: user.Claims,
		}
	}

	body, err := elemental.Encode(elemental.EncodingTypeJSON, sreq)
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

	sresp := api.NewScanResponse()
	if err := elemental.Decode(elemental.EncodingTypeJSON, rbody, sresp); err != nil {
		return fmt.Errorf("unable to decode response body: %w", err)
	}

	if sresp.Decision != api.ScanResponseDecisionAllow {
		return fmt.Errorf("%w: %s: %s", ErrBlocked, sresp.Decision, strings.Join(sresp.Reasons, ", "))
	}

	return nil
}
