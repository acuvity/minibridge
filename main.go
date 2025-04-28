package main

import (
	"context"

	"go.acuvity.ai/minibridge/cli"
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli.Main(ctx)
}
