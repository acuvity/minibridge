package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/scan"
)

func init() {

	initSharedFlagSet()

	Scan.Flags().AddFlagSet(fBackend)
}

// Scan is the cobra command to run the server.
var Scan = &cobra.Command{
	Use:              "scan [dump|sbom|check file.sbom] -- command [args...]",
	Short:            "Scan an MCP server for resources, prompts, etc and generate sbom",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	Args:             cobra.MinimumNArgs(2),

	RunE: func(cmd *cobra.Command, args []string) error {

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		var mcpServer client.MCPServer
		var err error
		if args[0] == "check" {
			mcpServer, err = client.NewMCPServer(args[2], args[3:]...)
		} else {
			mcpServer, err = client.NewMCPServer(args[1], args[2:]...)
		}
		if err != nil {
			return err
		}

		client := client.NewStdio(mcpServer)

		stream, err := client.Start(ctx)
		if err != nil {
			return fmt.Errorf("unable to start MCP server: %w", err)
		}

		dump, err := scan.DumpAll(ctx, stream)
		if err != nil {
			return fmt.Errorf("unable to dump tools: %w", err)
		}

		cancel()

		toolHashes, err := scan.HashTools(dump.Tools)
		if err != nil {
			return fmt.Errorf("unable to hash tools: %w", err)
		}

		promptHashes, err := scan.HashPrompts(dump.Prompts)
		if err != nil {
			return fmt.Errorf("unable to hash prompts: %w", err)
		}

		sbom := scan.SBOM{
			Tools:   toolHashes,
			Prompts: promptHashes,
		}

		switch args[0] {

		case "check":

			refSBOM, err := scan.LoadSBOM(args[1])
			if err != nil {
				return fmt.Errorf("unable to load sbom: %w", err)
			}

			if err := refSBOM.Tools.Matches(sbom.Tools); err != nil {
				return fmt.Errorf("tools sbom does not match: %w", err)
			}

			if err := refSBOM.Prompts.Matches(sbom.Prompts); err != nil {
				return fmt.Errorf("prompts sbom does not match: %w", err)
			}

		case "sbom":

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(sbom); err != nil {
				return fmt.Errorf("unable to encode sbom: %w", err)
			}

		case "dump":

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(dump); err != nil {
				return fmt.Errorf("unable to encode dump: %w", err)
			}

		default:
			return fmt.Errorf("first command must be either dump, sbom or check")
		}

		return nil
	},
}
