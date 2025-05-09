package frontend

import (
	"fmt"
	"io"
	"net/http"

	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/info"
)

func getBackendInfo(mfrontend Frontend) (info.Info, error) {

	inf := info.Info{}
	cl := mfrontend.HTTPClient()

	resp, err := cl.Get(fmt.Sprintf("%s/_info", mfrontend.BackendURL()))
	if err != nil {
		return inf, fmt.Errorf("unable to make backend info request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return inf, fmt.Errorf("invalid backend info response status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return inf, fmt.Errorf("unable to read backend info response body: %w", err)
	}

	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &inf); err != nil {
		return inf, fmt.Errorf("unable to decode backend info response body: %w", err)
	}

	return inf, nil
}
