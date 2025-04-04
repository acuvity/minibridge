package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	api "go.acuvity.ai/api/apex"
	"go.acuvity.ai/elemental"
)

var ErrBlocked = errors.New("request blocked")

func police(
	ctx context.Context,
	client *http.Client,
	policerURL string,
	policerToken string,
	rtype api.PoliceRequestTypeValue,
	data []byte,
) error {

	sreq := api.NewPoliceRequest()
	sreq.Type = rtype
	sreq.Messages = []string{strings.TrimSpace(string(data))}

	// TODO: add a way to let minibridge run the
	// extraction to retrieve the name/claims from the
	// request. This requires https://github.com/acuvity/acuvity/pull/1923
	//
	// sreq.User = &api.PoliceExternalUser{
	// 	Name:   "joe",
	// 	Claims: []string{"@org=acuvity.ai", "@scope=apps"},
	// }

	body, err := elemental.Encode(elemental.EncodingTypeJSON, sreq)
	if err != nil {
		return fmt.Errorf("unable to encode scan request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, policerURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("unable to create new http request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", policerToken))

	resp, err := client.Do(req)
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

func makeMCPError(data []byte, err error) []byte {

	s := struct {
		ID any `json:"id"`
	}{}
	_ = elemental.Decode(elemental.EncodingTypeJSON, data, &s)

	switch s.ID.(type) {
	case string:
		return fmt.Appendf([]byte{}, `{"jsonrpc":"2.0","id":"%s","error":{"code":451,"message":"%s"}}`, s.ID, err.Error())
	default:
		return fmt.Appendf([]byte{}, `{"jsonrpc":"2.0","id":%d,"error":{"code":451,"message":"%s"}}`, s.ID, err.Error())
	}
}
