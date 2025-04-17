package rego

import (
	"fmt"
	"log/slog"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/print"
)

type printer struct{}

func (p printer) Print(ctx print.Context, s string) error {
	slog.Info(fmt.Sprintf("Rego Print: %s", s), "row", ctx.Location.Row)
	return nil
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
