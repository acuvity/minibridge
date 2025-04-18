package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.acuvity.ai/minibridge/pkgs/client"
	"go.acuvity.ai/minibridge/pkgs/utils"
)

func init() {

	initSharedFlagSet()

	Scan.Flags().AddFlagSet(fBackend)
}

// Scan is the cobra command to run the server.
var Scan = &cobra.Command{
	Use:              "scan [tools|sbom|check file.sbom] -- command [args...]",
	Short:            "Scan a MCP server and extract a reference file",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	Args:             cobra.MinimumNArgs(2),

	RunE: func(cmd *cobra.Command, args []string) error {

		var mcpServer client.MCPServer
		if args[0] == "check" {
			mcpServer = client.MCPServer{Command: args[2], Args: args[3:]}
		} else {
			mcpServer = client.MCPServer{Command: args[1], Args: args[2:]}
		}

		client := client.NewStdio(mcpServer)
		stream, err := client.Start(cmd.Context())
		if err != nil {
			return fmt.Errorf("unable to start MCP server: %w", err)
		}

		dump, err := utils.DumpAll(cmd.Context(), stream)
		if err != nil {
			return fmt.Errorf("unable to dump tools: %w", err)
		}

		toolHashes, err := utils.HashTools(dump.Tools)
		if err != nil {
			return fmt.Errorf("unable to hash tools: %w", err)
		}

		promptHashes, err := utils.HashPrompts(dump.Prompts)
		if err != nil {
			return fmt.Errorf("unable to hash prompts: %w", err)
		}

		sbom := utils.SBOM{
			Tools:   toolHashes,
			Prompts: promptHashes,
		}

		switch args[0] {

		case "check":

			refSBOM, err := utils.LoadSBOM(args[1])
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

			d, _ := json.MarshalIndent(sbom, "", "  ")
			fmt.Println(string(d))

		case "dump":

			d, _ := json.MarshalIndent(dump, "", "  ")
			fmt.Println(string(d))

		default:
			return fmt.Errorf("first command must be either dump, sbom or check")
		}

		return nil
	},
}
