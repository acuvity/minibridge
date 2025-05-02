package scan

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/mitchellh/mapstructure"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type Dump struct {
	Tools             api.Tools             `json:"tools,omitempty"`
	Resources         api.Resources         `json:"resources,omitempty"`
	ResourceTemplates api.ResourceTemplates `json:"resourceTemplates,omitempty"`
	Prompts           api.Prompts           `json:"prompts,omitempty"`
}

// Dump dumps all the all available tools/resource/prompts from the given client.MCPStream.
func DumpAll(ctx context.Context, stream *client.MCPStream) (Dump, error) {

	if _, err := stream.Roundtrip(ctx, api.NewInitCall(api.ProtocolVersion20250326)); err != nil {
		return Dump{}, fmt.Errorf("unable to send mcp request: %w", err)
	}

	notif := api.NewMCPCall("")
	notif.Method = "notifications/initialized"
	if err := stream.Send(ctx, notif); err != nil {
		return Dump{}, fmt.Errorf("unable to send mcp inititlized notif: %w", err)
	}

	dump := Dump{}

	// Tools

	toolsReq := api.NewMCPCall(uuid.Must(uuid.NewV7()).String())
	toolsReq.Method = "tools/list"
	resps, err := stream.PRoundtrip(ctx, toolsReq)
	if err != nil {
		return Dump{}, fmt.Errorf("unable to send tools/list mcp request: %w", err)
	}

	for _, resp := range resps {

		if _, ok := resp.Result["tools"]; !ok {
			continue
		}

		tools := api.Tools{}
		if err := mapstructure.Decode(resp.Result["tools"], &tools); err != nil {
			return Dump{}, fmt.Errorf("unable to convert to tools: %w", err)
		}

		dump.Tools = append(dump.Tools, tools...)
	}

	// Resources
	resourcesReq := api.NewMCPCall(uuid.Must(uuid.NewV7()).String())
	resourcesReq.Method = "resources/list"
	resps, err = stream.PRoundtrip(ctx, resourcesReq)
	if err != nil {
		return Dump{}, fmt.Errorf("unable to send resources/list mcp request: %w", err)
	}

	for _, resp := range resps {

		if _, ok := resp.Result["resources"]; !ok {
			continue
		}

		resources := api.Resources{}
		if err := mapstructure.Decode(resp.Result["resources"], &resources); err != nil {
			return Dump{}, fmt.Errorf("unable to convert to resources: %w", err)
		}

		dump.Resources = append(dump.Resources, resources...)
	}

	// Resources Templates
	resourcesTemplateReq := api.NewMCPCall(uuid.Must(uuid.NewV7()).String())
	resourcesTemplateReq.Method = "resources/templates/list"
	resps, err = stream.PRoundtrip(ctx, resourcesTemplateReq)
	if err != nil {
		return Dump{}, fmt.Errorf("unable to send resources/templates/list mcp request: %w", err)
	}

	for _, resp := range resps {

		if _, ok := resp.Result["resourceTemplates"]; !ok {
			continue
		}

		resourceTemplates := api.ResourceTemplates{}
		if err := mapstructure.Decode(resp.Result["resourceTemplates"], &resourceTemplates); err != nil {
			return Dump{}, fmt.Errorf("unable to convert to resources templates: %w", err)
		}

		dump.ResourceTemplates = append(dump.ResourceTemplates, resourceTemplates...)
	}

	// Prompts
	promptsReq := api.NewMCPCall(uuid.Must(uuid.NewV7()).String())
	promptsReq.Method = "prompts/list"
	resps, err = stream.PRoundtrip(ctx, promptsReq)
	if err != nil {
		return Dump{}, fmt.Errorf("unable to send prompts/list mcp request: %w", err)
	}

	for _, resp := range resps {

		if _, ok := resp.Result["prompts"]; !ok {
			continue
		}

		prompts := api.Prompts{}
		if err := mapstructure.Decode(resp.Result["prompts"], &prompts); err != nil {
			return Dump{}, fmt.Errorf("unable to convert to prompts: %w", err)
		}

		dump.Prompts = append(dump.Prompts, prompts...)
	}

	return dump, nil
}

// HashTools will generate Hashes for the given api.Tools
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

		slices.SortFunc(h.Params, func(a Hash, b Hash) int {
			return strings.Compare(a.Name, b.Name)
		})

		hashes = append(hashes, h)
	}

	slices.SortFunc(hashes, func(a Hash, b Hash) int {
		return strings.Compare(a.Name, b.Name)
	})

	return hashes, nil
}

// HashPrompt generate Hashes for the given api.Prompt
func HashPrompts(prompts api.Prompts) (Hashes, error) {

	hashes := []Hash{}
	for _, tool := range prompts {

		h := Hash{
			Name: tool.Name,
			Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(tool.Description))),
		}

		for _, p := range tool.Arguments {

			h.Params = append(h.Params, Hash{
				Name: p.Name,
				Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(p.Description))),
			})
		}

		slices.SortFunc(h.Params, func(a Hash, b Hash) int {
			return strings.Compare(a.Name, b.Name)
		})

		hashes = append(hashes, h)
	}

	slices.SortFunc(hashes, func(a Hash, b Hash) int {
		return strings.Compare(a.Name, b.Name)
	})

	return hashes, nil
}
