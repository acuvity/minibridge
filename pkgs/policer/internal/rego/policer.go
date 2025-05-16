package rego

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"go.acuvity.ai/minibridge/pkgs/mcp"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type Policer struct {
	queryAllow   rego.PreparedEvalQuery
	queryReasons rego.PreparedEvalQuery
	queryMCP     rego.PreparedEvalQuery
}

const RegoRuntimeEnvPrefix = "REGO_POLICY_RUNTIME_"

// New returns a new Rego based Policer.
func New(policy string) (*Policer, error) {

	comp, err := precompile(policy, "default")
	if err != nil {
		return nil, fmt.Errorf("unable to compile rego policy: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rTerm := makeRegoRuntimeTerm()

	queryAllow, err := rego.New(rego.Compiler(comp), rego.Query("data.main.allow"), rego.Runtime(rTerm)).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego deny query: %w", err)
	}

	queryReasons, err := rego.New(rego.Compiler(comp), rego.Query("reasons := data.main.reasons"), rego.Runtime(rTerm)).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego deny query: %w", err)
	}

	queryMCP, err := rego.New(rego.Compiler(comp), rego.Query("mcp := data.main.mcp"), rego.Runtime(rTerm)).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego mcp query: %w", err)
	}

	return &Policer{
		queryAllow:   queryAllow,
		queryReasons: queryReasons,
		queryMCP:     queryMCP,
	}, nil
}

func (p *Policer) Type() string { return "rego" }

func (p *Policer) Police(ctx context.Context, preq api.Request) (*mcp.Message, error) {

	res, err := p.queryAllow.Eval(ctx, rego.EvalInput(preq), rego.EvalPrintHook(printer{}))
	if err != nil {
		return nil, fmt.Errorf("unable to eval allow query: %w", err)
	}

	if !res.Allowed() {

		res, err = p.queryReasons.Eval(ctx, rego.EvalInput(preq), rego.EvalPrintHook(printer{}))
		if err != nil {
			return nil, fmt.Errorf("unable to eval reasons query: %w", err)
		}

		reasons := []string{api.GenericDenyReason}

		if len(res) > 0 {
			bindings := res[0].Bindings
			breasons, _ := bindings["reasons"].([]any)

			if len(breasons) > 0 {
				reasons = make([]string, len(breasons))
				for i, v := range breasons {
					reasons[i], _ = v.(string)
				}
			}
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

	bindings := res[0].Bindings

	newmcp, ok := bindings["mcp"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid binding: mcp must be an map[string]any, got %T", bindings["mcp"])
	}

	mcall := &mcp.Message{}
	if err := mapstructure.Decode(newmcp, mcall); err != nil {
		return nil, fmt.Errorf("unable to decode rego mcp into valid MCP call: %w", err)
	}

	mcall.ID = preq.MCP.ID

	return mcall, nil
}

// makeRegoRuntimeTerm create a rego ast Term
// to expose prefixed env var to the rego runtime.
func makeRegoRuntimeTerm() *ast.Term {

	env := ast.NewObject()

	for _, s := range os.Environ() {
		parts := strings.SplitN(s, "=", 2)
		if !strings.HasPrefix(parts[0], RegoRuntimeEnvPrefix) {
			continue
		}
		if len(parts) == 2 {
			env.Insert(ast.StringTerm(strings.ReplaceAll(parts[0], RegoRuntimeEnvPrefix, "")), ast.StringTerm(parts[1]))
		}
	}

	obj := ast.NewObject()
	obj.Insert(ast.StringTerm("env"), ast.NewTerm(env))

	return ast.NewTerm(obj)
}
