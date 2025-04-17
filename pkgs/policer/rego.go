package policer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown/print"
	"go.acuvity.ai/minibridge/pkgs/policer/api"
)

type printer struct{}

func (p printer) Print(ctx print.Context, s string) error {
	slog.Info(fmt.Sprintf("Rego Print: %s", s), "row", ctx.Location.Row)
	return nil
}

type regoPolicer struct {
	query rego.PreparedEvalQuery
}

// NewRego returns a new Rego based Policer.
func NewRego(policy string) (Policer, error) {

	comp, err := precompile(policy, "default")
	if err != nil {
		return nil, fmt.Errorf("unable to compile rego policy: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query, err := rego.New(
		rego.Compiler(comp),
		rego.Query("deny := data.main.deny"),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare rego query: %w", err)
	}

	return &regoPolicer{
		query: query,
	}, nil
}

func (p *regoPolicer) Police(ctx context.Context, preq api.Request) error {

	res, err := p.query.Eval(
		ctx,
		rego.EvalInput(preq),
		rego.EvalPrintHook(printer{}),
	)
	if err != nil {
		return fmt.Errorf("unable to eval rego: %w", err)
	}

	if len(res) == 0 {
		return fmt.Errorf("invalid evaluation results: empty bindings")
	}

	if res.Allowed() {
		return nil
	}

	bindings := res[0].Bindings

	breasons, ok := bindings["deny"].([]any)
	if !ok {
		return fmt.Errorf("invalid binding: allow must be an array, got %T", bindings["reasons"])
	}

	if len(breasons) == 0 {
		return nil
	}

	reasons := make([]string, len(breasons))
	for i, v := range breasons {
		reasons[i], _ = v.(string)
	}

	return fmt.Errorf("%w: %s", ErrBlocked, strings.Join(reasons, ", "))
}

func precompile(policy string, name string, modules ...*ast.Module) (*ast.Compiler, error) {

	name = name + ".rego"

	compiler := ast.NewCompiler().WithEnablePrintStatements(true)
	module, err := prepareModule("main", policy)
	if err != nil {
		return nil, err
	}

	allModules := map[string]*ast.Module{
		name: module,
	}
	for _, m := range modules {
		allModules[m.Package.String()+".rego"] = m
	}

	compiler.Compile(allModules)

	if compiler.Failed() {
		return nil, fmt.Errorf("unable compile rego module: %w", compiler.Errors)
	}

	return compiler, nil
}

func prepareModule(name string, policy string) (*ast.Module, error) {

	caps := ast.CapabilitiesForThisVersion()

	module, err := ast.ParseModuleWithOpts(
		name,
		policy,
		ast.ParserOptions{
			Capabilities: caps,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to parse rego module: %w", err)
	}

	return module, nil
}
