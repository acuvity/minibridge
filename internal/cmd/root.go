package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.acuvity.ai/a3s/pkgs/bootstrap"
	"go.acuvity.ai/a3s/pkgs/conf"
	"go.acuvity.ai/a3s/pkgs/version"
)

func init() {

	initSharedFlagSet()

	Root.PersistentFlags().String("log-level", "info", "sets the log level.")
	Root.PersistentFlags().String("log-format", "console", "sets the log format.")
}

var Root = &cobra.Command{
	Use:              "minibridge",
	Short:            "Secure your MCP Servers",
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
			return err
		}

		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			return err
		}

		bootstrap.ConfigureLogger("minibridge", conf.LoggingConf{
			LogLevel:  viper.GetString("log-level"),
			LogFormat: viper.GetString("log-format"),
		})

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetBool("version") {
			fmt.Println(version.String("minibridge"))
			os.Exit(0)
		}
		return cmd.Usage()
	},
}
