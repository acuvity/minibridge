package utils

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/mitchellh/mapstructure"
	"go.acuvity.ai/elemental"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

func LoadSBOM(path string) (sbom SBOM, err error) {

	data, err := os.ReadFile(path) // #nosec: G304
	if err != nil {
		return sbom, fmt.Errorf("unable to load sbom file at '%s': %w", path, err)
	}

	if err := elemental.Decode(elemental.EncodingTypeJSON, data, &sbom); err != nil {
		return sbom, fmt.Errorf("unable to decode content of sbom file: %w", err)
	}

	return sbom, nil
}

// DumpTools dumps all the tools available from the given client.MCPStream.
func DumpTools(ctx context.Context, stream *client.MCPStream) (api.Tools, error) {

	if _, err := stream.Roundtrip(ctx, api.NewInitCall(api.ProtocolVersion20250326)); err != nil {
		return nil, fmt.Errorf("unable to send mcp request: %w", err)
	}

	call := api.NewMCPCall(1)
	call.Method = "tools/list"

	resps, err := stream.PRoundtrip(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("unable to send mcp request: %w", err)
	}

	tools := api.Tools{}

	for _, resp := range resps {

		if _, ok := resp.Result["tools"]; !ok {
			continue
		}

		ltools := api.Tools{}
		if err := mapstructure.Decode(resp.Result["tools"], &ltools); err != nil {
			return nil, fmt.Errorf("unable to convert to tools: %w", err)
		}

		tools = append(tools, ltools...)
	}

	return tools, nil
}

// HashTools will generate a hashfile of all the tools available in the given
// *client.MCPStream.
func HashTools(tools api.Tools) (Hashes, error) {

	hashes := []Hash{}
	for _, tool := range tools {

		h := Hash{
			Name: tool.Name,
			Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(tool.Description))),
		}

		for k, v := range tool.InputSchema {
			if k != "properties" {
				continue
			}
			for pk, pv := range v.(map[string]any) {

				pvv, ok := pv.(map[string]any)
				if !ok {
					continue
				}

				pdesc, ok := pvv["description"].(string)
				if !ok {
					continue
				}

				h.Params = append(h.Params, Hash{
					Name: pk,
					Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(pdesc))),
				})
			}
		}

		hashes = append(hashes, h)
	}

	return hashes, nil
}
