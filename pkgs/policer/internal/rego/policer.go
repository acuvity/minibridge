package rego

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/open-policy-agent/opa/v1/rego"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type Policer struct {
	queryDeny rego.PreparedEvalQuery
	queryMCP  rego.PreparedEvalQuery
}

// New returns a new Rego based Policer.
func New(policy string) (*Policer, error) {

	comp, err := precompile(policy, "default")
	if err != nil {
		return nil, fmt.Errorf("unable to compile rego policy: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryDeny, err := rego.New(rego.Compiler(comp), rego.Query("deny := data.main.deny")).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego deny query: %w", err)
	}

	queryMCP, err := rego.New(rego.Compiler(comp), rego.Query("mcp := data.main.mcp")).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego mcp query: %w", err)
	}

	return &Policer{
		queryDeny: queryDeny,
		queryMCP:  queryMCP,
	}, nil
}

func (p *Policer) Police(ctx context.Context, preq api.Request) (*api.MCPCall, error) {

	res, err := p.queryDeny.Eval(ctx, rego.EvalInput(preq), rego.EvalPrintHook(printer{}))
	if err != nil {
		return nil, fmt.Errorf("unable to eval deny query: %w", err)
	}

	if len(res) == 0 || res.Allowed() {
		return nil, nil
	}

	bindings := res[0].Bindings

	denies, ok := bindings["deny"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid binding: deny must be an array, got %T", bindings["reasons"])
	}

	if len(denies) > 0 {

		reasons := make([]string, len(denies))
		for i, v := range denies {
			reasons[i], _ = v.(string)
		}

		return nil, fmt.Errorf("%w: %s", api.ErrBlocked, strings.Join(reasons, ", "))
	}

	res, err = p.queryMCP.Eval(ctx, rego.EvalInput(preq), rego.EvalPrintHook(printer{}))
	if err != nil {
		return nil, fmt.Errorf("unable to eval mcp query: %w", err)
	}

	if len(res) == 0 {
		return nil, nil
	}

	bindings = res[0].Bindings

	mcp, ok := bindings["mcp"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid binding: mcp must be an map[string]any, got %T", bindings["mcp"])
	}

	mcall := &api.MCPCall{}
	if err := mapstructure.Decode(mcp, mcall); err != nil {
		return nil, fmt.Errorf("unable to decode rego mcp into valid MCP call: %w", err)
	}

	return mcall, nil
}
