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

func analyze(
	ctx context.Context,
	client *http.Client,
	apexURL string,
	apexToken string,
	data []byte,
) error {

	sreq := api.NewPoliceRequest()
	sreq.Type = api.PoliceRequestTypeInput
	sreq.Extractions = []*api.ExtractionRequest{
		{Data: data},
	}

	body, err := elemental.Encode(elemental.EncodingTypeJSON, sreq)
	if err != nil {
		return fmt.Errorf("unable to encode scan request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/_acuvity/police", apexURL),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("unable to create new http request: %w", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apexToken))

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
		return fmt.Errorf("invalid response from apex `%s`: %s", string(rbody), resp.Status)
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
		ID int `json:"id"`
	}{}
	_ = elemental.Decode(elemental.EncodingTypeJSON, data, &s)

	return fmt.Appendf([]byte{}, `{"jsonrpc":"2.0","id":%d,"error":{"code":451,"message":"%s"}}`, s.ID, err.Error())
}
