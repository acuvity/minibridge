package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend/client"
	"go.acuvity.ai/minibridge/pkgs/scan"
)

var fScan = pflag.NewFlagSet("scan", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fScan.DurationP("timeout", "t", 2*time.Minute, "maximum time to allow the scan to run.")

	fScan.Bool("exclude-resources", false, "exclude resources from scan")
	fScan.Bool("exclude-tools", false, "exclude tools from scan")
	fScan.Bool("exclude-prompts", false, "exclude prompts from scan")

	Scan.Flags().AddFlagSet(fScan)
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

		timeout := viper.GetDuration("timeout")

		exclusions := &scan.Exclusions{
			Prompts:   viper.GetBool("exclude-prompts"),
			Resources: viper.GetBool("exclude-resources"),
			Tools:     viper.GetBool("exclude-tools"),
		}

		var ctx context.Context
		var cancel context.CancelFunc

		if timeout > 0 {
			ctx, cancel = context.WithTimeout(cmd.Context(), timeout)
		} else {
			ctx, cancel = context.WithCancel(cmd.Context())
		}
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

		dump, err := scan.DumpAll(ctx, stream, exclusions)
		if err != nil {
			return fmt.Errorf("unable to dump tools: %w", err)
		}

		cancel()

		var toolHashes scan.Hashes

		if !exclusions.Tools {
			toolHashes, err = scan.HashTools(dump.Tools)
			if err != nil {
				return fmt.Errorf("unable to hash tools: %w", err)
			}
		}

		var promptHashes scan.Hashes

		if !exclusions.Prompts {
			promptHashes, err = scan.HashPrompts(dump.Prompts)
			if err != nil {
				return fmt.Errorf("unable to hash prompts: %w", err)
			}
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
